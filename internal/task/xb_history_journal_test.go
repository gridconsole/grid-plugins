package task

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// xbEvent is the event line E that a task writer put into its journal image and
// then died before the replay ended.
const xbEvent = `- {"task":"T-01","what":"note","note":"event E from the crashed writer"}`

func xbRead(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// xbJournal writes .task-write.json by hand in commitWrite's shape: a JSON map of
// file name to the whole file image. It returns the bytes it wrote.
func xbJournal(t *testing.T, dir string, files map[string][]byte) []byte {
	t.Helper()
	data, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, journalName), string(data))
	return data
}

func xbNoJournal(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(dir, journalName)); !os.IsNotExist(err) {
		t.Fatalf("journal is still there: %v", err)
	}
}

// A task writer died after writing its journal and before the replay ended. A hook
// appends a line, then the next task write replays: the appended line must survive
// and the crashed writer's event must come first.
func TestXbAppendKeepsLineAfterJournal(t *testing.T) {
	for _, withTask := range []bool{false, true} {
		name := "history only"
		if withTask {
			name = "task and history"
		}
		t.Run(name, func(t *testing.T) {
			s, task := writeFixture(t)
			old := "- 2026-01-02T03:04 · legacy line · by alex\n"
			write(t, filepath.Join(task.Path, "history.md"), old)
			files := map[string][]byte{"history.md": []byte(old + xbEvent + "\n")}
			if withTask {
				data := xbRead(t, task.Path, "task.md")
				files["task.md"] = bytes.Replace(data, []byte("state: queued"), []byte("state: closed"), 1)
			}
			xbJournal(t, task.Path, files)

			if err := AppendHistory(task.Path, "subagent finished", "", "grid-builder", ""); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Note(WriteOptions{ID: task.ID, Note: "after the append"}); err != nil {
				t.Fatal(err)
			}

			lines := historyLines(t, task.Path)
			if len(lines) != 4 {
				t.Fatalf("history has %d lines, want 4: %q", len(lines), lines)
			}
			if lines[0] != strings.TrimSuffix(old, "\n") || lines[1] != xbEvent {
				t.Fatalf("old line and E are not first: %q", lines)
			}
			stamp := strings.TrimPrefix(lines[2], "- ")
			if len(stamp) < 16 {
				t.Fatalf("appended line has no timestamp: %q", lines[2])
			}
			if want := taskfile.HistoryLine(stamp[:16], "subagent finished", "", "grid-builder", ""); lines[2] != want {
				t.Fatalf("appended line %q, want %q", lines[2], want)
			}
			if !strings.Contains(lines[3], `"note":"after the append"`) {
				t.Fatalf("note is not last: %q", lines)
			}
			text := string(xbRead(t, task.Path, "history.md"))
			for _, once := range []string{"event E from the crashed writer", "subagent finished"} {
				if n := strings.Count(text, once); n != 1 {
					t.Fatalf("%q appears %d times, want once:\n%s", once, n, text)
				}
			}
			xbNoJournal(t, task.Path)
			if withTask {
				if got, err := Read(task.Path); err != nil || got.State != "closed" {
					t.Fatalf("journal task.md was not replayed: %+v %v", got, err)
				}
			}
		})
	}
}

