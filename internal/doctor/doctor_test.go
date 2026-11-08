package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitOK() (string, error)   { return "git version 2.0.0-test", nil }
func gitGone() (string, error) { return "", errors.New("not found") }

func TestRun(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, home string) // home is a fresh, existing folder
		git   func() (string, error)
		// want maps check name to wanted detail fragment; "" means the check must pass,
		// "!" prefix means it must fail with that fragment.
		want   map[string]string
		wantOK bool
	}{
		{"healthy", func(t *testing.T, home string) { writeVersion(t, home, "1") }, gitOK,
			map[string]string{"grid home": "", "grid home writable": "", "format version": "", "git": ""}, true},
		{"VERSION missing", func(t *testing.T, home string) {}, gitOK,
			map[string]string{"format version": "!cannot read"}, false},
		{"VERSION not a number", func(t *testing.T, home string) { writeVersion(t, home, "x") }, gitOK,
			map[string]string{"format version": "!cannot read"}, false},
		{"newer format is refused", func(t *testing.T, home string) { writeVersion(t, home, "99") }, gitOK,
			map[string]string{"format version": "!This data is format 99; this grid reads up to 1. Update grid."}, false},
		{"older format asks for migrate", func(t *testing.T, home string) { writeVersion(t, home, "0") }, gitOK,
			map[string]string{"format version": "!Run: grid migrate"}, false},
		{"git missing", func(t *testing.T, home string) { writeVersion(t, home, "1") }, gitGone,
			map[string]string{"git": "!git was not found"}, false},
		{"home is read-only", func(t *testing.T, home string) {
			if os.Geteuid() == 0 {
				t.Skip("root can write anywhere")
			}
			writeVersion(t, home, "1")
			if err := os.Chmod(home, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(home, 0o700) })
		}, gitOK, map[string]string{"grid home": "", "grid home writable": "!cannot write"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			tc.setup(t, home)
			rep := Run(Options{GridHome: home, Label: "~/.grid", GitVersion: tc.git})
			assertChecks(t, rep, tc.want, tc.wantOK)
		})
	}
}

func TestRunMissingHome(t *testing.T) {
	rep := Run(Options{GridHome: filepath.Join(t.TempDir(), "absent"), Label: "~/.grid", GitVersion: gitOK})
	assertChecks(t, rep, map[string]string{"grid home": "!~/.grid does not exist", "git": ""}, false)
	if len(rep.Checks) != 2 {
		t.Errorf("checks that need the folder should be skipped, got %+v", rep.Checks)
	}
}

func TestRunHomeIsAFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rep := Run(Options{GridHome: file, Label: "~/.grid", GitVersion: gitOK})
	assertChecks(t, rep, map[string]string{"grid home": "!is not a folder"}, false)
}

func writeVersion(t *testing.T, home, v string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, "VERSION"), []byte(v+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertChecks(t *testing.T, rep Report, want map[string]string, wantOK bool) {
	t.Helper()
	if rep.OK != wantOK {
		t.Errorf("OK = %v, want %v: %+v", rep.OK, wantOK, rep.Checks)
	}
	byName := map[string]Check{}
	for _, c := range rep.Checks {
		byName[c.Name] = c
	}
	for name, frag := range want {
		c, ok := byName[name]
		if !ok {
			t.Errorf("no check %q in %+v", name, rep.Checks)
			continue
		}
		if strings.HasPrefix(frag, "!") {
			if c.OK || !strings.Contains(c.Detail, frag[1:]) {
				t.Errorf("%s: want a failure containing %q, got %+v", name, frag[1:], c)
			}
		} else if !c.OK {
			t.Errorf("%s: want it to pass, got %+v", name, c)
		}
	}
}
