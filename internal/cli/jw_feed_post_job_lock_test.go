package cli

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// jwFjWait is the failure path of a wait that the fixed code never reaches: every
// wait below is on a channel fed by fslock.OnBlockForTest or by a command's exit.
const jwFjWait = 30 * time.Second

// jwFjLockPath is the pass lock of the task, the file a holder pass and every
// Job post take.
func jwFjLockPath(task string) string {
	return filepath.Join(feed.Store{TaskDir: task}.Dir(), ".orchestrator.lock")
}

// jwFjHold takes .orchestrator.lock of the task on a descriptor of the test's own,
// as a holder pass does, and gives back the unlock. The unlock is idempotent and
// runs at cleanup, so a failed test never leaves a command waiting.
func jwFjHold(t *testing.T, task string) (release func()) {
	t.Helper()
	if err := os.MkdirAll(feed.Store{TaskDir: task}.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := fslock.OpenNoFollow(jwFjLockPath(task), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	var once sync.Once
	release = func() {
		once.Do(func() {
			fslock.Unlock(f)
			f.Close()
		})
	}
	t.Cleanup(release)
	return release
}

// jwFjWatch sets the hook that says a lock call on .orchestrator.lock of the task
// is about to wait. queued is sent the first such event; waits counts all of them.
// The hook only compares the path and sends without blocking; the test does not
// call t.Parallel and the previous hook is put back at cleanup.
func jwFjWatch(t *testing.T, task string) (queued <-chan struct{}, waits func() int) {
	t.Helper()
	want := jwFjLockPath(task)
	ch := make(chan struct{}, 1)
	var n atomic.Int32
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if path != want {
			return
		}
		n.Add(1)
		select {
		case ch <- struct{}{}:
		default:
		}
	}))
	return ch, func() int { return int(n.Load()) }
}

// jwFjPostHeld runs post in a goroutine while the test holds .orchestrator.lock.
// It waits for the event that the post queued on the lock, which it cannot finish
// before, then unlocks and returns the post's exit code. A post that finishes
// first never waited on the lock, and that fails the test.
func jwFjPostHeld(t *testing.T, task string, post func() int) int {
	t.Helper()
	release := jwFjHold(t, task)
	queued, _ := jwFjWatch(t, task)
	done := make(chan int, 1)
	go func() { done <- post() }()
	select {
	case code := <-done:
		t.Fatalf("the post finished with exit %d while .orchestrator.lock was held, and with no block event on it", code)
	case <-queued:
	case <-time.After(jwFjWait):
		t.Fatal("timed out waiting for the post to queue on .orchestrator.lock")
	}
	release()
	select {
	case code := <-done:
		return code
	case <-time.After(jwFjWait):
		t.Fatal("the post did not finish after .orchestrator.lock was released")
	}
	return -1
}

// jwFjJobs reads the `job` records of the Feed, whatever their verdict, by body id.
func jwFjJobs(t *testing.T, store feed.Store) map[string]string {
	t.Helper()
	r, err := store.ReadOnly(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, e := range r.Records {
		id, _ := e.Body["id"].(string)
		out[id] = string(e.Verdict)
	}
	return out
}

// A member's `grid feed post --type job` has no lease: it takes the pass lock
// through exclusive, and waits for a holder pass that has it.
func TestJwFjFeedPostJobWaitsForPassLock(t *testing.T) {
	h, task := feedHarness(t)
	code := jwFjPostHeld(t, task, func() int {
		return h.run(task, "feed", "post", "--type", "job", "--job", "J-1", "--body", personBody(t, "J-1", nil), "--json")
	})
	if code != 0 {
		t.Fatalf("feed post --type job after the unlock: %d %s", code, h.errb)
	}
	if got := jwFjJobs(t, feed.Store{TaskDir: task, TaskID: feedTaskID}); len(got) != 1 || got["J-1"] == "" {
		t.Fatalf("job records after the post: %v", got)
	}
}

// A writer with the task lease goes in through Store.locked: the lease first,
// then the pass lock. It waits for the holder pass the same way.
func TestJwFjLeasedFeedPostJobWaitsForPassLock(t *testing.T) {
	h, path, store, _ := approveJobHarness(t)
	code := jwFjPostHeld(t, path, func() int {
		return h.run(path, "feed", "post", "--type", "job", "--job", "J-3", "--lease", "task", "--body", personBody(t, "J-3", nil))
	})
	if code != 0 {
		t.Fatalf("leased feed post --type job after the unlock: %d %s", code, h.errb)
	}
	if got := jwFjJobs(t, store); got["J-3"] == "" {
		t.Fatalf("J-3 is not on the Feed: %v", got)
	}
}

// The MCP feed_post tool takes the same lock for a job record.
func TestJwFjMCPFeedPostJobWaitsForPassLock(t *testing.T) {
	h, task := feedHarness(t)
	t.Setenv("GRID_FEED_TASK", task)
	body := strings.TrimSpace(personBody(t, "J-1", nil))
	h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"feed_post","arguments":{"type":"job","job":"J-1","body":` + body + `}}}` + "\n")
	code := jwFjPostHeld(t, task, func() int { return h.run(h.home, "mcp") })
	if code != 0 || h.errb.Len() != 0 {
		t.Fatalf("mcp after the unlock: %d %s %s", code, h.out, h.errb)
	}
	lines := strings.Split(strings.TrimSpace(h.out.String()), "\n")
	if last := lines[len(lines)-1]; strings.Contains(last, `"isError":true`) {
		t.Fatalf("feed_post answered an error: %s", last)
	}
	if got := jwFjJobs(t, feed.Store{TaskDir: task, TaskID: feedTaskID}); len(got) != 1 || got["J-1"] == "" {
		t.Fatalf("job records after the MCP post: %v", got)
	}
}

// A held seed (a job whose after names a record not on the Feed) is still taken
// by feed post: PostJobLocked is not PostPerson and checks no after-target.
func TestJwFjHeldSeedStillAccepted(t *testing.T) {
	h, task := feedHarness(t)
	missing, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "feed", "post", "--type", "job", "--job", "J-3", "--after", missing, "--body", personBody(t, "J-3", nil)); code != 0 {
		t.Fatalf("the held seed: %d %s", code, h.errb)
	}
	if got := jwFjJobs(t, feed.Store{TaskDir: task, TaskID: feedTaskID}); got["J-3"] != "held" {
		t.Fatalf("job records %v, want J-3 held", got)
	}
}

// Another type is unchanged: with the pass lock held, a message post finishes at
// once and never waits on .orchestrator.lock.
func TestJwFjFeedPostOtherTypeTakesNoPassLock(t *testing.T) {
	h, task := feedHarness(t)
	jwFjHold(t, task)
	_, waits := jwFjWatch(t, task)
	if code := h.run(task, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"example"}`); code != 0 {
		t.Fatalf("message post with the lock held: %d %s", code, h.errb)
	}
	if n := waits(); n != 0 {
		t.Fatalf("a message post waited %d time(s) on .orchestrator.lock", n)
	}
}
