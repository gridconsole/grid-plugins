package task

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

func createdHistoryLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("history.md does not end in a newline: %q", b)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

// A hook line on a fresh task used to be dropped: Create writes no history.md
// and AppendHistory returned nil when the file was missing, so the "brain read"
// line the scorecard looks for never landed before the first journal write.
func TestAppendHistoryCreatesHistoryInAFreshTask(t *testing.T) {
	_, tk := writeFixture(t)
	path := filepath.Join(tk.Path, "history.md")
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("a fresh task already has history.md: %v", err)
	}
	if err := AppendHistory(tk.Path, "brain read", "brain:project/deploy", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	lines := createdHistoryLines(t, path)
	if len(lines) != 1 {
		t.Fatalf("want exactly one line, got %q", lines)
	}
	// The timestamp is the only part that moves; the rest is the HistoryLine shape.
	stamp := strings.TrimPrefix(lines[0], "- ")
	if len(stamp) < 16 {
		t.Fatalf("line has no timestamp: %q", lines[0])
	}
	want := taskfile.HistoryLine(stamp[:16], "brain read", "brain:project/deploy", "grid-builder")
	if lines[0] != want {
		t.Fatalf("line %q, want %q", lines[0], want)
	}
	if runtime.GOOS != "windows" {
		if got := fileMode(t, path); got != 0o600 {
			t.Errorf("new history.md mode %04o, want 0600", got)
		}
	}

	// A different event appends a second line.
	if err := AppendHistory(tk.Path, "subagent finished", "", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	lines = createdHistoryLines(t, path)
	if len(lines) != 2 || lines[0] != want || !strings.Contains(lines[1], " · subagent finished · by grid-builder") {
		t.Fatalf("second event: %q", lines)
	}

	// The same line again is still the consecutive duplicate it always was.
	before, _ := os.ReadFile(path)
	if err := AppendHistory(tk.Path, "subagent finished", "", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("a repeated line was appended:\n%s", after)
	}
	if runtime.GOOS != "windows" {
		if got := fileMode(t, path); got != 0o600 {
			t.Errorf("history.md mode %04o after appends, want 0600", got)
		}
	}
}

// Only a task folder gets a history.md of its own: one that holds a regular
// task.md. Anything else keeps the old answer, nil with nothing written.
func TestAppendHistoryNeedsARegularTaskFile(t *testing.T) {
	s, tk := writeFixture(t)
	root := t.TempDir()
	outside := filepath.Join(root, "outside.md")
	if err := os.WriteFile(outside, []byte("---\nstate: queued\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlinked := filepath.Join(root, "symlinked")
	mkdir(t, symlinked)
	if err := os.Symlink(outside, filepath.Join(symlinked, "task.md")); err != nil {
		t.Logf("symlinks unavailable: %v", err)
		symlinked = ""
	}
	taskDir := filepath.Join(root, "task-md-is-a-directory")
	mkdir(t, filepath.Join(taskDir, "task.md"))
	for name, dir := range map[string]string{
		"plain temp dir":      filepath.Join(root, "plain"),
		"grid home":           s.Resolver.GridHome,
		"project folder":      filepath.Dir(tk.Path),
		"task.md a directory": taskDir,
		"task.md a symlink":   symlinked,
	} {
		if dir == "" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			mkdir(t, dir)
			if err := AppendHistory(dir, "brain read", "brain:project/deploy", "grid-builder", ""); err != nil {
				t.Fatalf("want nil as on main, got %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, "history.md")); !os.IsNotExist(err) {
				t.Fatalf("history.md was created: %v", err)
			}
			if _, err := os.Lstat(filepath.Join(dir, ".task.lock")); !os.IsNotExist(err) {
				t.Fatalf(".task.lock was created in a folder that is not a task: %v", err)
			}
		})
	}
	if b, _ := os.ReadFile(outside); string(b) != "---\nstate: queued\n---\n" {
		t.Errorf("the file behind the task.md symlink changed: %q", b)
	}
}

// A history.md that is not a regular file is refused whether or not the task
// has one yet, and nothing is written through it.
func TestAppendHistoryStillRefusesALinkedHistory(t *testing.T) {
	_, tk := writeFixture(t)
	path := filepath.Join(tk.Path, "history.md")
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, name := range []string{"existing target", "dangling target"} {
		if name == "dangling target" {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			target = filepath.Join(filepath.Dir(target), "missing")
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
		}
		err := AppendHistory(tk.Path, "brain read", "brain:project/deploy", "grid-builder", "")
		if err == nil || !strings.Contains(err.Error(), "history must be a regular file") {
			t.Fatalf("%s: got %v, want the regular-file refusal", name, err)
		}
		if info, err := os.Lstat(path); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s: the symlink was replaced: %v, %v", name, info, err)
		}
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Errorf("a write went through the dangling symlink: %v", err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	mkdir(t, path)
	if err := AppendHistory(tk.Path, "brain read", "brain:project/deploy", "grid-builder", ""); err == nil || !strings.Contains(err.Error(), "history must be a regular file") {
		t.Fatalf("directory: got %v, want the regular-file refusal", err)
	}
	if info, err := os.Lstat(path); err != nil || !info.IsDir() {
		t.Fatalf("history.md directory was replaced: %v, %v", info, err)
	}
}

// Twenty hook lines on a fresh task at once: the first creates history.md under
// .task.lock and the rest append to it. None is lost and none fails.
func TestAppendHistoryCreateIsSafeUnderConcurrency(t *testing.T) {
	_, tk := writeFixture(t)
	const count = 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := AppendHistory(tk.Path, fmt.Sprintf("hook concurrent %02d", i), "", "worker", ""); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()
	lines := createdHistoryLines(t, filepath.Join(tk.Path, "history.md"))
	if len(lines) != count {
		t.Fatalf("want %d lines, got %d:\n%s", count, len(lines), strings.Join(lines, "\n"))
	}
	seen := map[string]bool{}
	for i := 0; i < count; i++ {
		marker := fmt.Sprintf(" · hook concurrent %02d · by worker", i)
		n := 0
		for _, l := range lines {
			if strings.Contains(l, marker) {
				n++
				seen[l] = true
			}
		}
		if n != 1 {
			t.Errorf("hook concurrent %02d appears %d times", i, n)
		}
	}
	if len(seen) != count {
		t.Errorf("want %d distinct lines, got %d", count, len(seen))
	}
	entries, err := os.ReadDir(tk.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".write-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

// The journal writers read the file that AppendHistory created and append
// after what it holds; they never start the file over.
func TestJournalWriteKeepsLinesAppendHistoryCreated(t *testing.T) {
	s, tk := writeFixture(t)
	path := filepath.Join(tk.Path, "history.md")
	if err := AppendHistory(tk.Path, "brain read", "brain:project/deploy", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Note(WriteOptions{ID: tk.ID, Note: "after the hook line"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendHistory(tk.Path, "subagent finished", "", "grid-builder", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Change(WriteOptions{ID: tk.ID, Action: "close"}); err != nil {
		t.Fatal(err)
	}
	lines := createdHistoryLines(t, path)
	if len(lines) != 4 {
		t.Fatalf("want 4 lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	for i, want := range []string{" · brain read · brain:project/deploy · by grid-builder", "after the hook line", " · subagent finished · by grid-builder", `"close"`} {
		if !strings.Contains(lines[i], want) {
			t.Errorf("line %d %q lacks %q", i, lines[i], want)
		}
	}
	if runtime.GOOS != "windows" {
		if got := fileMode(t, path); got != 0o600 {
			t.Errorf("history.md mode %04o after journal writes, want 0600", got)
		}
	}
}
