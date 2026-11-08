package question

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestConcurrentFirstWritesKeepOneTaskULID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "task.md")
	original := []byte("---\nid: T-01\n# Keep this comment\nextra: [one, two]\n---\n\n## Outcome\nKeep this body.\n")
	if err := os.WriteFile(path, original, 0640); err != nil {
		t.Fatal(err)
	}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + questionMachine}
	s := Store{TaskDir: dir, Writer: w}
	if _, err := s.List(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, original) {
		t.Fatalf("reader changed task: %q, %v", got, err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, err := s.Add(input())
		errs <- err
	}()
	go func() {
		<-start
		_, err := (feed.Store{TaskDir: dir}).Post(feed.Input{Type: "question", Body: map[string]any{"question": "Also continue?", "who": "owner"}}, w)
		errs <- err
	}()
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
	if t.Failed() {
		return
	}
	id, err := feed.TaskID(dir)
	if err != nil || !feed.IsID(id) {
		t.Fatalf("task identity: %q, %v", id, err)
	}
	got, err := os.ReadFile(path)
	if err != nil || bytes.Count(got, []byte("ulid: ")) != 1 || !bytes.Equal(bytes.Replace(got, []byte("ulid: "+id+"\n"), nil, 1), original) {
		t.Fatalf("task content changed beyond identity: %q, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0640 {
		t.Fatalf("task permissions changed: %v, %v", info, err)
	}
	add(t, s, input())
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, got) {
		t.Fatalf("later write changed identity: %q, %v", after, err)
	}
	read, err := (feed.Store{TaskDir: dir, TaskID: id}).ReadOnly(feed.Filter{})
	if err != nil || len(read.Records) != 3 {
		t.Fatalf("Feed: %+v, %v", read, err)
	}
	for _, entry := range read.Records {
		if entry.Task != id || entry.Verdict != "counted" {
			t.Fatalf("writer used a different task identity: %+v", entry)
		}
	}
}
