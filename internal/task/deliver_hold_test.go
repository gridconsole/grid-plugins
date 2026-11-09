package task

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
)

// The refusal is identifiable by the caller, and a dry run is refused too.
func TestDeliverNamesAHeldPlanWithAnError(t *testing.T) {
	for _, dry := range []bool{false, true} {
		s, task := deliverFixture(t, "")
		if err := os.Remove(filepath.Join(task.Path, approvalFile)); err != nil {
			t.Fatal(err)
		}
		_, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid", DryRun: dry})
		if !errors.Is(err, ErrPlanNotApproved) || !strings.Contains(err.Error(), task.ID) {
			t.Fatalf("dry=%v err=%v", dry, err)
		}
	}
}

// Writing plan.md again after the approval holds the task again: the receipt
// binds the plan the person saw.
func TestDeliverRefusesAPlanChangedAfterApproval(t *testing.T) {
	s, task := deliverFixture(t, "")
	write(t, filepath.Join(task.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n\nA risk the person has not seen.\n")
	if _, err := (proof.Store{TaskDir: task.Path}).Add(proof.Input{Item: "1", Text: "ok again", Kind: "check", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"}); !errors.Is(err, ErrPlanNotApproved) {
		t.Fatalf("err=%v", err)
	}
}
