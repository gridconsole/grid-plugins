package task

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func repeatApproveLines(lines []string) int {
	n := 0
	for _, l := range lines {
		if strings.Contains(l, `"what":"approve"`) {
			n++
		}
	}
	return n
}

// A person with no lease answers the plan Job, and the answer waits for the
// holder's next pass, so planJobResult says the Job has no result yet (ErrPlanJobOpen).
// A repeat approve of the same plan must then find the approval that grid approve
// wrote beside the plan (approval.json) and write nothing. TestApproveRepeatWritesNothing
// covers the repeat when the Job already holds the accepting result.
func TestRepeatApproveWithTheJobAnswerPendingWritesNothing(t *testing.T) {
	s, task := waitingFixture(t)
	h := setPlan(t, task, planV1)
	approvefix.OpenJob(t, task.Path)
	// task.md as the first approve finds it: waiting, with the ulid line OpenJob wrote.
	taskMD, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	o := WriteOptions{ID: task.ID, Note: "Reviewed"}
	first, err := s.Approve(o, h)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	history := historyLines(t, task.Path)
	if err := os.WriteFile(filepath.Join(task.Path, "task.md"), taskMD, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{h, plan.Digest(planV1), ""} {
		second, err := s.Approve(o, token)
		if err != nil {
			t.Fatalf("repeat approve with token %q: %v", token, err)
		}
		again, err := os.ReadFile(filepath.Join(task.Path, "approval.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(receipt, again) {
			t.Fatalf("repeat approve rewrote approval.json:\nfirst:  %s\nrepeat: %s", receipt, again)
		}
		after := historyLines(t, task.Path)
		if len(after) != len(history) || repeatApproveLines(after) != 1 {
			t.Fatalf("repeat approve with token %q changed history: %d approve lines, %d lines before and %d after", token, repeatApproveLines(after), len(history), len(after))
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("repeat approve returned another receipt:\nfirst:  %+v\nrepeat: %+v", first, second)
		}
	}
}

// A repeat bound to another deploy rule, or after the plan changed, is not a repeat.
func TestRepeatApproveWithTheJobAnswerPendingStillApprovesAChange(t *testing.T) {
	s, task := waitingFixture(t)
	h1 := setPlan(t, task, planV1)
	approvefix.OpenJob(t, task.Path)
	if _, err := s.Approve(WriteOptions{ID: task.ID, LandsHash: "rule-a"}, h1); err != nil {
		t.Fatal(err)
	}
	if event, err := s.Approve(WriteOptions{ID: task.ID, LandsHash: "rule-b"}, h1); err != nil || event.LandsHash != "rule-b" {
		t.Fatalf("another lands hash: %+v: %v", event, err)
	}
	h2 := setPlan(t, task, planV2)
	second, err := s.Approve(WriteOptions{ID: task.ID, LandsHash: "rule-b"}, h2)
	if err != nil {
		t.Fatalf("approve of the changed plan: %v", err)
	}
	if second.PlanHash != h2 || second.PlanDigest != plan.Digest(planV2) {
		t.Fatalf("the changed plan was not approved: %+v", second)
	}
	receipt, err := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	if err != nil || !strings.Contains(string(receipt), plan.Digest(planV2)) {
		t.Fatalf("approval.json does not name the changed plan: %s: %v", receipt, err)
	}
	if n := repeatApproveLines(historyLines(t, task.Path)); n != 3 {
		t.Fatalf("approve lines = %d, want 3", n)
	}
}

// A receipt that a reopen overwrote is not an approval: a press after a reopen
// approves again.
func TestRepeatApproveAfterReopenWithTheJobAnswerPendingApprovesAgain(t *testing.T) {
	s, task := waitingFixture(t)
	h := setPlan(t, task, planV1)
	approvefix.OpenJob(t, task.Path)
	if _, err := s.Approve(WriteOptions{ID: task.ID}, h); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"close", "reopen"} {
		if _, err := s.Change(WriteOptions{ID: task.ID, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(task.Path, "approval.json")); !strings.Contains(string(b), `"what":"reopen"`) {
		t.Fatalf("reopen did not overwrite the receipt: %s", b)
	}
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(WriteOptions{ID: task.ID}, h); err != nil {
		t.Fatalf("approve after reopen: %v", err)
	}
	if n := repeatApproveLines(historyLines(t, task.Path)); n != 2 {
		t.Fatalf("approve lines = %d, want 2", n)
	}
}
