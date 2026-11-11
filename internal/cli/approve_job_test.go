package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// approveJobHarness is a waiting task with plan.md and an open person Job J-2
// of an approve step.
func approveJobHarness(t *testing.T) (*harness, string, feed.Store, string) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	text := "# Plan\n\nShip the parser.\n"
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "intent.md"), []byte("# Intent\n\nShip the parser.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	md := "---\nid: T-07\nulid: " + feedTaskID + "\ntitle: Example\nproject: example\nstate: waiting\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-2", nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = "approve"
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", string(out)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	return h, path, store, text
}

// approvalResult is the body of the control that accepted J-2, or nil.
func approvalResult(t *testing.T, store feed.Store) map[string]any {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if e.Body["target"] == "J-2" && e.Body["status"] == "accepted" {
			return e.Body
		}
	}
	return nil
}

// grid approve with the current hash answers the plan's person Job with the
// digest, and approval.json is still written.
func TestApprovePostsPersonJobResultWithPlanDigest(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	body := approvalResult(t, store)
	if body == nil || body["plan_hash"] != plan.Hash(text) || body["plan_digest"] != plan.Digest(text) || body["answer"] != "approved" {
		t.Fatalf("Job result body %v", body)
	}
	if st, _ := store.State(); string(st.Jobs["J-2"].Status) != "accepted" {
		t.Fatalf("J-2 is %s", st.Jobs["J-2"].Status)
	}
	// The Job result is the only approval record: no approval.json (verdict 12).
	if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("approval.json was written: %v", err)
	}
	if a, err := taskstore.ApprovalFromJob(path); err != nil || a.PlanHash != plan.Hash(text) || a.PlanDigest != plan.Digest(text) {
		t.Fatalf("%+v %v", a, err)
	}
}

// A stale hash is refused as today and writes neither the Job result nor
// approval.json, through grid approve and through grid job result.
func TestApproveStaleHashWritesNoJobResult(t *testing.T) {
	h, path, store, _ := approveJobHarness(t)
	for _, args := range [][]string{
		{"approve", path, "--plan-hash", "000000"},
		{"job", "result", "J-2", "--plan-hash", "000000", "--body", `{"answer":"yes"}`},
	} {
		h.errb.Reset()
		if code := h.run(path, args...); code != 1 || !strings.Contains(h.errb.String(), "plan hash no longer matches") {
			t.Fatalf("%v: %d %s", args, code, h.errb.String())
		}
		if body := approvalResult(t, store); body != nil {
			t.Fatalf("%v wrote a Job result %v", args, body)
		}
		if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%v wrote approval.json: %v", args, err)
		}
	}
}

// grid job result --plan-hash answers the person Job; ApprovalFromJob matches
// until plan.md changes, then reports the result as stale.
func TestJobResultPlanHashAndApprovalFromJob(t *testing.T) {
	h, path, _, text := approveJobHarness(t)
	if _, err := taskstore.ApprovalFromJob(path); err == nil {
		t.Fatal("approved before any result")
	}
	if code := h.run(path, "job", "result", "J-2", "--plan-hash", plan.Digest(text), "--body", `{"answer":"Go ahead.","note":"checked"}`); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	a, err := taskstore.ApprovalFromJob(path)
	if err != nil || a.Job != "J-2" || a.PlanHash != plan.Hash(text) || a.PlanDigest != plan.Digest(text) || a.Answer != "Go ahead." || a.Note != "checked" {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("approval.json was written: %v", err)
	}
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(text+"\nOne more step.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := taskstore.ApprovalFromJob(path); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
		t.Fatalf("after plan.md changed: %v", err)
	}
}

// D11: grid approve answers the plan's person Job only when the Jobs it waits for
// are accepted; it exits 3, names the waiting Job and writes neither the result
// nor approval.json.
func TestApproveWaitsForTheAfterJob(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	text := "# Plan\n\nShip the parser.\n"
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "intent.md"), []byte("# Intent\n\nShip the parser.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	md := "---\nid: T-07\nulid: " + feedTaskID + "\ntitle: Example\nproject: example\nstate: waiting\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-1", "--body", personBody(t, "J-1", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-2", []any{"J-1"})), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = "approve"
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", string(out)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	h.errb.Reset()
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 3 || !strings.Contains(h.errb.String(), "J-1") {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if s := string(st.Jobs["J-1"].Status); s != "posted" {
		t.Fatalf("J-1 is %s", s)
	}
	if s := string(st.Jobs["J-2"].Status); s != "posted" {
		t.Fatalf("J-2 is %s", s)
	}
	if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("approval.json: %v", err)
	}
}

// A Feed snapshot that cannot be loaded makes grid approve exit 3 with the
// "grid approve: " prefix its other errors use, and writes no approval.json.
func TestApproveLoadFailureExits3WithPrefix(t *testing.T) {
	h, path, _, text := approveJobHarness(t)
	old := loadFeedSnapshot
	t.Cleanup(func() { loadFeedSnapshot = old })
	loadFeedSnapshot = func(feed.Store) (orchestrator.Snapshot, error) {
		return orchestrator.Snapshot{}, errors.New("feed unreadable")
	}
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 3 {
		t.Fatalf("exit %d %s", code, h.errb.String())
	}
	if got := h.errb.String(); got != "grid approve: feed unreadable\n" {
		t.Fatalf("stderr %q", got)
	}
	if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("approval.json: %v", err)
	}
}
