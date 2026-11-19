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
	layout := "15:04"
	if rl, nl := reset.Local(), f.now.Local(); rl.YearDay() != nl.YearDay() || rl.Year() != nl.Year() {
		layout = "Mon 15:04" // a reset on another day names the weekday
	}
	want := "Waiting for the usage limit, resets " + reset.Local().Format(layout)
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

// A usage_limit exit with no reset time has nothing to wait for, so it backs off as a crash does
// (ten seconds doubling, from the Job's usage_limit exits in a row) and is not offered again on
// every pass. It is no crash: however many there are, the cap never parks the Job on the person.
func TestUsageLimitWithoutResetTimeBacksOff(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.s.UsageReady = func(string) bool { return true }
	f.s.RetryDelay = CrashBackoff
	for i, wait := range []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second, 80 * time.Second} {
		yaUrExit(t, f, nil)
		out := f.tick(t) // parks the Job on the limit
		want := "Waiting for the usage limit, next try " + f.now.Add(wait).Local().Format("15:04:05")
		if got := uwRow(t, out, "J-1"); got.State != contract.Parked || got.Why != want {
			t.Fatalf("exit %d: row %+v, want parked with why %q", i+1, got, want)
		}
		f.now = f.now.Add(wait - time.Second)
		if out := f.tick(t); out.Assigned != 0 || f.state(t).Status != contract.Parked {
			t.Fatalf("exit %d: offered again before its %s back-off: %+v", i+1, wait, f.state(t))
		}
		f.now = f.now.Add(time.Second)
		if f.tick(t).Assigned != 1 {
			t.Fatalf("exit %d: not offered again after its %s back-off", i+1, wait)
		}
		f.ack(t)
	}
	yaUrExit(t, f, nil)
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "usage_limit" {
		t.Fatalf("five usage_limit exits in a row left the Job %s on %q, want parked on usage_limit", j.Status, j.Reason)
	}
}
