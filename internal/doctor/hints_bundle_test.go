package doctor

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFixHintForAllowBlockAndUnknownNames(t *testing.T) {
	for _, tc := range []struct{ name, detail, want string }{
		{"allow block", "Grid home x", "Fix the Grid home location, then run grid setup."},
		{"allow block", "cannot locate the Claude settings file", "Fix or move the Claude settings file, then run grid setup."},
		{"nothing", "", ""},
	} {
		if got := fixHint(tc.name, tc.detail); got != tc.want {
			t.Errorf("fixHint(%q, %q) = %q, want %q", tc.name, tc.detail, got, tc.want)
		}
	}
}

func TestRunWithNoSettingsPathFailsTheAllowBlock(t *testing.T) {
	home := t.TempDir()
	gridHome := filepath.Join(home, ".grid")
	if err := os.Mkdir(gridHome, 0700); err != nil {
		t.Fatal(err)
	}
	writeVersion(t, gridHome, "1")
	rep := Run(Options{Home: home, GridHome: gridHome, Label: "~/.grid", GitVersion: gitOK, HooksFile: healthyHooks(t), LookPath: gridOK})
	assertChecks(t, rep, map[string]string{"allow block": "!cannot locate the Claude settings file"}, false)
	for _, c := range rep.Checks {
		if c.Name == "allow block" && c.Fix != "Fix or move the Claude settings file, then run grid setup." {
			t.Fatalf("allow block fix: %+v", c)
		}
	}
}

func TestValidateHooksTooLargeAndEmptyEvent(t *testing.T) {
	big := `{"pad":"` + strings.Repeat("x", 1<<20) + `",` + strings.TrimPrefix(goodHooks, "{")
	if err := ValidateHooks([]byte(big)); err == nil || err.Error() != "plugin hooks file is too large" {
		t.Fatalf("over 1 MiB: %v", err)
	}
	empty := strings.Replace(goodHooks, `{"hooks":{`, `{"hooks":{"Stop":[],`, 1)
	if err := ValidateHooks([]byte(empty)); err == nil || err.Error() != "plugin hooks file has an empty event" {
		t.Fatalf("event with no groups: %v", err)
	}
}

// failingWriter fails the write numbered failOn (1 is the first).
type failingWriter struct {
	failOn, writes int
	err            error
}

func (w *failingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == w.failOn {
		return 0, w.err
	}
	return len(p), nil
}

func TestWriteReportReturnsTheWritersError(t *testing.T) {
	boom := errors.New("disk full")
	rep := Report{Checks: []Check{{Name: "git", OK: false, Detail: "gone", Fix: "install it"}}}
	if err := WriteReport(&failingWriter{failOn: 1, err: boom}, rep); !errors.Is(err, boom) {
		t.Fatalf("first write: %v", err)
	}
	w := &failingWriter{failOn: 2, err: boom}
	if err := WriteReport(w, rep); !errors.Is(err, boom) || w.writes != 2 {
		t.Fatalf("fix line write: %v after %d writes", err, w.writes)
	}
}

func TestBundleRefusals(t *testing.T) {
	out := filepath.Join(t.TempDir(), "x.zip")
	if _, err := Bundle(BundleOptions{Out: out}); err == nil || err.Error() != "cannot find the Grid home; set GRID_HOME" {
		t.Fatalf("no Grid home: %v", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an archive was written: %v", err)
	}

	gh := t.TempDir()
	if err := os.WriteFile(filepath.Join(gh, "logs"), []byte("not a folder"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Bundle(BundleOptions{GridHome: gh, Out: out}); err == nil || err.Error() != "logs must be a directory, not a symlink" {
		t.Fatalf("logs as a file: %v", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("an archive was written: %v", err)
	}
}

// zipOrder lists the entry names of an archive in the order they were written.
func zipOrder(t *testing.T, path string) []string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	return names
}

func TestBundleLeavesOutUnsafeNamesAndOrdersEqualTimesByName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fslock support is Unix-only")
	}
	at := time.Date(2026, 11, 8, 15, 4, 5, 0, time.UTC)

	home := t.TempDir()
	gh := filepath.Join(home, ".grid")
	writeBundleFixture(t, filepath.Join(gh, "logs", `a\b.log`), "backslash", at)
	writeBundleFixture(t, filepath.Join(gh, "logs", "ok.log"), "fine", at.Add(-time.Hour))
	path, err := Bundle(BundleOptions{GridHome: gh, Home: home, Out: filepath.Join(home, "one.zip"), Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(zipOrder(t, path), ","), "doctor.txt,versions.txt,logs/ok.log"; got != want {
		t.Fatalf("entries: got %s want %s", got, want)
	}

	home = t.TempDir()
	gh = filepath.Join(home, ".grid")
	var want []string
	for i := 14; i >= 0; i-- { // written newest name first, so only the sort puts them in order
		writeBundleFixture(t, filepath.Join(gh, "logs", fmt.Sprintf("log-%02d.txt", i)), "body", at)
	}
	for i := 0; i < 15; i++ {
		want = append(want, fmt.Sprintf("logs/log-%02d.txt", i))
	}
	path, err = Bundle(BundleOptions{GridHome: gh, Home: home, Out: filepath.Join(home, "two.zip"), Now: func() time.Time { return at }})
	if err != nil {
		t.Fatal(err)
	}
	got := zipOrder(t, path)
	if strings.Join(got[2:], ",") != strings.Join(want, ",") {
		t.Fatalf("equal times must come in name order: %v", got)
	}
}

func TestReadBundleTailRedactsAKeyCutByTheLimit(t *testing.T) {
	// The cut falls inside the marker (bytes 40-58 of the first line), so the
	// discarded prefix holds only "-----BEGIN" and the whole marker is seen only
	// in the carried bytes plus the dropped first line of the tail.
	first := strings.Repeat("A", 40) + "-----BEGIN KEY-----" + strings.Repeat("B", 100) + "\n"
	tail := "tail one\ntail two\n"
	const extra = 50 // the bytes cut from the front, all inside the first line
	filler := strings.Repeat("p", int(bundleFileLimit)+extra-len(first)-len(tail)-1) + "\n"
	path := filepath.Join(t.TempDir(), "big.log")
	if err := os.WriteFile(path, []byte(first+filler+tail), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := readBundleTail(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "-----BEGIN REDACTED-----\n") || !strings.HasSuffix(got, tail) || strings.Contains(got, "KEY") {
		t.Fatalf("bad tail: len %d, starts %q", len(got), got[:min(len(got), 40)])
	}
}
