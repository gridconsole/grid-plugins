package task

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// Store.Approve writes the history line and nothing else: no receipt file and no
// Job. The approval is posted by its caller as the plan Job's result.
func TestApproveWritesOnlyTheHistoryLine(t *testing.T) {
	s, task := waitingFixture(t)
	h := setPlan(t, task, planV1)
	event, err := s.Approve(WriteOptions{ID: task.ID, Note: "Reviewed"}, h)
	if err != nil || event.PlanHash != h || event.PlanDigest != plan.Digest(planV1) || event.What != "approve" {
		t.Fatalf("%+v: %v", event, err)
	}
	noReceiptFile(t, task.Path)
	lines := historyLines(t, task.Path)
	if len(lines) == 0 || !strings.Contains(lines[len(lines)-1], `"what":"approve"`) || !strings.Contains(lines[len(lines)-1], `"plan_hash":"`+h+`"`) {
		t.Fatalf("history: %v", lines)
	}
	if HasPlanJob(task.Path) || Approved(task.Path) {
		t.Fatal("Approve alone posted a Job or approved the plan")
	}
}

// The stale-hash refusal and the compare-and-set read the plan Job's result, not a
// file: a receipt file left in the folder is not an approval to replace.
func TestApproveCompareAndSetReadsTheJobResult(t *testing.T) {
	s, task := waitingFixture(t)
	h1 := setPlan(t, task, planV1)
	// A file shaped like an old receipt for another plan is not read.
	write(t, filepath.Join(task.Path, "approval.json"), `{"task":"`+task.ID+`","what":"approve","plan_hash":"abcdef","plan_digest":"`+strings.Repeat("a", 64)+`"}`)
	if _, err := approve(t, s, task, WriteOptions{ID: task.ID}, ""); err != nil {
		t.Fatalf("a receipt file blocked the first approval: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(task.Path, "approval.json")); strings.Contains(string(b), "abcdef") || !strings.Contains(string(b), `"what":"approve"`) {
		t.Fatalf("Approve did not rewrite the file with its own receipt: %s", b)
	}
	// The plan changes: the Job result of planV1 is now the stale approval.
	h2 := setPlan(t, task, planV2)
	before := historyLines(t, task.Path)
	if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err == nil || !strings.HasPrefix(err.Error(), "Changed since you approved") {
		t.Fatalf("implicit reapproval of a changed plan: %v", err)
	}
	if _, err := s.Approve(WriteOptions{ID: task.ID}, h1); err == nil || !strings.HasPrefix(err.Error(), "plan hash no longer matches") {
		t.Fatalf("stale hash: %v", err)
	}
	if after := historyLines(t, task.Path); len(after) != len(before) {
		t.Fatalf("a refusal wrote history: %v", after)
	}
	if _, err := approve(t, s, task, WriteOptions{ID: task.ID}, h2); err != nil {
		t.Fatalf("explicit reapproval: %v", err)
	}
	if found, err := planJobResult(task.Path); err != nil || found.PlanHash != h2 || found.PlanDigest != plan.Digest(planV2) {
		t.Fatalf("%+v: %v", found, err)
	}
}

// Reopening a task writes neither a receipt nor over one that is left in the folder.
func TestReopenWritesNoReceiptFile(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	old := `{"task":"` + task.ID + `","what":"approve","plan_hash":"abcdef"}`
	write(t, filepath.Join(task.Path, "approval.json"), old)
	for _, action := range []string{"close", "reopen"} {
		if _, err := s.Change(WriteOptions{ID: task.ID, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(task.Path, "approval.json")); !strings.Contains(string(b), `"what":"reopen"`) {
		t.Fatalf("reopen did not overwrite the file with its event: %s", b)
	}
	// With no receipt file in the folder none appears either.
	s2, task2 := waitingFixture(t)
	for _, action := range []string{"close", "reopen"} {
		if _, err := s2.Change(WriteOptions{ID: task2.ID, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	noReceiptFile(t, task2.Path)
}

// PlanHeldWith asks whether the task has an approve person Job: with one, only
// its result matching plan.md releases the hold (an old prose acceptance does not);
// with none, the history line still does.
func TestPlanHeldAsksForThePlanJob(t *testing.T) {
	_, task := waitingFixture(t)
	setPlan(t, task, planV1)
	text, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	accepted := taskfile.HistoryLine("2026-01-02T03:04", "plan accepted", "000000", "person") + "\n"
	held := func(history string) bool { return PlanHeld(task.Path, string(text), history) }
	if HasPlanJob(task.Path) {
		t.Fatal("the fixture already has a plan Job")
	}
	if !held("") || held(accepted) {
		t.Fatalf("no plan Job: held %t, with the history line %t; want true, false", held(""), held(accepted))
	}
	job := openJob(t, task.Path)
	if !HasPlanJob(task.Path) || !held("") || !held(accepted) {
		t.Fatalf("a plan Job with no result: has %t, held %t, with the history line %t; want true, true, true", HasPlanJob(task.Path), held(""), held(accepted))
	}
	postResult(t, task.Path, job, map[string]any{"plan_hash": plan.Hash(planV1), "plan_digest": plan.Digest(planV1)})
	if held("") || held(accepted) {
		t.Fatal("a matching result did not release the hold")
	}
	setPlan(t, task, planV2)
	if !held("") || !held(accepted) {
		t.Fatal("a changed plan stayed released")
	}
}

// A task with no approve person Job is not approved, whatever else is in its
// folder. It replaces TestTaskWithoutPlanJobFallsBackToApprovalJSON, which tested
// the approval.json fallback that verdict 12 removed.
func TestTaskWithoutPlanJobIsNotApproved(t *testing.T) {
	s, task := waitingFixture(t)
	h := setPlan(t, task, planV1)
	write(t, filepath.Join(task.Path, "approval.json"), `{"task":"`+task.ID+`","what":"approve","plan_hash":"`+h+`","plan_digest":"`+plan.Digest(planV1)+`"}`)
	if _, err := s.Approval("", task.ID); !errors.Is(err, ErrNoPlanJob) {
		t.Fatalf("Approval: %v", err)
	}
	if Approved(task.Path) || ApprovedDigest(task.Path, h, plan.Digest(planV1)) {
		t.Fatal("a task with no plan Job is approved")
	}
	// Approving it gives the same task an approval only through the Job.
	event, err := approve(t, s, task, WriteOptions{ID: task.ID}, h)
	if err != nil || !Approved(task.Path) {
		t.Fatalf("%+v: %v", event, err)
	}
}

// A journal an older plugin left with a receipt file replays its history and
// task.md and drops the receipt.
func TestRecoverWriteDropsAnOldReceiptEntry(t *testing.T) {
	_, task := waitingFixture(t)
	history := "- {\"what\":\"note\"}\n"
	journal := map[string][]byte{"history.md": []byte(history), "approval.json": []byte("{}\n"), "approval-ship.json": []byte("{}\n")}
	data, err := json.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(task.Path, journalName), string(data))
	if err := recoverWrite(task.Path); err != nil {
		t.Fatal(err)
	}
	noReceiptFile(t, task.Path)
	if got := readFile(t, filepath.Join(task.Path, "history.md")); got != history {
		t.Fatalf("history.md = %q", got)
	}
	if _, err := os.Stat(filepath.Join(task.Path, journalName)); !os.IsNotExist(err) {
		t.Fatal("journal not removed")
	}
	// A name that is neither a task file nor an old receipt is still refused.
	bad, _ := json.Marshal(map[string][]byte{"history.md": []byte(history), "other.json": []byte("{}")})
	write(t, filepath.Join(task.Path, journalName), string(bad))
	if err := recoverWrite(task.Path); err == nil || !strings.Contains(err.Error(), "invalid journal file") {
		t.Fatalf("unknown journal file: %v", err)
	}
}
