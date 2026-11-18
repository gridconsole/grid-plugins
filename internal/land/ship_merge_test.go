package land_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// A ship answer on a project with no deploy topic still lands the delivered
// branch: the merge runs once, its commit is in history and the task closes.
func TestShipAnswerNoDeployTopicMergesTheBranch(t *testing.T) {
	s, dir, _ := r7Fixture(t, nil)
	var merged []string
	s.Merge = func(id string) (string, error) {
		merged = append(merged, id)
		return "0123456789abcdef0123456789abcdef01234567", nil
	}
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || !ran {
		t.Fatalf("AfterShip = ran %t, %v; want ran and no error", ran, err)
	}
	if len(merged) != 1 || merged[0] != "T-01" {
		t.Fatalf("merge ran for %v, want once for T-01", merged)
	}
	history := r7History(t, dir)
	if !strings.Contains(history, "merged the task's branch as 0123456789ab") || !strings.Contains(history, "approved after J-6; no deploy topic; merged as 0123456789ab") {
		t.Fatalf("history does not name the merge:\n%s", history)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the merge, want closed", rec.State, err)
	}
	// A second call for the same Job merges nothing more.
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || ran || len(merged) != 1 {
		t.Fatalf("second AfterShip = ran %t, %v, %d merges; want not ran and one merge", ran, err, len(merged))
	}
}

// A merge that fails leaves the task waiting and returns a FailedError with the
// reason, so the caller can put the ship Job back with its person; the next
// answer merges again.
func TestShipAnswerNoDeployTopicMergeFailureKeepsTheTaskWaiting(t *testing.T) {
	s, dir, _ := r7Fixture(t, nil)
	merges := 0
	s.Merge = func(string) (string, error) {
		merges++
		if merges == 1 {
			return "", errors.New("merging lane into main conflicts in a.txt")
		}
		return "fedcba9876543210fedcba9876543210fedcba98", nil
	}
	_, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6")
	var failed *land.FailedError
	if !ran || !errors.As(err, &failed) || !strings.Contains(failed.Text, "conflicts in a.txt") {
		t.Fatalf("AfterShip = ran %t, %v; want a FailedError naming the conflict", ran, err)
	}
	history := r7History(t, dir)
	if !strings.Contains(history, " · land failed · merging lane into main conflicts in a.txt · ") || strings.Contains(history, "nothing to land") {
		t.Fatalf("history does not record the failed merge:\n%s", history)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after a failed merge, want waiting", rec.State, err)
	}
	if err := s.Reopened("", "T-01", "J-6"); err != nil {
		t.Fatal(err)
	}
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || !ran || merges != 2 {
		t.Fatalf("AfterShip after the reopen = ran %t, %v, %d merges; want a second merge", ran, err, merges)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the second merge, want closed", rec.State, err)
	}
}

// A merge that finds no branch to land (the task has none) closes the task on the
// answer as it did before the merge existed.
func TestShipAnswerNoDeployTopicNoBranchClosesWithoutMerge(t *testing.T) {
	s, dir, _ := r7Fixture(t, nil)
	s.Merge = func(string) (string, error) { return "", nil }
	if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || !ran {
		t.Fatalf("AfterShip = ran %t, %v; want ran and no error", ran, err)
	}
	history := r7History(t, dir)
	if !strings.Contains(history, "J-6 approved; no deploy topic for project sample, nothing to land") || strings.Contains(history, "merged") {
		t.Fatalf("history for a task with no branch:\n%s", history)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v, want closed", rec.State, err)
	}
}
