package task

import (
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// approvalSnapshot holds everything an approval may write: the task header, the
// history and the newest result of the plan Job. Nothing writes a receipt file.
func approvalSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range []string{"task.md", "history.md"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out[name] = string(data)
	}
	found, _ := planJobResult(dir)
	out["result"] = found.Record
	noReceiptFile(t, dir)
	return out
}

// A repeat approve whose plan Job result already matches the plan returns that
// result and writes nothing: no second result and no second history line.
func TestApproveRepeatWritesNothing(t *testing.T) {
	for _, at := range []string{"plan"} {
		t.Run(at, func(t *testing.T) {
			s, task := waitingFixture(t)
			h := setPlan(t, task, planV1)
			first, err := approve(t, s, task, WriteOptions{ID: task.ID, Note: "Looks right"}, "")
			if err != nil {
				t.Fatal(err)
			}
			before := approvalSnapshot(t, task.Path)
			if before["result"] == "" {
				t.Fatal("the plan Job has no result after the first approval")
			}
			for _, token := range []string{"", h, plan.Digest(planV1)} {
				again, err := s.Approve(WriteOptions{ID: task.ID, Note: "Pressed twice", Via: "grid-app"}, token)
				if err != nil {
					t.Fatalf("repeat with token %q: %v", token, err)
				}
				if after := approvalSnapshot(t, task.Path); !maps.Equal(after, before) {
					t.Fatalf("repeat with token %q wrote:\nhistory before:\n%s\nhistory after:\n%s", token, before["history.md"], after["history.md"])
				}
				if !sameApproval(again, first) {
					t.Fatalf("repeat with token %q returned %+v, want the stored %+v", token, again, first)
				}
			}
		})
	}
}

// A result bound to another deploy rule is not a repeat: approving again rebinds it.
func TestApproveRepeatWithNewLandsHashRewrites(t *testing.T) {
	s, task := waitingFixture(t)
	h := setPlan(t, task, planV1)
	if _, err := approve(t, s, task, WriteOptions{ID: task.ID, LandsHash: "rule-a"}, ""); err != nil {
		t.Fatal(err)
	}
	event, err := approve(t, s, task, WriteOptions{ID: task.ID, LandsHash: "rule-b"}, h)
	if err != nil || event.LandsHash != "rule-b" {
		t.Fatalf("%+v: %v", event, err)
	}
	found, err := planJobResult(task.Path)
	if err != nil || found.LandsHash != "rule-b" {
		t.Fatalf("result not rebound: %+v %v", found, err)
	}
}
