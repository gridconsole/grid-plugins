package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func historyLines(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestNoteRecordsOneLine(t *testing.T) {
	for _, tc := range []struct {
		note string
		ok   bool
	}{{"built item 1", true}, {"has · punctuation, and \"quotes\"", true}, {"", false}, {"  ", false}, {"two\nlines", false}, {"carriage\rreturn", false}, {"null\x00", false}} {
		t.Run(fmt.Sprintf("%q", tc.note), func(t *testing.T) {
			s, task := writeFixture(t)
			before, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
			event, err := s.Note(WriteOptions{ID: task.ID, Note: tc.note})
			if tc.ok != (err == nil) {
				t.Fatalf("ok=%v: %v", tc.ok, err)
			}
			after, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
			if !tc.ok {
				if string(before) != string(after) {
					t.Fatal("refused note wrote history")
				}
				return
			}
			if event.What != "note" || event.Note != tc.note || event.FromState != "queued" || event.ToState != "queued" || event.Who.ID != "local" {
				t.Fatalf("%+v", event)
			}
			lines := historyLines(t, task.Path)
			var recorded Event
			if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[len(lines)-1], "- ")), &recorded); err != nil || recorded != event {
				t.Fatalf("%+v, %v", recorded, err)
			}
			got, err := Read(task.Path)
			if err != nil || got.State != "queued" {
				t.Fatalf("a note changed the task: %+v %v", got, err)
			}
			if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
				t.Fatal("journal was not removed")
			}
		})
	}
}

// One lock covers state moves and notes: neither side loses a line to the other.
// (Before notes used the task writer, a second history lock lost 29 to 39 of 121.)
func TestNoteAndChangeLoseNoLines(t *testing.T) {
	s, task := writeFixture(t)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: "running"}); err != nil {
		t.Fatal(err)
	}
	base := len(historyLines(t, task.Path))
	const n = 60
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		next := []string{"waiting", "running"}
		for i := 0; i < n; i++ {
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: next[i%2]}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < n; i++ {
			if _, err := s.Note(WriteOptions{ID: task.ID, Note: fmt.Sprintf("note %d", i)}); err != nil {
				t.Error(err)
				return
			}
		}
	}()
	wg.Wait()
	lines := historyLines(t, task.Path)
	if len(lines) != base+2*n {
		t.Fatalf("history has %d lines, want %d", len(lines), base+2*n)
	}
	seen := map[string]int{}
	for _, line := range lines[base:] {
		var e Event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "- ")), &e); err != nil {
			t.Fatal(err)
		}
		if e.What == "note" {
			seen[e.Note]++
		}
	}
	for i := 0; i < n; i++ {
		if seen[fmt.Sprintf("note %d", i)] != 1 {
			t.Fatalf("note %d appears %d times", i, seen[fmt.Sprintf("note %d", i)])
		}
	}
}

// A note writes history alone, so a crash leaves a one-file journal. The next
// writer replays it before its own write; a replay never overwrites a later line.
func TestNoteJournalIsReplayed(t *testing.T) {
	s, task := writeFixture(t)
	journal, _ := json.Marshal(map[string][]byte{"history.md": []byte("- committed note\n")})
	write(t, filepath.Join(task.Path, journalName), string(journal))
	if _, err := s.Note(WriteOptions{ID: task.ID, Note: "after the crash"}); err != nil {
		t.Fatal(err)
	}
	lines := historyLines(t, task.Path)
	if len(lines) != 2 || lines[0] != "- committed note" || !strings.Contains(lines[1], `"note":"after the crash"`) {
		t.Fatalf("%v", lines)
	}
	if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
		t.Fatal("journal was not removed")
	}
	// A journal with no history image is still refused as incomplete.
	empty, _ := json.Marshal(map[string][]byte{"task.md": []byte("x")})
	write(t, filepath.Join(task.Path, journalName), string(empty))
	if _, err := s.Note(WriteOptions{ID: task.ID, Note: "refused"}); err == nil || !strings.Contains(err.Error(), "incomplete task write journal") {
		t.Fatal(err)
	}
}

func TestNoteOnClosedTask(t *testing.T) {
	s, task := writeFixture(t)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Note(WriteOptions{ID: task.ID, Note: "late remark"}); err != nil {
		t.Fatal(err)
	}
	if got, err := Read(task.Path); err != nil || got.State != "closed" {
		t.Fatalf("%+v %v", got, err)
	}
}
