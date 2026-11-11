//go:build windows

package artifact

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const notASymlink = "source must be a regular file, not a symlink"

func windowsTarget(t *testing.T) (root, file string) {
	t.Helper()
	root = t.TempDir()
	file = filepath.Join(root, "target.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, file
}

func refusedWith(t *testing.T, what string, s Store, source, want string) {
	t.Helper()
	if _, err := s.Import(source, ""); err == nil || err.Error() != want {
		t.Fatalf("Import of %s: %v, want %q", what, err, want)
	}
	if err := s.CheckImport(source); err == nil || err.Error() != want {
		t.Fatalf("CheckImport of %s: %v, want %q", what, err, want)
	}
	if _, err := os.Stat(filepath.Join(s.TaskPath, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("refused import of %s wrote artifacts", what)
	}
}

func TestImportWindowsRefusesSymlinkSource(t *testing.T) {
	root, target := windowsTarget(t)
	link := filepath.Join(root, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("this account cannot create a symlink (needs the symlink privilege or developer mode): " + err.Error())
	}
	refusedWith(t, "a file symlink", Store{TaskPath: t.TempDir()}, link, notASymlink)
}

func TestImportWindowsRefusesJunctionSource(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "real")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(root, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, dir).CombinedOutput(); err != nil {
		t.Skip("cmd /c mklink /J failed: " + err.Error() + ": " + string(out))
	}
	refusedWith(t, "a directory junction", Store{TaskPath: t.TempDir()}, junction, notASymlink)
}

func TestImportWindowsPlainFolderIsNotARegularFile(t *testing.T) {
	refusedWith(t, "a plain folder", Store{TaskPath: t.TempDir()}, t.TempDir(), "source must be a regular file")
}

func TestImportWindowsPlainFileSucceeds(t *testing.T) {
	_, target := windowsTarget(t)
	s := Store{TaskPath: t.TempDir()}
	if err := s.CheckImport(target); err != nil {
		t.Fatalf("CheckImport of a plain file: %v", err)
	}
	r, err := s.Import(target, "")
	if err != nil {
		t.Fatalf("Import of a plain file: %v", err)
	}
	if r.File == "" || r.SHA256 == "" {
		t.Fatalf("record is incomplete: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(s.TaskPath, "artifacts", r.File)); err != nil {
		t.Fatalf("imported file is missing: %v", err)
	}
}
