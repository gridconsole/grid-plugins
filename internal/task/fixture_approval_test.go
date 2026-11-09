package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// approveFixturePlan leaves the receipt a person's approval of plan.md leaves, so
// a task in any state can be delivered. It writes the file alone: history stays
// as the test finds it.
func approveFixturePlan(t *testing.T, s Store, task Task) {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(task.Path, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	event := s.event(task, WriteOptions{Action: "approve"}, task.State)
	event.PlanHash, event.PlanDigest = plan.Hash(string(text)), plan.Digest(string(text))
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(task.Path, approvalFile), string(data)+"\n")
}
