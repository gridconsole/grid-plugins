//go:build linux || darwin

package artifact

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// importWithin fails the test if Import has not returned by the deadline, which is
// what a blocking open of a pipe looks like.
func importWithin(t *testing.T, s Store, source string) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		_, err := s.Import(source, "")
		done <- err
	}()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("import blocked on " + source)
		return nil
	}
}

func TestImportRefusesPipe(t *testing.T) {
	root := t.TempDir()
	pipe := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	err := importWithin(t, Store{TaskPath: root}, pipe)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("import of a pipe: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatal("refused import wrote artifacts")
	}
}

func TestImportRefusesDevice(t *testing.T) {
	err := importWithin(t, Store{TaskPath: t.TempDir()}, os.DevNull)
	if err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("import of a device: %v", err)
	}
}

func TestImportRefusesSymlinkSource(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.txt")
	link := filepath.Join(root, "link.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	err := importWithin(t, Store{TaskPath: root}, link)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("import of a symlink: %v", err)
	}
}

func TestImportRefusesUnusableName(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, `a\b.txt`)
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{TaskPath: root}).Import(source, ""); err == nil || !strings.Contains(err.Error(), "artifact file name") {
		t.Fatalf("import: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatal("refused import wrote artifacts")
	}
}
