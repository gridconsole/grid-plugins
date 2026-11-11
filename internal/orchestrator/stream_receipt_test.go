package orchestrator_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

const (
	receiptULID    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	receiptMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	receiptPlan    = "# Plan\n\nShip the parser.\n"
)

// receiptTask is a waiting task folder with plan.md and an old approval.json. ulid
// is its task.md id line, or "" for an old task with no Feed yet.
func receiptTask(t *testing.T, ulid string, patch map[string]any) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "projects", "p", "tasks", "T-01-task")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	md := "---\nid: T-01\n"
	if ulid != "" {
		md += "ulid: " + ulid + "\n"
	}
	md += "title: Example\nproject: example\nstate: waiting\ncreated: 2026-11-01T08:00:00Z\nupdated: 2026-11-01T08:00:00Z\n---\n"
	receipt := map[string]any{
		"task": "T-01", "who": map[string]any{"id": "dana", "role": "member"}, "when": "2026-11-01T09:00:00Z",
		"what": "approve", "from_state": "waiting", "to_state": "waiting", "note": "looks right",
		"lands_hash": "ab12", "plan_hash": plan.Hash(receiptPlan), "plan_digest": plan.Digest(receiptPlan),
	}
	for k, v := range patch {
		if v == nil {
			delete(receipt, k)
		} else {
			receipt[k] = v
		}
	}
	data, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"task.md": md, "intent.md": "Example", "plan.md": receiptPlan, "approval.json": string(data)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func receiptStream(dirs ...string) *orchestrator.Stream {
	return &orchestrator.Stream{
		Member: "worker", Machine: receiptMachine, Role: "cli", PID: os.Getpid(), PIDStart: "2026-01-02T03:04:05.000Z",
		Tasks: func() ([]string, error) { return dirs, nil },
		Out:   &bytes.Buffer{}, Diag: &bytes.Buffer{},
		ReceiptPending: migrate.ReceiptPending,
		ReceiptMove: func(dir string, w feed.Writer) error {
			_, _, err := migrate.ReceiptToJob(dir, w)
			return err
		},
	}
}

func gone(dir, name string) bool {
	_, err := os.Lstat(filepath.Join(dir, name))
	return errors.Is(err, os.ErrNotExist)
}

// A task that is only read (no approve, no open Job) still gets its person Job
// from the worker's pass: the receipt becomes the accepted result, the file is
// removed, and the lease is given back.
func TestStreamPassMovesTheReceiptOfATaskNobodyApproves(t *testing.T) {
	dir := receiptTask(t, receiptULID, nil)
	s := receiptStream(dir)
	for i := 0; i < 3; i++ {
		if err := s.Pass(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if !gone(dir, "approval.json") {
		t.Fatal("approval.json is still there")
	}
	a, err := task.ApprovalFromJob(dir)
	if err != nil || a.Job != "J-1" || a.PlanHash != plan.Hash(receiptPlan) || a.PlanDigest != plan.Digest(receiptPlan) || a.LandsHash != "ab12" || a.Note != "looks right" {
		t.Fatalf("approval %+v %v", a, err)
	}
	if !task.Approved(dir) {
		t.Fatal("the plan does not read as approved after the move")
	}
	tenure, err := (lease.TaskStore{Feed: feed.Store{TaskDir: dir, TaskID: receiptULID}}).Read("task")
	if err != nil || tenure.Holder != nil {
		t.Fatalf("lease not given back: %+v %v", tenure, err)
	}
	jobs, err := (feed.Store{TaskDir: dir, TaskID: receiptULID}).Read(feed.Filter{Type: "job"})
	if err != nil || len(jobs.Records) != 1 {
		t.Fatalf("a second pass must post nothing more: %+v %v", jobs, err)
	}
}

// A receipt that never approved a plan is not pending: the pass leaves the file
// and takes no lease for it, on every pass.
func TestStreamPassLeavesAReceiptThatNeverApprovedAPlan(t *testing.T) {
	dir := receiptTask(t, receiptULID, map[string]any{"plan_digest": nil})
	if migrate.ReceiptPending(dir) {
		t.Fatal("a receipt with no plan_digest is pending")
	}
	s := receiptStream(dir)
	for i := 0; i < 2; i++ {
		if err := s.Pass(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if gone(dir, "approval.json") {
		t.Fatal("approval.json was removed")
	}
	all, err := (feed.Store{TaskDir: dir, TaskID: receiptULID}).Read(feed.Filter{})
	if err != nil || len(all.Records) != 0 {
		t.Fatalf("the pass wrote to the Feed (a lease take posts a handover): %+v %v", all.Records, err)
	}
}

// An old task with no Feed identity and a plan receipt gets its identity from the
// pass, so the move has a Feed to write into.
func TestStreamPassGivesAnOldTaskItsFeedWhenItMovesTheReceipt(t *testing.T) {
	dir := receiptTask(t, "", nil)
	if _, err := feed.TaskID(dir); !errors.Is(err, feed.ErrNoTaskID) {
		t.Fatalf("fixture has an id: %v", err)
	}
	s := receiptStream(dir)
	for i := 0; i < 3; i++ {
		if err := s.Pass(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := feed.TaskID(dir); err != nil {
		t.Fatalf("no task id after the pass: %v", err)
	}
	if !gone(dir, "approval.json") || !task.Approved(dir) {
		t.Fatal("the receipt was not moved into an approved Job")
	}
}
