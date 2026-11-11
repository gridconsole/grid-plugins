package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// approveReceiptKeys are the keys of the receipt the app's real-plugin tests read
// from the task folder after grid approve --note (testdata/contract/plan-digest).
var approveReceiptKeys = []string{"from_state", "note", "plan_digest", "plan_hash", "task", "to_state", "what", "when", "who"}

func readApproveReceipt(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "approval.json"))
	if err != nil {
		t.Fatalf("approval.json: %v", err)
	}
	var receipt map[string]any
	if err := json.Unmarshal(data, &receipt); err != nil {
		t.Fatalf("approval.json is not one JSON object: %v: %s", err, data)
	}
	return receipt
}

// requireApproveReceipt fails unless the folder holds the approve receipt: the copy
// of the approval that grid approve writes beside the plan Job result.
func requireApproveReceipt(t *testing.T, dir string) {
	t.Helper()
	receipt := readApproveReceipt(t, dir)
	for _, key := range []string{"task", "who", "when", "from_state", "to_state", "plan_hash", "plan_digest"} {
		if _, ok := receipt[key]; !ok {
			t.Fatalf("approval.json lacks %q: %v", key, receipt)
		}
	}
	if receipt["what"] != "approve" {
		t.Fatalf("approval.json what = %v, want approve", receipt["what"])
	}
}

// grid approve --json prints the receipt as one JSON object with the nine keys, and
// the same nine keys are in approval.json with the same values.
func TestApproveReceiptFileHasTheNineKeysTheAppReads(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	h.leased(fixture.Path)
	if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
	if _, err := os.Stat(filepath.Join(fixture.Path, "approval.json")); err == nil {
		t.Fatal("approval.json exists before the first approve")
	}
	if code := h.run(h.home, "approve", "T-01", "--json", "--note", "looks right"); code != 0 {
		t.Fatalf("approve: exit %d: %s", code, h.errb)
	}
	var stdout map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &stdout); err != nil {
		t.Fatalf("approve --json is not one JSON object: %v: %s", err, h.out)
	}
	receipt := readApproveReceipt(t, fixture.Path)
	keys := make([]string, 0, len(receipt))
	for key := range receipt {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if !slices.Equal(keys, approveReceiptKeys) {
		t.Fatalf("receipt keys = %v, want %v", keys, approveReceiptKeys)
	}
	for key, value := range receipt {
		if !reflect.DeepEqual(stdout[key], value) {
			t.Errorf("approve --json %s = %v, approval.json = %v", key, stdout[key], value)
		}
	}
	if receipt["plan_hash"] != plan.Hash(approvedPlan) || receipt["plan_digest"] != plan.Digest(approvedPlan) || receipt["note"] != "looks right" {
		t.Fatalf("receipt does not carry the plan and the note: %v", receipt)
	}
	// The Job result stays the approval the plugin's readers use.
	if a, err := task.ApprovalFromJob(fixture.Path); err != nil || a.PlanDigest != plan.Digest(approvedPlan) {
		t.Fatalf("Job result %+v: %v", a, err)
	}
}
