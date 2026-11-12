package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/routine"
)

// blLaunchSchedule runs `grid routine schedule daily --json` in the harness and
// returns the text of the timer files it planned. The executable is a path in a
// versioned folder, as the plugin installs it. Windows only previews the plan;
// elsewhere the OS commands (systemctl, launchctl) are fake scripts first on PATH
// that record their arguments and do nothing, so no real timer is installed.
func blLaunchSchedule(t *testing.T, h *harness, exe string) string {
	t.Helper()
	h.env.Executable = func() (string, error) { return exe, nil }
	if code := h.run(h.root, "routine", "add", "daily", "--every", "1h", "--run", "grid version"); code != 0 {
		t.Fatalf("routine add: exit %d: %s", code, h.errb)
	}
	var calls string
	if runtime.GOOS != "windows" {
		bin := t.TempDir()
		calls = filepath.Join(bin, "calls.log")
		script := "#!/bin/sh\necho \"$0 $*\" >> '" + calls + "'\nexit 0\n"
		for _, name := range []string{"systemctl", "launchctl"} {
			if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(h.root, "routine", "schedule", "daily", "--json"); code != 0 {
		t.Fatalf("routine schedule: exit %d: %s", code, h.errb)
	}
	if runtime.GOOS == "windows" {
		var reply struct {
			Timers []struct {
				Files []struct{ Content string } `json:"files"`
			} `json:"timers"`
		}
		if err := json.Unmarshal(h.out.Bytes(), &reply); err != nil || len(reply.Timers) != 1 || len(reply.Timers[0].Files) == 0 {
			t.Fatalf("preview reply: %s (%v)", h.out, err)
		}
		return reply.Timers[0].Files[0].Content
	}
	// Only the fake scripts may have been started.
	if log, err := os.ReadFile(calls); err != nil || !strings.Contains(string(log), filepath.Dir(calls)) {
		t.Fatalf("the fake OS command did not run: %q %v", log, err)
	}
	var text []string
	for _, pattern := range []string{
		filepath.Join(h.home, ".config", "systemd", "user", "*.service"),
		filepath.Join(h.home, "Library", "LaunchAgents", "*.plist"),
	} {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			text = append(text, string(b))
		}
	}
	if len(text) != 1 {
		t.Fatalf("expected one timer file, got %d", len(text))
	}
	return text[0]
}

func blLaunchVersionedExe(h *harness) string {
	return filepath.Join(h.root, "plugins", "cache", "grid", "grid", "0.1.9", "bin", "grid-test")
}

func TestBlLaunchScheduleStartsTheLauncherWhenThereIsOne(t *testing.T) {
	h := newHarness(t)
	launcher := routine.LauncherPath(h.gridHome)
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("launcher"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := blLaunchSchedule(t, h, blLaunchVersionedExe(h))
	if !strings.Contains(content, launcher) {
		t.Fatalf("timer does not start the launcher %s:\n%s", launcher, content)
	}
	if strings.Contains(content, "0.1.9") || strings.Contains(content, "grid-test") {
		t.Fatalf("timer names the versioned executable:\n%s", content)
	}
}

func TestBlLaunchScheduleStartsTheExecutableWithoutALauncher(t *testing.T) {
	h := newHarness(t)
	exe := blLaunchVersionedExe(h)
	content := blLaunchSchedule(t, h, exe)
	if !strings.Contains(content, exe) {
		t.Fatalf("timer does not start the executable %s:\n%s", exe, content)
	}
	if strings.Contains(content, routine.LauncherPath(h.gridHome)) {
		t.Fatalf("timer names a launcher that is not there:\n%s", content)
	}
}

// An update that removes the binary leaves the launcher link dangling; a timer
// must not be planned on a file that cannot start.
func TestBlLaunchScheduleIgnoresALauncherWhoseTargetIsGone(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a symlink needs a privilege on Windows; the Windows launcher is a hard link or a copy")
	}
	h := newHarness(t)
	launcher := routine.LauncherPath(h.gridHome)
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(h.root, "removed-by-update"), launcher); err != nil {
		t.Fatal(err)
	}
	exe := blLaunchVersionedExe(h)
	content := blLaunchSchedule(t, h, exe)
	if !strings.Contains(content, exe) || strings.Contains(content, launcher) {
		t.Fatalf("timer should start the executable, not the dangling launcher:\n%s", content)
	}
}
