package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setGate writes an old `gate:` line into task.md, as a task made before the
// fixed gates went away has one.
func setGate(t *testing.T, task Task, gate string) {
	t.Helper()
	path := filepath.Join(task.Path, "task.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.Replace(string(data), "state: waiting", "state: waiting\ngate: "+gate, 1))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// An old gate: line in task.md is ignored (S6c3, Q1, Q10): Show reads the task
// without an error, the task has no gate in its JSON, and approving it approves
// the plan with the current hash as the plan Job's result, never in a ship or preview receipt.
func TestOldGateLineIsIgnored(t *testing.T) {
	for _, gate := range []string{"ship", "preview"} {
		s, task := waitingFixture(t)
		h := setPlan(t, task, planV1)
		setGate(t, task, gate)
		shown, err := s.Show("", task.ID)
		if err != nil {
			t.Fatalf("Show with gate: %s: %v", gate, err)
		}
		data, err := json.Marshal(shown)
		if err != nil || strings.Contains(string(data), `"gate"`) {
			t.Fatalf("task JSON has a gate (%v): %s", err, data)
		}
		event, err := approve(t, s, task, WriteOptions{ID: task.ID}, h)
		if err != nil || event.Gate != "" || event.PlanHash != h {
			t.Fatalf("approve at old gate %s: %+v %v", gate, event, err)
		}
		for _, name := range []string{"approval-ship.json", "approval-preview.json"} {
			if _, err := os.Stat(filepath.Join(task.Path, name)); err == nil {
				t.Fatalf("%s written", name)
			}
		}
		if !Approved(task.Path) {
			t.Fatalf("the plan is not approved after approving a task with an old gate: %s", gate)
		}
	}
}
