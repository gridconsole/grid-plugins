package cli

import (
	"encoding/json"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestTaskNewThenQuestionAdd(t *testing.T) {
	h := newHarness(t)
	h.env.Machine = ""
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_WORKER", "")
	if code := h.run(h.home, "task", "new", "Choose the export format", "--project", "personal", "--json"); code != 0 {
		t.Fatalf("new: %d %s", code, h.errb)
	}
	var created struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	// Check creation separately: lazy initialization must not hide a missing ID.
	id, err := feed.TaskID(created.Path)
	if err != nil || !feed.IsID(id) {
		t.Errorf("new task identity: %q, %v", id, err)
	}
	q := questionAdd(t, h, created.Path, "member")
	read, err := (feed.Store{TaskDir: created.Path, TaskID: id}).ReadOnly(feed.Filter{})
	if err != nil || len(read.Records) != 1 || read.Records[0].ID != q.Post || read.Records[0].Task != id {
		t.Fatalf("question Feed: %+v, %v", read, err)
	}
}
