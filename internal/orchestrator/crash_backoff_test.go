package orchestrator

import (
	"strings"
	"testing"
	"time"
)

func TestCrashBackoffSchedule(t *testing.T) {
	for n, want := range map[int]time.Duration{
		0: 10 * time.Second, 1: 10 * time.Second, 2: 20 * time.Second, 3: 40 * time.Second,
		4: 80 * time.Second, 5: 160 * time.Second, 6: 5 * time.Minute, 7: 5 * time.Minute, 50: 5 * time.Minute,
	} {
		if got := CrashBackoff(n); got != want {
			t.Errorf("CrashBackoff(%d) = %s, want %s", n, got, want)
		}
	}
}

// A crashed attempt is not retried at the next pass: the Job stays Ready, says when it will be
// tried, and is assigned once the wait is over. The second wait is twice the first.
func TestCrashRetryWaitsOutTheBackoff(t *testing.T) {
	f := setup(t)
	f.s.RetryDelay = CrashBackoff
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.exit(t, "crash")
	out := f.tick(t)
	if out.Assigned != 0 {
		t.Fatal("a crashed attempt was retried at once")
	}
	if len(out.Rows) != 1 || !strings.HasPrefix(out.Rows[0].Why, "Waiting to retry after a failed attempt") {
		t.Fatalf("rows %+v", out.Rows)
	}
	f.now = f.now.Add(9 * time.Second)
	if f.tick(t).Assigned != 0 {
		t.Fatal("retried before the first wait of 10 s was over")
	}
	f.now = f.now.Add(time.Second)
	if f.tick(t).Assigned != 1 || f.state(t).Attempt != 2 {
		t.Fatalf("not retried after the wait: %+v", f.state(t))
	}
	f.ack(t)
	f.exit(t, "crash")
	if f.tick(t).Assigned != 0 {
		t.Fatal("the second crash was retried at once")
	}
	f.now = f.now.Add(19 * time.Second)
	if f.tick(t).Assigned != 0 {
		t.Fatal("retried before the second wait of 20 s was over")
	}
	f.now = f.now.Add(time.Second)
	if f.tick(t).Assigned != 1 || f.state(t).Attempt != 3 {
		t.Fatalf("not retried after the second wait: %+v", f.state(t))
	}
}

// With no RetryDelay the next pass retries, as before.
func TestCrashRetryWithoutDelayIsImmediate(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.exit(t, "crash")
	if f.tick(t).Assigned != 1 {
		t.Fatal("not retried")
	}
}
