package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// A hard link planted outside the task folder before a sandbox starts can still
// write a protected file, so the hold reader, Show and Deliver refuse the task.
func TestALinkedProtectedFileHoldsTheTask(t *testing.T) {
	for _, name := range []string{"task.md", "history.md", approvalFile} {
		t.Run(name, func(t *testing.T) {
			s, tk := deliverFixture(t, "")
			seedHistory(t, tk.Path)
			text, _ := os.ReadFile(filepath.Join(tk.Path, "task.md"))
			history, _ := os.ReadFile(filepath.Join(tk.Path, "history.md"))
			if PlanHeld(tk.Path, string(text), string(history)) {
				t.Fatal("the approved fixture is held before the link")
			}
			if err := os.Link(filepath.Join(tk.Path, name), filepath.Join(t.TempDir(), "t")); err != nil {
				t.Fatal(err)
			}
			if !PlanHeld(tk.Path, string(text), string(history)) {
				t.Errorf("PlanHeld = false after a link to %s", name)
			}
			if _, err := s.Show(tk.Path, tk.ID); err == nil || !strings.Contains(err.Error(), "linked_file") || !strings.Contains(err.Error(), name) {
				t.Errorf("Show err = %v; want linked_file naming %s", err, name)
			}
			if _, err := s.Deliver(DeliverOptions{Cwd: tk.Path, Task: tk.ID, Outcome: "Ready", By: "grid"}); err == nil || !strings.Contains(err.Error(), "linked_file") {
				t.Errorf("Deliver err = %v; want linked_file", err)
			}
		})
	}
}

// Grid's history writer renames a new file into place, so a planted link keeps
// the old inode: it neither sees the new line nor aliases the live file.
func TestAppendHistoryLeavesAPlantedLinkOnTheOldInode(t *testing.T) {
	_, tk := deliverFixture(t, "")
	seedHistory(t, tk.Path)
	path := filepath.Join(tk.Path, "history.md")
	link := filepath.Join(t.TempDir(), "t")
	if err := os.Link(path, link); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(link)
	if err := AppendHistory(tk.Path, "files changed", "notes.md", "grid", ""); err != nil {
		t.Fatal(err)
	}
	live, _ := os.Stat(path)
	planted, _ := os.Stat(link)
	if os.SameFile(live, planted) {
		t.Error("the planted link still aliases history.md after a Grid write")
	}
	if got, _ := os.ReadFile(link); string(got) != string(before) {
		t.Errorf("the planted link changed: %q", got)
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), "files changed · notes.md") {
		t.Errorf("history.md lacks the new line: %q", got)
	}
}

// Appenders serialize on .task.lock, not on the history.md they replace, so no
// rename drops a line another appender wrote.
func TestAppendHistoryKeepsEveryConcurrentLine(t *testing.T) {
	_, tk := deliverFixture(t, "")
	seedHistory(t, tk.Path)
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if err := AppendHistory(tk.Path, "files changed", fmt.Sprintf("w%d-%d.md", w, i), "grid", ""); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	got, _ := os.ReadFile(filepath.Join(tk.Path, "history.md"))
	for w := 0; w < 2; w++ {
		for i := 0; i < 20; i++ {
			if !strings.Contains(string(got), fmt.Sprintf(" · w%d-%d.md · ", w, i)) {
				t.Errorf("history.md lost w%d-%d.md", w, i)
			}
		}
	}
}

func seedHistory(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "history.md"), []byte("- 2026-01-02T03:04 · task created · by grid\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
