package routine

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

func machineFolderAt(t *testing.T, dir string) machinefolder.Folder {
	t.Helper()
	return machinefolder.Resolve(machinefolder.Env{
		Getenv: func(k string) string {
			if k == machinefolder.EnvDir {
				return dir
			}
			return ""
		},
		Home: t.TempDir(), GOOS: "linux",
	})
}

func launchdPlan(t *testing.T, spec TimerSpec) TimerPlan {
	t.Helper()
	spec.Home, spec.GridHome, spec.Binary, spec.Name, spec.Path = t.TempDir(), t.TempDir(), "/bin/grid", "nightly", "/usr/bin"
	p, err := Launchd{UID: 501}.Plan(spec)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLaunchdLogGoesToTheMachineFolderPerLogin: with a machine folder the timer's
// stdout and stderr go to launcher/<login>/routines/<label> there, and the plist
// names them, so two logins that schedule one routine never share a log.
func TestLaunchdLogGoesToTheMachineFolderPerLogin(t *testing.T) {
	machine := t.TempDir()
	folder := machineFolderAt(t, machine)
	dirs := map[string]string{}
	for _, login := range []string{"alice", "bob"} {
		p := launchdPlan(t, TimerSpec{Machine: folder, Login: login})
		want := filepath.Join(machine, "launcher", login, "routines", p.Label)
		if p.LogDir != want {
			t.Fatalf("%s: log folder %s, want %s", login, p.LogDir, want)
		}
		plist := p.Files[0].Content
		if !strings.Contains(plist, filepath.Join(want, "stdout.log")) || !strings.Contains(plist, filepath.Join(want, "stderr.log")) {
			t.Fatalf("%s: the plist does not name the log in %s:\n%s", login, want, plist)
		}
		dirs[login] = p.LogDir
	}
	if dirs["alice"] == dirs["bob"] {
		t.Fatalf("two logins share the log folder %s", dirs["alice"])
	}
}

// TestLaunchdLogUnconfiguredStaysInTheGridHome: no machine folder, or the private OS
// standard place, keeps <grid home>/logs/routines/<label>.
func TestLaunchdLogUnconfiguredStaysInTheGridHome(t *testing.T) {
	for name, spec := range map[string]TimerSpec{
		"zero folder":    {Login: "alice"},
		"default folder": {Machine: machineFolderAt(t, ""), Login: "alice"},
		"no login":       {Machine: machineFolderAt(t, t.TempDir())},
	} {
		p := launchdPlan(t, spec)
		wantDir := filepath.Join(filepath.Dir(filepath.Dir(p.LogDir)), "routines", p.Label)
		if filepath.Base(filepath.Dir(filepath.Dir(p.LogDir))) != "logs" || p.LogDir != wantDir {
			t.Fatalf("%s: log folder %s, want <grid home>/logs/routines/%s", name, p.LogDir, p.Label)
		}
	}
}
