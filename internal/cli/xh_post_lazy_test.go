package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func xhExists(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, name))
	return err == nil
}

// A post on a task that has published nothing and has no open question adds its
// one record to feed/ and no projection file: the app counts the files of feed/ as
// the task's lines, and a reader folds when needs-you.json is absent.
func TestXhPostMakesNoProjectionFile(t *testing.T) {
	h, task := feedHarness(t)
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	for i := 0; i < 2; i++ {
		if code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"example"}`, "--json"); code != 0 {
			t.Fatalf("post %d: exit %d: %s", i, code, h.errb)
		}
		for _, name := range []string{"state.json", "needs-you.json"} {
			if xhExists(store.Dir(), name) {
				t.Fatalf("post %d made feed/%s on a task with nothing published and no question", i, name)
			}
		}
	}
}

// A question opens needs-you.json; once it is there every later post keeps it
// and state.json current, so a published file is never stale.
func TestXhPostRefreshesPublishedFiles(t *testing.T) {
	h, task := feedHarness(t)
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	if code := h.run(task, "feed", "post", "--type", "question", "--body", `{"question":"Continue?","who":"member"}`, "--json"); code != 0 {
		t.Fatalf("question: exit %d: %s", code, h.errb)
	}
	if !xhExists(store.Dir(), "needs-you.json") {
		t.Fatal("an open question left no feed/needs-you.json")
	}
	if code := h.run(task, "feed", "state", "--json"); code != 0 {
		t.Fatalf("state: exit %d: %s", code, h.errb)
	}
	if !xhExists(store.Dir(), "state.json") {
		t.Fatal("feed state left no feed/state.json")
	}
	before, err := os.ReadFile(filepath.Join(store.Dir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"later"}`, "--json"); code != 0 {
		t.Fatalf("post: exit %d: %s", code, h.errb)
	}
	after, err := os.ReadFile(filepath.Join(store.Dir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) == string(after) {
		t.Fatalf("state.json is stale after a post: %s", after)
	}
}
