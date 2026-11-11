package orchestrator

import (
	"context"
	"errors"
	"testing"
)

// TestFlakeNWWaitsOnceWhenAPassAssignsNothing keeps the zero in
// TestNextWaitReturnsAssignedJobBeforeTheTick from being vacuous: with nothing
// to assign, Next does wait, and it waits through the pause field exactly once.
func TestFlakeNWWaitsOnceWhenAPassAssignsNothing(t *testing.T) {
	f := setup(t)
	calls := 0
	f.s.pause = func(context.Context) error {
		calls++
		return context.Canceled
	}
	out, err := f.s.Next(context.Background(), true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, out = %+v", err, out)
	}
	if out.Assigned != 0 {
		t.Fatalf("Assigned = %d, want 0", out.Assigned)
	}
	if calls != 1 {
		t.Fatalf("pause called %d times, want 1", calls)
	}
}
