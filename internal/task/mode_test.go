package task

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func chmodFile(t *testing.T, path string, perm os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, perm); err != nil {
		t.Fatal(err)
	}
	if got := fileMode(t, path); got != perm {
		t.Fatalf("%s mode %04o after chmod, want %04o", filepath.Base(path), got, perm)
	}
}

// A rewrite keeps the mode of the file it replaces. AppendHistory renames
// history.md into place since FP1 (2792be12), and the journal writers rename
// task.md and history.md; neither may change who can read a person's file.
func TestRewriteKeepsTheFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file modes carry only a read-only bit")
	}
	for _, perm := range []os.FileMode{0o664, 0o640, 0o644} {
		t.Run(fmt.Sprintf("AppendHistory/%04o", perm), func(t *testing.T) {
			_, tk := writeFixture(t)
			seedHistory(t, tk.Path)
			path := filepath.Join(tk.Path, "history.md")
			chmodFile(t, path, perm)
			before, _ := os.ReadFile(path)
			if err := AppendHistory(tk.Path, "subagent finished", "", "grid-builder", ""); err != nil {
				t.Fatal(err)
			}
			if after, _ := os.ReadFile(path); len(after) <= len(before) {
				t.Fatalf("AppendHistory appended nothing: %q", after)
			}
			if got := fileMode(t, path); got != perm {
				t.Errorf("history.md mode %04o after AppendHistory, want %04o", got, perm)
			}
		})
		t.Run(fmt.Sprintf("journal/%04o", perm), func(t *testing.T) {
			s, tk := writeFixture(t)
			seedHistory(t, tk.Path)
			for _, name := range []string{"task.md", "history.md"} {
				chmodFile(t, filepath.Join(tk.Path, name), perm)
			}
			if _, err := s.Change(WriteOptions{ID: tk.ID, Action: "close"}); err != nil {
				t.Fatal(err)
			}
			if got, err := Read(tk.Path); err != nil || got.State != "closed" {
				t.Fatalf("close did not land: %+v, %v", got, err)
			}
			for _, name := range []string{"task.md", "history.md"} {
				if got := fileMode(t, filepath.Join(tk.Path, name)); got != perm {
					t.Errorf("%s mode %04o after a journal write, want %04o", name, got, perm)
				}
			}
		})
	}
}

// Control: a file Grid creates keeps the mode main gave it. A journal write
// creates history.md and a receipt 0600, AppendHistory keeps that, and the task
// lock it takes is 0600.
func TestNewFilesKeepTheirMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows file modes carry only a read-only bit")
	}
	_, tk := writeFixture(t)
	history := filepath.Join(tk.Path, "history.md")
	receipt := filepath.Join(tk.Path, approvalFile)
	for _, path := range []string{history, receipt} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("fixture already has %s: %v", filepath.Base(path), err)
		}
	}
	if err := commitWrite(tk.Path, Event{Task: tk.ID, What: "note", Note: "x"}, map[string][]byte{approvalFile: []byte("{}\n")}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{history, receipt} {
		if got := fileMode(t, path); got != 0o600 {
			t.Errorf("new %s mode %04o, want 0600", filepath.Base(path), got)
		}
	}
	if err := AppendHistory(tk.Path, "subagent finished", "", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	if got := fileMode(t, history); got != 0o600 {
		t.Errorf("history.md mode %04o after AppendHistory, want 0600", got)
	}
	if got := fileMode(t, filepath.Join(tk.Path, ".task.lock")); got != 0o600 {
		t.Errorf(".task.lock mode %04o, want 0600", got)
	}
}
