package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateMachine(t *testing.T) {
	states := []string{"queued", "running", "waiting", "stuck", "reworking", "pausing", "paused", "closed", "unknown"}
	expected := map[string]string{
		"queued":    "running waiting paused stuck",
		"running":   "waiting stuck reworking pausing paused",
		"waiting":   "running queued stuck reworking paused",
		"stuck":     "running queued waiting paused",
		"reworking": "running waiting stuck pausing paused",
		"pausing":   "paused running stuck",
		"paused":    "queued running",
	}
	for _, from := range states {
		for _, action := range []string{"status", "stop", "close", "reopen", "unknown"} {
			for _, to := range states {
				t.Run(from+"/"+action+"/"+to, func(t *testing.T) {
					want := ""
					switch action {
					case "status":
						if strings.Contains(" "+expected[from]+" ", " "+to+" ") {
							want = to
						}
					case "stop", "close":
						if from != "closed" && from != "unknown" {
							want = "closed"
						}
					case "reopen":
						if from == "closed" {
							want = "queued"
						}
					}
					got, err := NextState(from, action, to)
					if got != want || (err != nil) != (want == "") {
						t.Fatalf("got %q, %v; want %q", got, err, want)
					}
					if err != nil && !strings.Contains(err.Error(), `current state "`+from+`"`) {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func writeFixture(t *testing.T) (Store, Task) {
	t.Helper()
	s, home := storeFor(t)
	task, err := s.New(NewOptions{Cwd: home, Words: "Example task", Project: "personal"})
	if err != nil {
		t.Fatal(err)
	}
	return s, task
}

func TestChangePreservesTaskAndHistory(t *testing.T) {
	s, task := writeFixture(t)
	path := filepath.Join(task.Path, "task.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(data), "state: queued", "# Keep this comment\nextra: [one, two]\nstate: queued", 1) + "\n## Outcome\nKeep this body exactly.\n"
	write(t, path, text)
	write(t, filepath.Join(task.Path, "history.md"), "- legacy entry\n")
	event, err := s.Change(WriteOptions{ID: task.ID, Action: "close", Note: "done\nwith · punctuation"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Read(task.Path)
	if err != nil || got.State != "closed" || got.Intent != task.Intent {
		t.Fatalf("%+v: %v", got, err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"# Keep this comment", "extra: [one, two]", "\n## Outcome\nKeep this body exactly.\n"} {
		if !strings.Contains(string(data), keep) {
			t.Fatalf("lost %q: %s", keep, data)
		}
	}
	before := append([]byte{}, data...)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err == nil {
		t.Fatal("closed a closed task")
	}
	data, _ = os.ReadFile(path)
	if !bytes.Equal(before, data) {
		t.Fatal("failed write changed task")
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != 2 || lines[0] != "- legacy entry" {
		t.Fatalf("%v: %v", lines, err)
	}
	var recorded Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(lines[1], "- ")), &recorded); err != nil || recorded != event {
		t.Fatalf("%+v, %v", recorded, err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
		t.Fatal("journal was not removed")
	}
}

func TestWriteRecovery(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			s, task := writeFixture(t)
			data, _ := os.ReadFile(filepath.Join(task.Path, "task.md"))
			changed := bytes.Replace(data, []byte("state: queued"), []byte("state: closed"), 1)
			files := map[string][]byte{"task.md": changed, "history.md": []byte("- committed close\n")}
			journal, _ := json.Marshal(files)
			write(t, filepath.Join(task.Path, journalName), string(journal))
			if partial {
				write(t, filepath.Join(task.Path, "task.md"), string(changed))
			}
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "reopen"}); err != nil {
				t.Fatal(err)
			}
			lines, err := s.History("", task.ID)
			if err != nil || len(lines) != 2 || lines[0] != "- committed close" {
				t.Fatalf("%v: %v", lines, err)
			}
			if _, err := s.History("", task.ID); err != nil {
				t.Fatal(err)
			}
			again, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
			if strings.Count(string(again), "committed close") != 1 {
				t.Fatal("duplicated recovery")
			}
		})
	}
}

func TestWriteRefusesSymlinks(t *testing.T) {
	for _, name := range []string{".task.lock", journalName, "history.md", "task.md"} {
		t.Run(name, func(t *testing.T) {
			s, task := writeFixture(t)
			target := filepath.Join(filepath.Dir(task.Path), "outside")
			original := "outside\n"
			if name == "task.md" {
				data, _ := os.ReadFile(filepath.Join(task.Path, name))
				original = string(data)
				if err := os.Remove(filepath.Join(task.Path, name)); err != nil {
					t.Fatal(err)
				}
			}
			write(t, target, original)
			if err := os.Symlink(target, filepath.Join(task.Path, name)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err == nil {
				t.Fatal("followed a symlink")
			}
			got, _ := os.ReadFile(target)
			if string(got) != original {
				t.Fatal("changed outside file")
			}
		})
	}
}

// Two independently exec'd writers repeatedly contend on the same state. Each
// successful write prints a receipt; rejected moves print nothing. The parent
// compares receipts to disk and checks the entire state chain, not just counts.
func TestConcurrentCloseReopen(t *testing.T) {
	flakeCRRun(t, []string{"close", "reopen"}, false)
}
