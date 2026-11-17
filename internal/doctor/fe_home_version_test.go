package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFreshHomeVersion(t *testing.T) {
	mk := func(t *testing.T, home string, dirs ...string) {
		t.Helper()
		for _, d := range dirs {
			if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
	}
	tests := []struct {
		name       string
		setup      func(t *testing.T, home string)
		wantDetail string
		wantFix    string
	}{
		// A1, a hooks-only home, no longer fails: TestFreshHomePasses covers it.
		{"A2 sessions and projects", func(t *testing.T, home string) { mk(t, home, "sessions", "projects") },
			"cannot read", "Run grid migrate"},
		{"A3 VERSION not a number", func(t *testing.T, home string) {
			mk(t, home, "sessions")
			writeVersion(t, home, "x")
		}, "cannot read", "Run grid migrate"},
		{"A4 empty home", func(t *testing.T, home string) {}, "cannot read", "Run grid migrate"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			tc.setup(t, home)
			rep := Run(Options{Home: filepath.Dir(home), SettingsPath: healthyAllowSettings(t, filepath.Dir(home), home), GridHome: home, Label: "~/.grid", GitVersion: gitOK, HooksFile: healthyHooks(t), LookPath: gridOK})
			assertChecks(t, rep, map[string]string{"format version": "!" + tc.wantDetail}, false)
			for _, c := range rep.Checks {
				if c.Name == "format version" && !hasPrefix(c.Fix, tc.wantFix) {
					t.Errorf("fix = %q, want prefix %q", c.Fix, tc.wantFix)
				}
			}
		})
	}
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }
