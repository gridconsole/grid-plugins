package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// An approval waits for .orchestrator.lock to post the person Job result
// without holding .task.lock. A tick takes .orchestrator.lock and then
// .task.lock (publishPlan), so the opposite order could hang both for good.
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

			done := make(chan int, 1)
			go func() { done <- h.run(path, args...) }()
			// approval.json is written under .task.lock; after that the command
			// must let go of .task.lock while .orchestrator.lock is still taken.
			free := false
			for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline) && !free; time.Sleep(10 * time.Millisecond) {
				select {
				case code := <-done:
					release()
					t.Fatalf("finished while .orchestrator.lock was held: %d %s", code, h.errb.String())
				default:
				}
				if _, err := os.Stat(filepath.Join(path, "approval.json")); err == nil && fslock.TryLockExclusive(taskLock) == nil {
					fslock.Unlock(taskLock)
					free = true
				}
			}
			release()
			select {
			case code := <-done:
				if code != 0 {
					t.Fatalf("%d %s", code, h.errb.String())
				}
			case <-time.After(10 * time.Second):
				t.Fatal("did not finish after .orchestrator.lock was released")
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
