package task

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestRecoveryKeepsLazilyAllocatedTaskID(t *testing.T) {
	s, task := writeFixture(t)
	legacyTask(t, task.Path)
	data, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Replace(data, []byte("state: queued"), []byte("state: closed"), 1)
	journal, err := json.Marshal(map[string][]byte{"task.md": changed, "history.md": []byte("- committed close\n")})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(task.Path, journalName), string(journal))
	id, err := feed.EnsureTaskID(task.Path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "reopen"}); err != nil {
		t.Fatal(err)
	}
	got, err := feed.TaskID(task.Path)
	if err != nil || got != id {
		t.Fatalf("recovery replaced identity: %q, want %q: %v", got, id, err)
	}
	read, err := Read(task.Path)
	if err != nil || read.State != "queued" {
		t.Fatalf("recovery failed: %+v, %v", read, err)
	}
}
