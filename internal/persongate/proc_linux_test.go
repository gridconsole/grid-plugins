//go:build linux

package persongate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLinuxExeFallback(t *testing.T) {
	root := t.TempDir()
	pid := os.Getpid()
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "stat"), []byte(fmt.Sprintf("%d (grid with ) parens) S 0 0 0", pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte("codex\x00run\x00"), 0600); err != nil {
		t.Fatal(err)
	}
	tree, err := linuxSnapshot(root)
	if err != nil || tree[pid].Exe != "codex" || len(tree[pid].Args) != 2 {
		t.Fatalf("%+v %v", tree, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := linuxSnapshot(root); err == nil {
		t.Fatal("unreadable exe without cmdline passed")
	}
}
