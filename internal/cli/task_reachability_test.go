package cli

import (
	"encoding/json"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTaskLeaseFreshHomeSequence(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h := newHarness(t)
	h.env.Machine = "" // Let task new allocate the identity of this fresh home.
	t.Setenv("GRID_HOME", h.gridHome)
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "owner") // Personal is owner-only (L67R).
	for _, key := range []string{"GRID_WORKER", "GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_JOB", "GRID_WAKE", "GRID_ATTEMPT"} {
		t.Setenv(key, "")
	}
	if entries, err := os.ReadDir(h.gridHome); err != nil || len(entries) != 0 {
		t.Fatalf("home is not fresh: %v %v", entries, err)
	}
	if code := h.run(h.home, "task", "new", "Document the export format", "--json"); code != 0 {
		t.Fatalf("task new: %d %s", code, h.errb)
	}
	var created struct{ Path string }
	if err := json.Unmarshal(h.out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	draft := filepath.Join(h.home, "draft.md")
	text, err := os.ReadFile("../../testdata/job/task/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	h.write(draft, "# Plan\n\nRecall: nothing found\n\n"+string(text))
	if code := h.run(created.Path, "plan", "write", draft); code != 0 {
		t.Fatalf("plan write: %d %s", code, h.errb)
	}
	// Default workflows require the person to approve the plan before publication.
	if code := h.run(created.Path, "task", "status", "T-01", "waiting"); code != 0 {
		t.Fatalf("task waiting: %d %s", code, h.errb)
	}
	if code := h.run(created.Path, "approve", "T-01"); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb)
	}
	if code := h.run(created.Path, append(taskTakeArgs(pid, start), "--json")...); code != 0 {
		t.Fatalf("lease take task: %d %s", code, h.errb)
	}
	var grant struct{ Gen, Fence int64 }
	if err := json.Unmarshal(h.out.Bytes(), &grant); err != nil {
		t.Fatal(err)
	}
	epoch := fmt.Sprintf("%d.%d", grant.Gen, grant.Fence)
	if code := h.run(created.Path, "job", "next", "--lease-epoch", epoch, "--json"); code != 0 || !strings.Contains(h.out.String(), `"id":"J-1"`) || !strings.Contains(h.out.String(), `"state":"assigned"`) {
		t.Fatalf("job next: %d %s %s", code, h.out, h.errb)
	}
	id, err := feed.TaskID(created.Path)
	if err != nil {
		t.Fatal(err)
	}
	records, err := (feed.Store{TaskDir: created.Path, TaskID: id}).Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records.Records) < 2 || records.Records[0].Type != "handover" {
		t.Fatalf("first record must be handover: %+v", records)
	}
}
