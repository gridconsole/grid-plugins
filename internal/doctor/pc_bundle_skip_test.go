package doctor

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func zipEntry(t *testing.T, path, name string) (string, bool) {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(b), true
	}
	return "", false
}

// TestBundleSkipsAnUnreadableLauncherFolderAndSaysSo: a support bundle is best effort.
// A launcher folder that cannot be walked is left out, the bundle is still written
// with the logs of the Grid home, and skipped.txt names what was skipped and why.
func TestBundleSkipsAnUnreadableLauncherFolderAndSaysSo(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this login cannot read")
	}
	gh, machine := t.TempDir(), t.TempDir()
	writeLog(t, filepath.Join(gh, "logs", "grid-open.log"), "home log\n")
	writeLog(t, filepath.Join(machine, "launcher", "alice", "server.log"), "server\n")
	locked := filepath.Join(machine, "launcher", "alice")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o700) })

	path, err := Bundle(BundleOptions{GridHome: gh, Machine: machineAt(machine), Login: "alice", Out: filepath.Join(t.TempDir(), "bundle.zip")})
	if err != nil {
		t.Fatalf("an unreadable launcher folder must not stop the bundle: %v", err)
	}
	got := strings.Join(bundleNames(t, path), ",")
	if want := "doctor.txt,logs/grid-open.log,skipped.txt,versions.txt"; got != want {
		t.Fatalf("bundle holds %s, want %s", got, want)
	}
	line, ok := zipEntry(t, path, "skipped.txt")
	if !ok || !strings.Contains(line, "skipped the launcher logs of alice") || !strings.Contains(line, "permission denied") {
		t.Fatalf("skipped.txt = %q, want a line naming the launcher logs of alice and why", line)
	}
}

// TestBundleSkipsALauncherThatHasNoPlainLoginName: Launcher(login) failing is the
// other skip. The login is not one plain name, so nothing is walked.
func TestBundleSkipsALauncherThatHasNoPlainLoginName(t *testing.T) {
	gh, machine := t.TempDir(), t.TempDir()
	writeLog(t, filepath.Join(gh, "logs", "grid-open.log"), "home log\n")
	path, err := Bundle(BundleOptions{GridHome: gh, Machine: machineAt(machine), Login: "a/b", Out: filepath.Join(t.TempDir(), "bundle.zip")})
	if err != nil {
		t.Fatalf("a login that is not a plain name must not stop the bundle: %v", err)
	}
	line, ok := zipEntry(t, path, "skipped.txt")
	if !ok || !strings.Contains(line, "skipped the launcher logs of a/b") || !strings.Contains(line, "not one plain name") {
		t.Fatalf("skipped.txt = %q", line)
	}
}
