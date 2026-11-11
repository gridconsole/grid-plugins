package migrate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

const (
	receiptTaskID  = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	receiptMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	receiptPlan    = "# Plan\n\nShip the parser.\n"
)

// receiptFolder is a waiting task folder with plan.md, a task id, and the task
// lease held by the writer it returns. approval is the receipt text, or "".
func receiptFolder(t *testing.T, approval string) (string, feed.Writer) {
	t.Helper()
	dir := t.TempDir()
	md := "---\nid: T-01\nulid: " + receiptTaskID + "\ntitle: Example\nproject: example\nstate: waiting\ncreated: 2026-11-01T08:00:00Z\nupdated: 2026-11-01T08:00:00Z\n---\n"
	for name, text := range map[string]string{"task.md": md, "plan.md": receiptPlan, "approval.json": approval} {
		if name == "approval.json" && approval == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	w := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: "dana@" + receiptMachine, Lease: "task"}
	tenure, err := (lease.TaskStore{Feed: feed.Store{TaskDir: dir, TaskID: receiptTaskID}}).TakeTask(w, os.Getpid(), "2026-01-02T03:04:05.000Z")
	if err != nil {
		t.Fatal(err)
	}
	w.Gen, w.Fence = tenure.Gen, tenure.Fence
	// The first record of a tenure is its handover; a tick posts it in production.
	if _, err := (feed.Store{TaskDir: dir, TaskID: receiptTaskID}).Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	return dir, w
}

func receiptJSON(t *testing.T, patch map[string]any) string {
	t.Helper()
	m := map[string]any{
		"task": "T-01", "who": map[string]any{"id": "dana", "role": "member"}, "when": "2026-11-01T09:00:00Z",
		"what": "approve", "from_state": "waiting", "to_state": "waiting", "note": "looks right",
		"lands_hash": "ab12", "plan_hash": plan.Hash(receiptPlan), "plan_digest": plan.Digest(receiptPlan),
	}
	for k, v := range patch {
		if v == nil {
			delete(m, k)
		} else {
			m[k] = v
		}
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func gone(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return errors.Is(err, os.ErrNotExist)
}

// An old approval.json becomes the result of a person Job of kind approve, with
// the plan revision it named; the file is gone and a second run finds nothing.
func TestReceiptToJobMovesTheReceiptIntoTheApproveJobResult(t *testing.T) {
	dir, w := receiptFolder(t, "")
	if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte(receiptJSON(t, nil)), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome, job, err := ReceiptToJob(dir, w)
	if err != nil || outcome != ReceiptMoved || job != "J-1" {
		t.Fatalf("outcome %q job %q err %v", outcome, job, err)
	}
	if gone(t, filepath.Join(dir, "approval.json")) {
		t.Fatal("approval.json was removed")
	}
	a, err := taskstore.ApprovalFromJob(dir)
	if err != nil {
		t.Fatalf("the moved receipt does not approve the plan: %v", err)
	}
	if a.Job != "J-1" || a.PlanHash != plan.Hash(receiptPlan) || a.PlanDigest != plan.Digest(receiptPlan) || a.LandsHash != "ab12" || a.Note != "looks right" || a.Answer != "approved" {
		t.Fatalf("result %+v lost a receipt field", a)
	}
	snap, err := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: receiptTaskID})
	if err != nil || len(snap.Items) != 1 || !snap.Items[0].IsPerson() || snap.Items[0].Kind != "approve" {
		t.Fatalf("items %+v err %v: want one person Job of kind approve", snap.Items, err)
	}
	if outcome, _, err := ReceiptToJob(dir, w); err != nil || outcome != ReceiptCurrent {
		t.Fatalf("second run: %q %v; the Job is the record and the file stays", outcome, err)
	}
	if snap, _ := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: receiptTaskID}); len(snap.Items) != 1 {
		t.Fatalf("second run posted another Job: %+v", snap.Items)
	}
}

// The moved result names the revision the person saw: a plan that changed since
// is still refused, as it was with the file.
func TestReceiptToJobKeepsTheStalePlanRefused(t *testing.T) {
	dir, w := receiptFolder(t, "")
	if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte(receiptJSON(t, map[string]any{"plan_hash": "000000", "plan_digest": strings.Repeat("0", 64)})), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome, _, err := ReceiptToJob(dir, w); err != nil || outcome != ReceiptMoved {
		t.Fatalf("outcome %q err %v", outcome, err)
	}
	if _, err := taskstore.ApprovalFromJob(dir); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
		t.Fatalf("a receipt for another revision approved the plan: %v", err)
	}
}

// A task that already has an approve person Job keeps the Job as its record: the
// file is retired, nothing is posted, and an open Job stays open.
func TestReceiptToJobRetiresTheFileBesideAnExistingJob(t *testing.T) {
	dir, w := receiptFolder(t, "")
	store := feed.Store{TaskDir: dir, TaskID: receiptTaskID}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "job", Job: "J-1", Body: PlanJobBody("J-1", "dana")}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte(receiptJSON(t, nil)), 0o600); err != nil {
		t.Fatal(err)
	}
	if outcome, _, err := ReceiptToJob(dir, w); err != nil || outcome != ReceiptCurrent {
		t.Fatalf("outcome %q err %v", outcome, err)
	}
	if gone(t, filepath.Join(dir, "approval.json")) {
		t.Fatal("approval.json was removed")
	}
	if _, err := taskstore.ApprovalFromJob(dir); err == nil || !strings.Contains(err.Error(), "no result yet") {
		t.Fatalf("the open Job was answered from the file: %v", err)
	}
}

// A receipt that never approved a plan (no full digest, or another gate) is left
// as it is and posts nothing.
func TestReceiptToJobLeavesAReceiptThatNeverApprovedAPlan(t *testing.T) {
	for name, patch := range map[string]map[string]any{
		"no digest": {"plan_digest": nil},
		"a gate":    {"gate": "ship"},
		"reopen":    {"what": "reopen"},
	} {
		t.Run(name, func(t *testing.T) {
			dir, w := receiptFolder(t, "")
			if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte(receiptJSON(t, patch)), 0o600); err != nil {
				t.Fatal(err)
			}
			outcome, why, err := ReceiptToJob(dir, w)
			if err != nil || outcome != ReceiptKept || why == "" {
				t.Fatalf("outcome %q why %q err %v", outcome, why, err)
			}
			if gone(t, filepath.Join(dir, "approval.json")) {
				t.Fatal("the file was removed")
			}
			if snap, err := orchestrator.Load(feed.Store{TaskDir: dir, TaskID: receiptTaskID}); err != nil || len(snap.Items) != 0 {
				t.Fatalf("posted %+v err %v", snap.Items, err)
			}
		})
	}
}

// A folder with no approval.json is not touched.
func TestReceiptToJobWithNoFileDoesNothing(t *testing.T) {
	dir, w := receiptFolder(t, "")
	if outcome, _, err := ReceiptToJob(dir, w); err != nil || outcome != ReceiptNone {
		t.Fatalf("outcome %q err %v", outcome, err)
	}
}
