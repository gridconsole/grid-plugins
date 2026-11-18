package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// ccMechanical writes what a run leaves in the task folder as a side effect: its
// session hook file and a usage row. Neither is work, but both are files of the
// task, so they change the progress hash on every attempt.
func ccMechanical(t *testing.T, f *fixture, n int) {
	t.Helper()
	dir := filepath.Join(f.s.Feed.TaskDir, "sessions", ".hooks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("s%d.json", n)), []byte(fmt.Sprintf(`{"n":%d}`, n)), 0o600); err != nil {
		t.Fatal(err)
	}
	usage := filepath.Join(f.s.Feed.TaskDir, "usage")
	if err := os.MkdirAll(usage, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(usage, "usage.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := fmt.Fprintf(file, `{"attempt":%d,"cost":0}`+"\n", n); err != nil {
		t.Fatal(err)
	}
}

// The live storm: every attempt wrote a hook file and a usage row, the progress hash changed
// each pass, and the crash count cleared each time, so the park never came. Three crashes park
// the Job whatever the task folder holds in between.
func TestCrashCapHoldsWhenEveryAttemptWritesFiles(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for n := 1; n <= 3; n++ {
		f.ack(t)
		ccMechanical(t, f, n)
		f.exit(t, "crash")
		f.tick(t)
	}
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatalf("three crashes did not park the Job: %s %s attempt %d", j.Status, j.Reason, j.Attempt)
	}
}

// Real work between crashes (a new file in the task, a checkpoint) does not reopen the count:
// the cap counts crashes from the Feed, as the empty finish limit does.
func TestCrashCapIgnoresProgress(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for n := 1; n <= 3; n++ {
		f.ack(t)
		if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, fmt.Sprintf("research-%d.md", n)), []byte("new evidence"), 0o600); err != nil {
			t.Fatal(err)
		}
		f.exit(t, "crash")
		f.tick(t)
	}
	if j := f.state(t); j.Status != contract.Parked {
		t.Fatalf("progress between crashes reopened the count: %s %s", j.Status, j.Reason)
	}
}

// A crash with no exit code and no work at all is still a crash: the counter must not treat a
// fast exit as no attempt.
func TestCrashCapCountsFastCrashes(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 2 {
		f.ack(t)
		f.exit(t, "")
		f.tick(t)
	}
	if j := f.state(t); j.Status == contract.Parked {
		t.Fatalf("parked after two: %s", j.Reason)
	}
	f.ack(t)
	f.exit(t, "")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatalf("an exit with no reason did not count: %s %s", j.Status, j.Reason)
	}
}

// A usage_limit exit is no crash and no failed attempt: it neither counts nor ends the streak.
func TestCrashCapSkipsUsageLimit(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 2 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	f.ack(t)
	yaUrExit(t, f, nil)
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "usage_limit" {
		t.Fatalf("a usage limit counted as a crash: %s %s", j.Status, j.Reason)
	}
	f.s.UsageReady = func(string) bool { return true }
	if f.tick(t).Assigned != 1 {
		t.Fatal("not offered again")
	}
	f.ack(t)
	f.exit(t, "crash")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatalf("the third crash did not park: %s %s", j.Status, j.Reason)
	}
}

// An attempt that ended done ends the streak: two crashes, one that ran to its end, then two
// more crashes do not reach three in a row.
func TestCrashCapEndsAtAnAttemptThatEndedDone(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 2 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	f.ack(t)
	f.exit(t, "done")
	f.tick(t)
	for range 2 {
		if j := f.state(t); j.Status == contract.Parked {
			t.Fatalf("an attempt that ended done did not end the streak: %s %s", j.Status, j.Reason)
		}
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	if j := f.state(t); j.Status == contract.Parked {
		t.Fatalf("two crashes after a done attempt parked the Job: %s %s", j.Status, j.Reason)
	}
}
