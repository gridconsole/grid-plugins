package doctor

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

func bundleNames(t *testing.T, path string) []string {
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
	sort.Strings(names)
	return names
}

func writeLog(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func machineAt(dir string) machinefolder.Folder {
	return machinefolder.Resolve(machinefolder.Env{
		Getenv: func(k string) string {
			if k == machinefolder.EnvDir {
				return dir
			}
			return ""
		},
		Home: "/nonexistent-home", GOOS: "linux",
	})
}

// TestBundleIncludesTheLauncherLogsOfTheMachineFolder: with a machine folder, the
// support bundle holds this login's launcher logs (the server and worker logs and the
// routine timer logs), and not the lock or pid files beside them, and not another
// login's folder. Without one the bundle is what it was.
func TestBundleIncludesTheLauncherLogsOfTheMachineFolder(t *testing.T) {
	gh, machine := t.TempDir(), t.TempDir()
	writeLog(t, filepath.Join(gh, "logs", "grid-open.log"), "home log\n")
	writeLog(t, filepath.Join(machine, "launcher", "alice", "server.log"), "server\n")
	writeLog(t, filepath.Join(machine, "launcher", "alice", "routines", "nightly", "stdout.log"), "routine\n")
	writeLog(t, filepath.Join(machine, "launcher", "alice", "lock"), "")
	writeLog(t, filepath.Join(machine, "launcher", "alice", "pid"), "123\n")
	writeLog(t, filepath.Join(machine, "launcher", "bob", "server.log"), "other login\n")

	path, err := Bundle(BundleOptions{GridHome: gh, Machine: machineAt(machine), Login: "alice", Out: filepath.Join(t.TempDir(), "with.zip")})
	if err != nil {
		t.Fatal(err)
	}
	got := bundleNames(t, path)
	want := []string{"doctor.txt", "logs/grid-open.log", "logs/machine/launcher/routines/nightly/stdout.log", "logs/machine/launcher/server.log", "versions.txt"}
	if len(got) != len(want) {
		t.Fatalf("bundle holds %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bundle holds %v, want %v", got, want)
		}
	}

	path, err = Bundle(BundleOptions{GridHome: gh, Out: filepath.Join(t.TempDir(), "without.zip")})
	if err != nil {
		t.Fatal(err)
	}
	if got := bundleNames(t, path); len(got) != 3 {
		t.Fatalf("without a machine folder the bundle holds %v, want doctor.txt, logs/grid-open.log and versions.txt", got)
	}
}
