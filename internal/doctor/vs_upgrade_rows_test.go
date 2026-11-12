package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/version"
)

// vsDocOptions is a healthy environment whose grid on PATH is /x/bin/grid.
func vsDocOptions(t *testing.T) Options {
	t.Helper()
	home := t.TempDir()
	writeVersion(t, home, "1")
	return Options{Home: filepath.Dir(home), SettingsPath: healthyAllowSettings(t, filepath.Dir(home), home), GridHome: home, Label: "~/.grid",
		GitVersion: gitOK, HooksFile: healthyHooks(t), LookPath: func(string) (string, error) { return "/x/bin/grid", nil }}
}

func vsDocCheck(rep Report, name string) (Check, bool) {
	for _, c := range rep.Checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

func vsDocWant(t *testing.T, rep Report, name string) Check {
	t.Helper()
	c, ok := vsDocCheck(rep, name)
	if !ok {
		t.Fatalf("no %q check in %+v", name, rep.Checks)
	}
	return c
}

func TestVsDocPathVersionDiffers(t *testing.T) {
	o := vsDocOptions(t)
	var asked string
	o.GridVersion = func(p string) (string, error) { asked = p; return "0.0.1-vs", nil }
	rep := Run(o)
	c := vsDocWant(t, rep, "grid on PATH")
	if c.OK || rep.OK {
		t.Fatalf("a different grid on PATH must fail: %+v", c)
	}
	if asked != "/x/bin/grid" {
		t.Errorf("GridVersion asked about %q, want /x/bin/grid", asked)
	}
	for _, want := range []string{"/x/bin/grid", "0.0.1-vs", version.Version} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail %q lacks %q", c.Detail, want)
		}
	}
	if !strings.Contains(c.Fix, "Update or remove the grid at the path above") {
		t.Errorf("fix = %q", c.Fix)
	}
}

func TestVsDocPathVersionSame(t *testing.T) {
	o := vsDocOptions(t)
	o.GridVersion = func(string) (string, error) { return version.Version, nil }
	rep := Run(o)
	c := vsDocWant(t, rep, "grid on PATH")
	if !c.OK || c.Detail != "grid is on PATH; version "+version.Version {
		t.Fatalf("got %+v", c)
	}
	if !rep.OK {
		t.Fatalf("report not ok: %+v", rep)
	}
}

func TestVsDocPathVersionNotRead(t *testing.T) {
	for name, fn := range map[string]func(string) (string, error){
		"nil":   nil,
		"error": func(string) (string, error) { return "", errors.New("exit 1") },
	} {
		t.Run(name, func(t *testing.T) {
			o := vsDocOptions(t)
			o.GridVersion = fn
			rep := Run(o)
			c := vsDocWant(t, rep, "grid on PATH")
			if !c.OK || c.Detail != "grid is on PATH; version not read" || c.Fix != "" {
				t.Fatalf("got %+v", c)
			}
			if !rep.OK {
				t.Fatalf("report not ok: %+v", rep)
			}
		})
	}
}

func TestVsDocPathNotFoundUnchanged(t *testing.T) {
	o := vsDocOptions(t)
	o.LookPath = func(string) (string, error) { return "", errors.New("missing") }
	o.GridVersion = func(string) (string, error) { t.Error("GridVersion called without a grid"); return "", nil }
	c := vsDocWant(t, Run(o), "grid on PATH")
	if c.OK || c.Detail != "grid was not found on PATH" || !strings.HasPrefix(c.Fix, "Run grid setup") {
		t.Fatalf("got %+v", c)
	}
}

func vsDocLauncherPath(o Options) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(o.GridHome, "bin", "grid.exe")
	}
	return filepath.Join(o.GridHome, "bin", "grid")
}

func TestVsDocLauncherAbsentHasNoCheck(t *testing.T) {
	o := vsDocOptions(t)
	o.GridVersion = func(string) (string, error) { return version.Version, nil }
	if c, ok := vsDocCheck(Run(o), "launcher"); ok {
		t.Fatalf("launcher check without a launcher: %+v", c)
	}
}

func vsDocWithLauncher(t *testing.T, o Options) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "current-grid")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	if err := os.WriteFile(target, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := vsDocLauncherPath(o)
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, launcher); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func TestVsDocLauncherVersions(t *testing.T) {
	o := vsDocOptions(t)
	target := vsDocWithLauncher(t, o)
	answer := "0.0.2-old"
	o.GridVersion = func(p string) (string, error) {
		if p == "/x/bin/grid" {
			return version.Version, nil
		}
		return answer, nil
	}
	rep := Run(o)
	c := vsDocWant(t, rep, "launcher")
	if c.OK || rep.OK {
		t.Fatalf("a different launcher must fail: %+v", c)
	}
	for _, want := range []string{target, "0.0.2-old", version.Version} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail %q lacks %q", c.Detail, want)
		}
	}
	if !strings.Contains(c.Fix, "repoint the launcher") {
		t.Errorf("fix = %q", c.Fix)
	}

	answer = version.Version
	rep = Run(o)
	c = vsDocWant(t, rep, "launcher")
	if !c.OK || c.Detail != "launcher points at "+target+"; version "+version.Version || !rep.OK {
		t.Fatalf("same version: %+v ok=%v", c, rep.OK)
	}

	o.GridVersion = nil
	c = vsDocWant(t, Run(o), "launcher")
	if !c.OK || c.Detail != "launcher points at "+target+"; version not read" {
		t.Fatalf("not read: %+v", c)
	}
}

func TestVsDocWorkerStoppedHasNoCheck(t *testing.T) {
	o := vsDocOptions(t)
	// A status file left behind with no lock held is a stale worker, not a running one.
	if err := os.WriteFile(filepath.Join(o.GridHome, "worker-status.json"), []byte(`{"version":"0.0.9"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if c, ok := vsDocCheck(Run(o), "worker"); ok {
		t.Fatalf("worker check for a stopped worker: %+v", c)
	}
}