// With no journal the append is the same bytes as before: the old history plus one
// HistoryLine, and an identical second line is still dropped.
func TestXbAppendNoJournalUnchanged(t *testing.T) {
	_, task := writeFixture(t)
	path := filepath.Join(task.Path, "history.md")
	write(t, path, "- legacy entry\n")
	if err := AppendHistory(task.Path, "subagent finished", "ok", "grid-builder", "hook"); err != nil {
		t.Fatal(err)
	}
	got := string(xbRead(t, task.Path, "history.md"))
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 2 || lines[0] != "- legacy entry" {
		t.Fatalf("%q", lines)
	}
	stamp := strings.TrimPrefix(lines[1], "- ")
	if len(stamp) < 16 {
		t.Fatalf("appended line has no timestamp: %q", lines[1])
	}
	want := "- legacy entry\n" + taskfile.HistoryLine(stamp[:16], "subagent finished", "ok", "grid-builder", "hook") + "\n"
	if got != want {
		t.Fatalf("history is %q, want %q", got, want)
	}

	if err := AppendHistory(task.Path, "subagent finished", "ok", "grid-builder", "hook"); err != nil {
		t.Fatal(err)
	}
	if again := string(xbRead(t, task.Path, "history.md")); again != got {
		t.Fatalf("an identical second line was not dropped: %q", again)
	}

	// The why is part of the repeat check: the same line with another why is new.
	if err := AppendHistoryWhy(task.Path, "subagent finished", "ok", "grid-builder", "hook", "retry"); err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSuffix(string(xbRead(t, task.Path, "history.md")), "\n"), "\n")
	if len(lines) != 3 || !strings.HasSuffix(lines[2], " · via hook · why retry") {
		t.Fatalf("%q", lines)
	}
	xbNoJournal(t, task.Path)
}

// Outside a task folder the journal is none of AppendHistory's business: it
// returns nil, creates no history.md and leaves the journal byte-equal.
func TestXbAppendNoTaskNoJournalTouch(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T, dir string)
	}{
		{"no task.md", func(t *testing.T, dir string) {}},
		{"task.md is a directory", func(t *testing.T, dir string) { mkdir(t, filepath.Join(dir, "task.md")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			journal := xbJournal(t, dir, map[string][]byte{"history.md": []byte(xbEvent + "\n")})
			if err := AppendHistory(dir, "subagent finished", "", "grid-builder", ""); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(dir, "history.md")); !os.IsNotExist(err) {
				t.Fatalf("history.md was written: %v", err)
			}
			if got := xbRead(t, dir, journalName); !bytes.Equal(got, journal) {
				t.Fatalf("journal changed: %q", got)
			}
		})
	}
	t.Run("history.md without task.md", func(t *testing.T) {
		dir := t.TempDir()
		write(t, filepath.Join(dir, "history.md"), "- legacy entry\n")
		journal := xbJournal(t, dir, map[string][]byte{"history.md": []byte(xbEvent + "\n")})
		if err := AppendHistory(dir, "subagent finished", "", "grid-builder", ""); err != nil {
			t.Fatal(err)
		}
		if got := xbRead(t, dir, journalName); !bytes.Equal(got, journal) {
			t.Fatalf("journal changed: %q", got)
		}
		if strings.Contains(string(xbRead(t, dir, "history.md")), "event E") {
			t.Fatal("the journal was replayed outside a task folder")
		}
	})
}

// A journal that cannot be replayed is the replay's error. Nothing is appended
// past it and history.md and the journal stay byte-equal.
func TestXbAppendCorruptJournal(t *testing.T) {
	for _, tc := range []struct {
		name    string
		journal func(t *testing.T, dir string) []byte
		want    string
	}{
		{"not JSON", func(t *testing.T, dir string) []byte {
			write(t, filepath.Join(dir, journalName), "this is not json")
			return []byte("this is not json")
		}, "task write journal"},
		{"no history image", func(t *testing.T, dir string) []byte {
			return xbJournal(t, dir, map[string][]byte{"task.md": []byte("x")})
		}, "incomplete task write journal"},
		{"unknown file", func(t *testing.T, dir string) []byte {
			return xbJournal(t, dir, map[string][]byte{"history.md": []byte(xbEvent + "\n"), "notes.txt": []byte("x")})
		}, "invalid journal file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, task := writeFixture(t)
			write(t, filepath.Join(task.Path, "history.md"), "- legacy entry\n")
			journal := tc.journal(t, task.Path)
			err := AppendHistory(task.Path, "subagent finished", "", "grid-builder", "")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %v, want %q", err, tc.want)
			}
			if got := string(xbRead(t, task.Path, "history.md")); got != "- legacy entry\n" {
				t.Fatalf("history.md changed: %q", got)
			}
			if got := xbRead(t, task.Path, journalName); !bytes.Equal(got, journal) {
				t.Fatalf("journal changed: %q", got)
			}
		})
	}
}
