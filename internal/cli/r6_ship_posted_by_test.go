package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// r6Steps decodes task.steps of a workflow show --json answer as maps, so a
// key the answer carries is seen whatever Go struct wrote it.
func r6Steps(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var doc struct {
		Task struct {
			Steps []map[string]any `json:"steps"`
		} `json:"task"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("not one JSON object: %v\n%s", err, raw)
	}
	return doc.Task.Steps
}

// The built-in flow's Ship step says deliver posts its person Job
// (posted_by: deliver), so the orchestrator posts no Job for it; no other step
// carries the field. The captured fixture holds the same answer.
func TestR6BuiltInShipPostedByDeliver(t *testing.T) {
	w := nhWfSetup(t)
	w.h.write(filepath.Join(w.project, "tasks", "T-1-show", "task.md"), nhWfTaskText(""))
	raw := w.out(t, w.checkout, "workflow", "show", "--task", "T-1", "--json")

	fixture, err := os.ReadFile(yiWfFixturePath("cmd-workflow-show", "valid", "task-built-in"))
	if err != nil {
		t.Fatal(err)
	}
	for label, answer := range map[string][]byte{"answer": raw, "fixture task-built-in": fixture} {
		steps := r6Steps(t, answer)
		if len(steps) != 3 {
			t.Fatalf("%s: steps %v, want Run, Check, Ship", label, steps)
		}
		for i, s := range steps {
			got, has := s["posted_by"]
			if i == 2 {
				if s["name"] != "Ship" || s["kind"] != "approve" || got != "deliver" {
					t.Errorf("%s: step %v, want Ship (approve) with posted_by deliver", label, s)
				}
				continue
			}
			if has {
				t.Errorf("%s: step %v carries posted_by; only the Ship step deliver posts does", label, s)
			}
		}
	}
}
