package contract

import "testing"

// A Blocked Job reads as Started with the reason "blocked", and a Job folded from
// blocked then exit carries that state.
func TestB2StateOfBlocked(t *testing.T) {
	if state, reason := StateOf(Blocked); state != StateStarted || reason != "blocked" {
		t.Fatalf("StateOf(Blocked) = %q, %q, want Started, blocked", state, reason)
	}
	posts := append(base(t), plBd2Loop(t, "started", 4, 1), plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "done"))
	j := Fold(posts).Jobs["J-1"]
	if j == nil || j.Status != Blocked {
		t.Fatalf("Job = %+v, want blocked", j)
	}
	if j.State != StateStarted || j.StateReason != "blocked" {
		t.Fatalf("Job state = %q %q, want Started blocked", j.State, j.StateReason)
	}
}
