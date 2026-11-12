package routine

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// blLaunchWriteExe writes a file standing for a grid binary in a folder of its own.
func blLaunchWriteExe(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "grid-binary")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBlLaunchPathName(t *testing.T) {
	home := filepath.Join(t.TempDir(), "grid home")
	for goos, name := range map[string]string{"linux": "grid", "darwin": "grid", "windows": "grid.exe"} {
		if got, want := blLaunchPath(home, goos), filepath.Join(home, "bin", name); got != want {
			t.Fatalf("%s launcher: %s, want %s", goos, got, want)
		}
	}
	want := filepath.Join(home, "bin", "grid")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if got := LauncherPath(home); got != want {
		t.Fatalf("LauncherPath: %s, want %s", got, want)
	}
}

// A timer file names the launcher in the Grid home and nothing of the versioned folder.
func TestBlLaunchTimerFilesNameTheLauncher(t *testing.T) {
	home := t.TempDir()
	gridHome := filepath.Join(home, ".grid")
	versioned := filepath.Join(home, ".claude", "plugins", "cache", "grid", "grid", "0.1.9", "bin")
	path := "/usr/local/bin:/usr/bin:/bin"
	for _, tc := range []struct {
		name    string
		backend Backend
		goos    string
	}{
		{"systemd", Systemd{}, "linux"},
		{"launchd", Launchd{UID: 501}, "darwin"},
		{"schtasks", Schtasks{}, "windows"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			launcher := blLaunchPath(gridHome, tc.goos)
			plan, err := tc.backend.Plan(TimerSpec{Home: home, GridHome: gridHome, Binary: launcher, Name: "daily", Path: path})
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Files) == 0 {
				t.Fatal("no timer file planned")
			}
			for _, file := range plan.Files {
				if strings.Contains(file.Content, versioned) || strings.Contains(file.Content, "0.1.9") {
					t.Fatalf("%s names the versioned folder:\n%s", file.Path, file.Content)
				}
			}
			var command string
			switch tc.name {
			case "systemd":
				for _, line := range strings.Split(plan.Files[0].Content, "\n") {
					if rest, ok := strings.CutPrefix(line, "ExecStart="); ok {
						command = strings.SplitN(rest, `" "`, 2)[0]
						command = strings.TrimPrefix(command, `"`)
					}
				}
			case "launchd":
				_, _, args := plistTopLevel(t, plan.Files[0].Content)
				if len(args) > 0 {
					command = args[0]
				}
			case "schtasks":
				var task schtasksXML
				decoder := xml.NewDecoder(strings.NewReader(plan.Files[0].Content))
				decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
				if err := decoder.Decode(&task); err != nil {
					t.Fatal(err)
				}
				command = task.Command
			}
			if command != launcher {
				t.Fatalf("timer command %q, want the launcher %q", command, launcher)
			}
			if want := map[string]string{"linux": "grid", "darwin": "grid", "windows": "grid.exe"}[tc.goos]; filepath.Base(command) != want || filepath.Base(filepath.Dir(command)) != "bin" {
				t.Fatalf("timer command %q is not bin/%s in the Grid home", command, want)
			}
		})
	}
}

// The Windows way of making the launcher runs on any host: a hard link, or a copy
// when the link fails, replaced only when size or modification time differ.
func TestBlLaunchFileHardLinkThenCopyFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bin", "grid.exe")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	exe := blLaunchWriteExe(t, "binary one")
	changed, err := blLaunchFile(path, exe)
	if err != nil || !changed {
		t.Fatalf("first refresh: changed %v, err %v", changed, err)
	}
	pathInfo, _ := os.Stat(path)
	exeInfo, _ := os.Stat(exe)
	if !os.SameFile(pathInfo, exeInfo) {
		t.Fatal("launcher is not a hard link to the executable")
	}
	if changed, err = blLaunchFile(path, exe); err != nil || changed {
		t.Fatalf("second refresh: changed %v, err %v", changed, err)
	}

	// An update puts a new binary of another size at a new path: the launcher follows.
	exe2 := blLaunchWriteExe(t, "binary number two")
	if changed, err = blLaunchFile(path, exe2); err != nil || !changed {
		t.Fatalf("new exe refresh: changed %v, err %v", changed, err)
	}
	if got, _ := os.ReadFile(path); string(got) != "binary number two" {
		t.Fatalf("launcher holds %q", got)
	}

	// No hard link (another volume): a copy with the executable's time, left alone next time.
	blLaunchLink = func(string, string) error { return errors.New("links are not available") }
	t.Cleanup(func() { blLaunchLink = os.Link })
	exe3 := blLaunchWriteExe(t, "binary three!!")
	if changed, err = blLaunchFile(path, exe3); err != nil || !changed {
		t.Fatalf("copy refresh: changed %v, err %v", changed, err)
	}
	pathInfo, _ = os.Stat(path)
	exe3Info, _ := os.Stat(exe3)
	if os.SameFile(pathInfo, exe3Info) || !pathInfo.ModTime().Equal(exe3Info.ModTime()) || pathInfo.Size() != exe3Info.Size() {
		t.Fatalf("copy is a link or differs: %v %v", pathInfo.ModTime(), exe3Info.ModTime())
	}
	if got, _ := os.ReadFile(path); string(got) != "binary three!!" {
		t.Fatalf("copy holds %q", got)
	}
	if changed, err = blLaunchFile(path, exe3); err != nil || changed {
		t.Fatalf("copy rerun: changed %v, err %v", changed, err)
	}

	// Same size, other time: replaced.
	later := exe3Info.ModTime().Add(time.Hour)
	if err := os.Chtimes(exe3, later, later); err != nil {
		t.Fatal(err)
	}
	if changed, err = blLaunchFile(path, exe3); err != nil || !changed {
		t.Fatalf("time refresh: changed %v, err %v", changed, err)
	}
	if pathInfo, _ = os.Stat(path); !pathInfo.ModTime().Equal(later) {
		t.Fatalf("copy time %v, want %v", pathInfo.ModTime(), later)
	}
	if left, _ := filepath.Glob(path + ".*"); len(left) != 0 {
		t.Fatalf("temporary files left behind: %v", left)
	}
}
