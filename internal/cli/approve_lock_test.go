package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// An approval waits for .orchestrator.lock to post the person Job result
// without holding .task.lock. A tick takes .orchestrator.lock and then
// .task.lock (publishPlan), so the opposite order could hang both for good.
//
// The test holds .orchestrator.lock and parks the command at the seam between
// Approve and the post, where it checks that approval.json is written and
// .task.lock is free. No time window is involved: the command and the test hand
// over through channels. A command that holds .task.lock across the post, or
// that never gets there, fails the check or stops the test at its go test timeout.
func TestApprovalPostDoesNotHoldTaskLockWhileWaitingForOrchestratorLock(t *testing.T) {
	for _, name := range []string{"approve", "job result"} {
		t.Run(name, func(t *testing.T) {
			h, path, store, text := approveJobHarness(t)
			args := []string{"approve", path, "--plan-hash", plan.Hash(text)}
			if name == "job result" {
				args = []string{"job", "result", "J-2", "--plan-hash", plan.Hash(text), "--body", `{"answer":"yes"}`}
			}
			pass, err := fslock.OpenNoFollow(filepath.Join(store.Dir(), ".orchestrator.lock"), os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			defer pass.Close()
			if err := fslock.LockExclusive(pass); err != nil {
				t.Fatal(err)
			}
			held := true
			release := func() {
				if held {
					held = false
					fslock.Unlock(pass)
				}
			}
			defer release()
			taskLock, err := fslock.OpenNoFollow(filepath.Join(path, ".task.lock"), os.O_CREATE|os.O_RDWR, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			defer taskLock.Close()

			reached, proceed := make(chan struct{}), make(chan struct{})
			h.env.afterApprove = func() {
				close(reached)
				<-proceed
			}
			done := make(chan int, 1)
			go func() { done <- h.run(path, args...) }()
			select {
			case <-reached:
			case code := <-done:
				release()
				t.Fatalf("finished without reaching the post, or while .orchestrator.lock was held: %d %s", code, h.errb.String())
			}
			// Approve has returned and the post has not started: the command is
			// parked, and .orchestrator.lock is still held by this test.
			_, statErr := os.Stat(filepath.Join(path, "approval.json"))
			free := fslock.TryLockExclusive(taskLock) == nil
			if free {
				fslock.Unlock(taskLock)
			}
			release()
			close(proceed)
			if code := <-done; code != 0 {
				t.Fatalf("%d %s", code, h.errb.String())
			}
			if statErr != nil {
				t.Fatalf("approval.json was not written before the post: %v", statErr)
			}
			if !free {
				t.Fatal(".task.lock was held while waiting for .orchestrator.lock")
			}
			if body := approvalResult(t, store); body == nil || body["plan_digest"] != plan.Digest(text) {
				t.Fatalf("Job result %v", body)
			}
		})
	}
}
