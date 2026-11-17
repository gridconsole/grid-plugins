package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

func runDoctor(t *testing.T, home, settingsPath string) Report {
	t.Helper()
	return Run(Options{Home: filepath.Dir(home), SettingsPath: settingsPath, GridHome: home, Label: "~/.grid", GitVersion: gitOK, HooksFile: healthyHooks(t), LookPath: gridOK})
}

func check(t *testing.T, rep Report, name string) (Check, bool) {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

// A home only the hooks have written to, or only grid setup, has no VERSION and
// needs none yet: the first project or task writes it.
func TestFreshHomePasses(t *testing.T) {
	for _, dir := range []string{"sessions", "setup"} {
		t.Run(dir, func(t *testing.T) {
			home := t.TempDir()
			if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
				t.Fatal(err)
			}
			rep := runDoctor(t, home, healthyAllowSettings(t, filepath.Dir(home), home))
			c, ok := check(t, rep, "format version")
			if !ok || !c.OK || c.Fix != "" || strings.Contains(c.Detail, "run grid setup") {
				t.Errorf("format version: %+v", c)
			}
			if !rep.OK {
				t.Errorf("a fresh home fails: %+v", rep.Checks)
			}
		})
	}
}

// The real grid setup, then the doctor: the person is not sent back to setup.
func TestFreshHomeAfterGridSetupPasses(t *testing.T) {
	home := t.TempDir()
	user := filepath.Dir(home)
	configDir := t.TempDir()
	var out, errb bytes.Buffer
	code := settings.RunClaudeSetup(settings.ClaudeSetupOptions{
		Home: user, GridHome: home, ConfigDir: configDir, Yes: true, JSON: true,
		Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb,
	})
	if code != 0 {
		t.Fatalf("grid setup exit %d: %s%s", code, out.String(), errb.String())
	}
	if _, err := os.Stat(filepath.Join(home, "VERSION")); err == nil {
		t.Fatal("grid setup wrote VERSION; this test is about the home it leaves without one")
	}
	rep := runDoctor(t, home, filepath.Join(configDir, "settings.json"))
	c, _ := check(t, rep, "format version")
	if !c.OK || c.Fix != "" {
		t.Errorf("format version after grid setup: %+v", c)
	}
	if !rep.OK {
		t.Errorf("doctor after grid setup: %+v", rep.Checks)
	}
}

// A home that holds projects but no VERSION is an older build's: migrate is still the advice.
func TestHomeWithProjectsAndNoVersionStillAsksForMigrate(t *testing.T) {
	for _, dirs := range [][]string{{"setup", "projects"}, {"sessions", "personal"}} {
		home := t.TempDir()
		for _, d := range dirs {
			if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		rep := runDoctor(t, home, healthyAllowSettings(t, filepath.Dir(home), home))
		c, _ := check(t, rep, "format version")
		if c.OK || !strings.HasPrefix(c.Fix, "Run grid migrate") {
			t.Errorf("%v: format version %+v", dirs, c)
		}
	}
}

func TestDoctorSaysWhenTheOldAppFolderIsInTheWay(t *testing.T) {
	home := t.TempDir()
	writeVersion(t, home, "1")
	if err := os.MkdirAll(filepath.Join(home, "machine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "machine", "host.lock"), []byte("4242\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := runDoctor(t, home, healthyAllowSettings(t, filepath.Dir(home), home))
	c, ok := check(t, rep, "machine id")
	if !ok || c.OK || rep.OK {
		t.Fatalf("machine id row: %+v (found %v), report OK %v", c, ok, rep.OK)
	}
	if !strings.Contains(c.Detail, "~/.grid/machine is a folder from the older Grid app") || strings.Contains(c.Detail, "regular ULID file") {
		t.Errorf("detail %q", c.Detail)
	}
	for _, want := range []string{"grid project add", "machine.old-app", "close it"} {
		if !strings.Contains(c.Fix, want) {
			t.Errorf("fix %q does not say %q", c.Fix, want)
		}
	}
	if _, err := os.Stat(filepath.Join(home, "machine", "host.lock")); err != nil {
		t.Errorf("doctor touched the folder: %v", err)
	}
}

// Homes without the folder get no extra row, so every report stays as it was.
func TestDoctorHasNoMachineRowOtherwise(t *testing.T) {
	for name, setup := range map[string]func(t *testing.T, home string){
		"no machine": func(t *testing.T, home string) {},
		"id file": func(t *testing.T, home string) {
			writeFile(t, filepath.Join(home, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAV\n")
		},
		"edited file": func(t *testing.T, home string) { writeFile(t, filepath.Join(home, "machine"), "my notes\n") },
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			writeVersion(t, home, "1")
			setup(t, home)
			rep := runDoctor(t, home, healthyAllowSettings(t, filepath.Dir(home), home))
			if c, ok := check(t, rep, "machine id"); ok {
				t.Errorf("unexpected row %+v", c)
			}
		})
	}
}

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}
