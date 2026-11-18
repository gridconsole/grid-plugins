package orchestrator

import (
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// uwRow is the row of a Job in a pass's output.
func uwRow(t *testing.T, out Output, id string) Row {
	t.Helper()
	for _, r := range out.Rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no row for %s in %+v", id, out.Rows)
	return Row{}
}

// A Job the worker ended on the usage limit shows a reason the person can read, with the reset
// time of the exit code in their own clock, and a Job queued behind it shows the same reason
// and is not started.
func TestUsageLimitRowsSayWhenItResets(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	reset := f.now.Add(3 * time.Hour).Truncate(time.Minute)
	yaUrExit(t, f, reset.Unix())
	f.job(t, "J-2", nil)
	out := f.tick(t)
	want := "Waiting for the usage limit, resets " + reset.Local().Format("15:04")
	if got := uwRow(t, out, "J-1"); got.State != contract.Parked || got.Why != want {
		t.Fatalf("parked row %+v, want why %q", got, want)
	}
	if got := uwRow(t, out, "J-2"); got.Why != want {
		t.Fatalf("queued row %+v, want why %q", got, want)
	}
	if out.Assigned != 0 {
		t.Fatal("a Job started while the limit holds")
	}
}

// With no reset time in the exit the reason still reads plainly.
func TestUsageLimitRowWithoutResetTime(t *testing.T) {
	f := yaUrParked(t, nil)
	f.s.UsageReady = func(string) bool { return false }
	out := f.tick(t)
	if got := uwRow(t, out, "J-1").Why; got != "Waiting for the usage limit" {
		t.Fatalf("why %q", got)
	}
}

// Room on another account offers the Job again at once, but when that attempt ends on the same
// reset time the account the worker picked had no room: only the reset time lifts the hold, so no
// agent starts before it.
func TestUsageLimitRepeatWaitsForTheReset(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	reset := f.now.Add(3 * time.Hour).Unix()
	yaUrExit(t, f, reset)
	f.s.UsageReady = func(string) bool { return true }
	f.s.AccountRoom = func() (bool, bool) { return true, true }
	f.tick(t) // parks the Job on the limit
	if f.tick(t).Assigned != 1 {
		t.Fatal("room on another account did not offer the Job again")
	}
	f.ack(t)
	yaUrExit(t, f, reset)
	f.tick(t) // parks the Job on the limit again
	for range 3 {
		out := f.tick(t)
		if out.Assigned != 0 || f.state(t).Status != contract.Parked {
			t.Fatalf("started again before the reset: %+v", f.state(t))
		}
	}
	f.now = f.now.Add(3*time.Hour + time.Minute)
	if f.tick(t).Assigned != 1 {
		t.Fatal("not offered again after the reset")
	}
}
