package brain

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bigLog is a hand-built log past the 1 MiB cap of safefile.Read, in the format
// from before versions existed.
func bigLog(t *testing.T, s Store) (string, int) {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("# Brain log\n\n")
	lines := 0
	for b.Len() <= 1<<20+1<<10 {
		b.WriteString("- 2025-12-01T10:00:00.000Z | accepted | alpha | job:T-01/J-01 |\n")
		lines++
	}
	putFile(t, s, "projects/example/knowledge/log.md", b.String())
	return b.String(), lines
}

func logRoot(t *testing.T, s Store) *os.Root {
	t.Helper()
	root, err := s.scope("project", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func TestLargeLogStillTakesConfigEditsAndAccepts(t *testing.T) {
	s := testStore(t)
	before, lines := bigLog(t, s)

	if err := s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) {
		return EncodeConfig(map[string]string{"name": "daily"}, "First.")
	}); err != nil {
		t.Fatalf("config edit on a log over 1 MiB: %v", err)
	}
	n := sample()
	accepted(t, s, n)

	log := readLog(t, s)
	if !strings.HasPrefix(log, before) {
		t.Fatal("the existing log was rewritten, not appended to")
	}
	for _, want := range []string{"| accepted | daily | config:", "| accepted | checks | job:T-01/J-01"} {
		if strings.Count(log, want) != 1 {
			t.Fatalf("log should hold %q once", want)
		}
	}

	// The same decision again is a retry: still one line each, also past 1 MiB.
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatal(err)
	}
	if err := s.EditConfig("project", "routines", "daily", func(old []byte) ([]byte, error) { return old, nil }); err != nil {
		t.Fatal(err)
	}
	if readLog(t, s) != log {
		t.Fatal("a retry changed the log")
	}

	changes, err := s.Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatalf("changes on a log over 1 MiB: %v", err)
	}
	if len(changes) != lines+2 {
		t.Fatalf("changes: %d, want %d", len(changes), lines+2)
	}
	// Both new lines share one stamp; the later line leads, then the earlier.
	if changes[0].Name != "checks" || changes[1].Name != "daily" || changes[2].Name != "alpha" {
		t.Fatalf("newest first: %+v", changes[:3])
	}
	if limited, err := s.Changes("project", time.Time{}, 1); err != nil || len(limited) != 1 || limited[0].Name != "checks" {
		t.Fatalf("limit: %+v %v", limited, err)
	}
}

func TestLogAuditRetryOnlyLooksAtTheTail(t *testing.T) {
	s := testStore(t)
	root := logRoot(t, s)
	c := Change{At: "2026-01-02T03:04:05.000Z", Status: "accepted", Topic: "routines", Name: "daily",
		Version: 1, Hash: auditHash([]byte("x")), OriginKind: "config", OriginID: "abc"}
	for range 3 {
		if err := logAudit(root, c); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := readLog(t, s), "# Brain log\n\n"+auditLine(c); got != want {
		t.Fatalf("log:\n%s\nwant:\n%s", got, want)
	}

	// Buried deeper than the tail, the line is not seen again: reading is bounded.
	other := c
	other.Name = "other"
	var filler strings.Builder
	for i := 0; filler.Len() <= logTail; i++ {
		other.OriginID = fmt.Sprintf("n%04d", i)
		filler.WriteString(auditLine(other))
	}
	f, err := os.OpenFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "log.md"), os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(filler.String()); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := logAudit(root, c); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(readLog(t, s), auditLine(c)); got != 2 {
		t.Fatalf("a line outside the tail should be appended again, found %d", got)
	}
}

func TestLogAuditEndsAnInterruptedLine(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/knowledge/log.md", "# Brain log\n\n- 2025-12-01T10:00:00.000Z | accepted | alp")
	root := logRoot(t, s)
	c := Change{At: "2026-01-02T03:04:05.000Z", Status: "accepted", Topic: "routines", Name: "daily", OriginKind: "config", OriginID: "abc"}
	if err := logAudit(root, c); err != nil {
		t.Fatal(err)
	}
	got, err := s.Changes("project", time.Time{}, 0)
	if err != nil || len(got) != 1 || got[0].Name != "daily" {
		t.Fatalf("changes: %+v %v", got, err)
	}
}

func TestLogRefusesSymlinksAndNonRegularFiles(t *testing.T) {
	for name, make := range map[string]func(dir string) error{
		"symlink":   func(dir string) error { return os.Symlink("elsewhere.md", filepath.Join(dir, "log.md")) },
		"directory": func(dir string) error { return os.Mkdir(filepath.Join(dir, "log.md"), 0o700) },
	} {
		t.Run(name, func(t *testing.T) {
			s := testStore(t)
			dir := filepath.Join(s.GridHome, "projects", "example", "knowledge")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := make(dir); err != nil {
				t.Skip(err)
			}
			c := Change{At: "2026-01-02T03:04:05.000Z", Status: "accepted", Topic: "routines", Name: "daily", OriginKind: "config", OriginID: "abc"}
			if err := logAudit(logRoot(t, s), c); err == nil {
				t.Fatal("log appended to")
			}
			if _, err := s.Changes("project", time.Time{}, 0); err == nil {
				t.Fatal("log read")
			}
			if _, err := os.Stat(filepath.Join(dir, "elsewhere.md")); err == nil {
				t.Fatal("symlink target was created")
			}
		})
	}
}
