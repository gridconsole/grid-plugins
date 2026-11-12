package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sgLimit = 10 << 20

// sgServer is the fake grid-server of the TestSg rigs: it writes sg-started to its
// output first, then app-url as the fixture's server does.
func sgServer(f *startFixture) {
	f.t.Helper()
	f.program("grid-server", `echo sg-started
printf 'http://%s\n' "$FAKE_ADDR" > "$GRID_HOME/app-url"
`)
}

func sgLogPath(f *startFixture, suffix string) string {
	return filepath.Join(f.gridHome, "logs", "grid-server.log"+suffix)
}

// sgWrite writes the log file <suffix> with a marker line padded to size bytes.
func sgWrite(t *testing.T, f *startFixture, suffix, marker string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(f.gridHome, "logs"), 0o700); err != nil {
		t.Fatal(err)
	}
	b := []byte(marker + "\n")
	if size > len(b) {
		b = append(b, []byte(strings.Repeat("x", size-len(b)))...)
	}
	if err := os.WriteFile(sgLogPath(f, suffix), b, 0o600); err != nil {
		t.Fatal(err)
	}
}

func sgRead(t *testing.T, f *startFixture, suffix string) string {
	t.Helper()
	b, err := os.ReadFile(sgLogPath(f, suffix))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sgExists(f *startFixture, suffix string) bool {
	_, err := os.Lstat(sgLogPath(f, suffix))
	return err == nil
}

func sgOpen(t *testing.T, f *startFixture) {
	t.Helper()
	f.env.OpenURL = func(string) error { return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
}

func TestSgRotatesAtStart(t *testing.T) {
	f := newStartFixture(t)
	sgServer(f)
	sgWrite(t, f, "", "sg-old", sgLimit+1)
	sgOpen(t, f)
	old, err := os.Stat(sgLogPath(f, ".1"))
	if err != nil {
		t.Fatalf("no grid-server.log.1: %v", err)
	}
	if old.Size() != sgLimit+1 {
		t.Errorf(".1 size %d, want %d", old.Size(), sgLimit+1)
	}
	cur, err := os.Stat(sgLogPath(f, ""))
	if err != nil {
		t.Fatal(err)
	}
	if cur.Size() >= 1024 {
		t.Errorf("grid-server.log size %d, want under 1 KiB", cur.Size())
	}
	if !strings.Contains(sgRead(t, f, ""), "sg-started") {
		t.Errorf("grid-server.log lacks sg-started")
	}
	for name, fi := range map[string]os.FileInfo{".log": cur, ".log.1": old} {
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("%s mode %o, want 600", name, fi.Mode().Perm())
		}
	}
}

func TestSgKeepsSeven(t *testing.T) {
	f := newStartFixture(t)
	sgServer(f)
	sgWrite(t, f, "", "m0", sgLimit+1)
	for i := 1; i <= 6; i++ {
		sgWrite(t, f, "."+string(rune('0'+i)), "m"+string(rune('0'+i)), 0)
	}
	sgOpen(t, f)
	if sgExists(f, ".7") {
		t.Errorf("grid-server.log.7 exists")
	}
	for i := 1; i <= 6; i++ {
		want := "m" + string(rune('0'+i-1))
		got := sgRead(t, f, "."+string(rune('0'+i)))
		if !strings.HasPrefix(got, want+"\n") {
			t.Errorf(".%d starts %q, want marker %s", i, got[:min(len(got), 8)], want)
		}
	}
	if strings.Contains(sgRead(t, f, ".6"), "m6\n") {
		t.Errorf("old .6 marker still there")
	}
	if !strings.Contains(sgRead(t, f, ""), "sg-started") {
		t.Errorf("grid-server.log lacks sg-started")
	}
}

func TestSgSmallLogAppends(t *testing.T) {
	f := newStartFixture(t)
	sgServer(f)
	sgWrite(t, f, "", "sg-old", 5000)
	sgOpen(t, f)
	if sgExists(f, ".1") {
		t.Errorf("grid-server.log.1 exists for a small log")
	}
	got := sgRead(t, f, "")
	if !strings.HasPrefix(got, "sg-old\n") || len(got) < 5000 {
		t.Errorf("old bytes not kept at the start (len %d)", len(got))
	}
	if !strings.HasSuffix(got, "sg-started\n") {
		t.Errorf("log does not end with sg-started")
	}
}

func TestSgRunningProgramNotRotated(t *testing.T) {
	f := newStartFixture(t)
	sgServer(f)
	f.running()
	sgWrite(t, f, "", "sg-old", sgLimit+1)
	sgOpen(t, f)
	if w, s := f.counts(); w != 0 || s != 0 {
		t.Fatalf("started with the app running: worker %d, server %d", w, s)
	}
	fi, err := os.Stat(sgLogPath(f, ""))
	if err != nil || fi.Size() != sgLimit+1 {
		t.Errorf("log changed: %v %v", fi, err)
	}
	if sgExists(f, ".1") {
		t.Errorf("grid-server.log.1 exists")
	}
}

func TestSgFailedShiftStillStarts(t *testing.T) {
	f := newStartFixture(t)
	sgServer(f)
	sgWrite(t, f, "", "sg-old", sgLimit+1)
	dir := sgLogPath(f, ".6")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "one"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	sgOpen(t, f)
	if _, s := f.counts(); s != 1 {
		t.Fatalf("server starts %d, want 1", s)
	}
	if sgExists(f, ".1") {
		t.Errorf("grid-server.log.1 exists after a failed shift")
	}
	got := sgRead(t, f, "")
	if len(got) <= sgLimit+1 || !strings.HasSuffix(got, "sg-started\n") {
		t.Errorf("log not appended to (len %d)", len(got))
	}
}
