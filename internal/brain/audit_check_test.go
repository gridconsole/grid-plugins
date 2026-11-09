package brain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const auditAt = "2026-01-02T03:04:05.000Z"

func auditCodes(r CheckResult, code string) []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Code == code {
			out = append(out, f)
		}
	}
	return out
}

// auditLog writes the home level's log from lines built by auditLine.
func auditLog(t *testing.T, s Store, lines ...string) {
	t.Helper()
	chkPut(t, s, "knowledge/log.md", "# Brain log\n\n"+strings.Join(lines, ""), 0o600)
}

func auditEntry(t *testing.T, s Store, topic, name, text string) []byte {
	t.Helper()
	chkPut(t, s, "knowledge/"+topic+"/"+name+".md", text, 0o600)
	return []byte(text)
}

func accLine(at string, version int, hash string) string {
	return auditLine(Change{At: at, Status: "accepted", Topic: "decisions", Name: "e1", Version: version, Hash: hash, OriginKind: "job", OriginID: "T-01/J-01"})
}

func wantNoDrift(t *testing.T, s Store) {
	t.Helper()
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	if got := auditCodes(r, "audit_hash"); len(got) != 0 {
		t.Fatalf("unexpected audit_hash: %+v", got)
	}
}

// The entries the keeper wrote have no finding; a hand edit does, and names
// the newest logged version's hash. Both keeper paths L137 logs are covered:
// Accept (v1) and a config edit (v1, v2).
func TestAuditDriftAfterHandEdit(t *testing.T) {
	s := testStore(t)
	n := sample()
	accepted(t, s, n)
	edit := func(text string) {
		t.Helper()
		if err := s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) {
			return EncodeConfig(map[string]string{"name": "daily"}, text)
		}); err != nil {
			t.Fatal(err)
		}
	}
	edit("First.")
	edit("Second.")
	v2 := versionsOf(t, s, "routines", "daily")[1]
	wantNoDrift(t, s)

	root := filepath.Join(s.GridHome, "projects", "example", "knowledge")
	daily := filepath.Join(root, "routines", "daily.md")
	b, err := os.ReadFile(daily)
	if err != nil {
		t.Fatal(err)
	}
	edited := append(b, []byte("\nAdded by hand.\n")...)
	if err := os.WriteFile(daily, edited, 0o600); err != nil {
		t.Fatal(err)
	}
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	got := auditCodes(r, "audit_hash")
	want := "entry changed outside the keeper: v2 was " + v2.Hash + ", now " + auditHash(edited)
	if len(got) != 1 || got[0].File != "brain:project/knowledge/routines/daily.md" || got[0].Message != want || got[0].Line != 0 {
		t.Fatalf("finding = %+v, want %q", got, want)
	}

	// The accepted note drifts too, against its only version.
	v1 := versionsOf(t, s, "conventions", "checks")[0]
	note := filepath.Join(root, "conventions", "checks.md")
	nb, err := os.ReadFile(note)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(note, append(nb, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	r, err = s.Check()
	if err != nil {
		t.Fatal(err)
	}
	got = auditCodes(r, "audit_hash")
	if len(got) != 2 || got[0].File != "brain:project/knowledge/conventions/checks.md" || !strings.Contains(got[0].Message, "v1 was "+v1.Hash+", now "+auditHash(append(nb, '\n'))) {
		t.Fatalf("findings = %+v", got)
	}
}

// The log is append-only and the newest hashed line for an entry stands for it.
// Removing the keep-newest update (keeping the first line seen) turns the
// first two rows red; removing the stamp guard turns the third red.
func TestAuditNewestHashedLineWins(t *testing.T) {
	for name, stamps := range map[string][2]string{
		"same stamp, later line":       {auditAt, auditAt},
		"later stamp":                  {"2026-01-02T03:04:05.000Z", "2026-01-02T04:00:00.000Z"},
		"earlier stamp, appended last": {"2026-01-02T04:00:00.000Z", "2026-01-02T03:04:05.000Z"},
	} {
		s := chkStore(t)
		current := auditEntry(t, s, "decisions", "e1", "current bytes\n")
		old := accLine(stamps[0], 1, "sha256:0123456789abcdef")
		newest := accLine(stamps[1], 2, auditHash(current))
		lines := []string{old, newest}
		if name == "earlier stamp, appended last" {
			// v2 carries the later stamp; the v1 line is appended after it.
			lines = []string{accLine(stamps[0], 2, auditHash(current)), accLine(stamps[1], 1, "sha256:0123456789abcdef")}
		}
		auditLog(t, s, lines...)
		wantNoDrift(t, s)
	}
}

// A rejection publishes nothing: its line never becomes the newest hash, even
// when it carries one. Removing the rejected guard turns this red.
func TestAuditRejectedLineIsNeverNewest(t *testing.T) {
	s := chkStore(t)
	current := auditEntry(t, s, "decisions", "e1", "current bytes\n")
	rejected := auditLine(Change{At: "2026-01-03T00:00:00.000Z", Status: "rejected", Topic: "decisions", Name: "e1", Version: 3, Hash: "sha256:fedcba9876543210", OriginKind: "job", OriginID: "T-01/J-01", Reason: "No."})
	auditLog(t, s, accLine(auditAt, 2, auditHash(current)), rejected)
	wantNoDrift(t, s)
}

func TestAuditLegacyLinesAndGarbageGiveNoFinding(t *testing.T) {
	s := chkStore(t)
	auditEntry(t, s, "decisions", "legacy", "old entry\n")
	auditEntry(t, s, "decisions", "kept", "kept entry\n")
	legacy := "- 2026-01-02T03:04:05.000Z | accepted | legacy | job:T-01/J-01 | \n"
	auditLog(t, s, legacy,
		"not a log line\n",
		"- 2026-01-02T03:04:05.000Z | accepted | kept | job:T-01/J-01 |  | decisions | v1 | sha256:nothex\n",
		"- 2026-01-02T03:04:05.000Z | accepted | gone | job:T-01/J-01 |  | decisions | v1 | sha256:0123456789abcdef\n",
		"- ../../x | accepted\n")
	wantNoDrift(t, s)

	// No log at all is no finding either.
	if err := os.Remove(filepath.Join(s.GridHome, "knowledge", "log.md")); err != nil {
		t.Fatal(err)
	}
	wantNoDrift(t, s)
}
