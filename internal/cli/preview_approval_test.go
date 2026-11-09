package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// At a task's preview gate grid approve records a preview approval and its
// history line, and leaves the plan approval alone.
func TestApproveAtPreviewGateRecordsPreviewApproval(t *testing.T) {
	h, dir := deliveryFixture(t)
	if code := h.run(dir, "task", "status", "T-01", "waiting"); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(dir, "approve", "T-01", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	planBytes, err := os.ReadFile(filepath.Join(dir, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	taskMD := filepath.Join(dir, "task.md")
	data, err := os.ReadFile(taskMD)
	if err != nil {
		t.Fatal(err)
	}
	h.write(taskMD, strings.Replace(string(data), "state: waiting", "state: waiting\ngate: preview", 1))
	h.out.Reset()
	if code := h.run(dir, "approve", "T-01", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	var event task.Event
	if err := json.Unmarshal(h.out.Bytes(), &event); err != nil || event.What != "approve" || event.Gate != "preview" || event.PlanHash == "" {
		t.Fatalf("%+v %v", event, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "approval-preview.json")); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(dir, "approval.json")); err != nil || string(got) != string(planBytes) {
		t.Fatal("preview approval changed the plan receipt", err)
	}
	history, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil || !strings.Contains(string(history), " · preview approved · "+event.PlanHash+" · by ") {
		t.Fatalf("no preview approved line: %v\n%s", err, history)
	}
	if !task.Approved(dir) {
		t.Fatal("the plan approval stopped counting")
	}
}

// A preview receipt that matches the plan counts as approved work to migrate
// around, like the plan and ship receipts.
func TestPlanMigrateRefusesMatchingPreviewApproval(t *testing.T) {
	h := newHarness(t)
	dir := planTask(h)
	old := planFixture(t, "legacy/form-b.md") + "\nRecall: nothing found\n"
	h.write(filepath.Join(dir, "plan.md"), old)
	h.write(filepath.Join(dir, "approval-preview.json"), "{\"plan_hash\":\""+plan.Hash(old)+"\"}\n")
	if code := h.run(dir, "plan", "migrate"); code != 1 || !strings.Contains(h.errb.String(), "need approving again") {
		t.Fatalf("%d: %s", code, h.errb)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "plan.md")); err != nil || string(data) != old {
		t.Fatal("refusal changed plan")
	}
	h.errb.Reset()
	if code := h.run(dir, "plan", "migrate", "--yes"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
}
