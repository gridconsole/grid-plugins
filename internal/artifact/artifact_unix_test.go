//go:build linux || darwin

package artifact

import (
	"path/filepath"
	"syscall"
	"testing"
)

func TestImportRefusesPipe(t *testing.T) {
	root := t.TempDir()
	pipe := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(pipe, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{TaskPath: root}).Import(pipe, ""); err == nil {
		t.Fatal("import accepted a pipe")
	}
}
