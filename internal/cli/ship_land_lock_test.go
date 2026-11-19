package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// A land holds .land.lock through its deploy command, up to ten minutes, and another
// process's land of the same task must not stall the holder's pass: the tick that accepts
// the answer finds the lock busy, leaves the land for a later pass and goes on. Once the
// lock is free the next pass runs the land, which was never marked as run.
func TestShipTickedPassDoesNotWaitForTheLandLockAndLandsOnTheNextPass(t *testing.T) {
	env, dir, repo, laneSHA, store, stderr := shipMergeFixture(t, false, true)
	before := shipGit(t, repo, "rev-parse", "main")
	person := env
	person.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	if code := Run([]string{"approve", "T-01"}, person); code != 0 {
		t.Fatalf("unleased approve: %d %s", code, stderr)
	}
	freeTaskLease(t, store)

	worker := env
	worker.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "GRID_ROLE":
			return "member"
		case "PATH", "HOME":
			return os.Getenv(key)
		}
		return ""
	}
	pid, start := taskLeaseIdentity(t)
	var out, diag bytes.Buffer
	s := workerStream(worker, "dana", env.Machine, "member", pid, start, &out, &diag)

	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".land.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	waited := make(chan string, 4)
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if filepath.Base(path) == ".land.lock" {
			select {
			case waited <- path:
			default:
			}
		}
	}))
	passes := make(chan error, 2)
	pass := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		passes <- s.Pass(ctx)
	}
	// A pass that waited for the lock is let go before the fixture's folders are removed.
	running := 0
	t.Cleanup(func() {
		fslock.Unlock(lock)
		lock.Close()
		for ; running > 0; running-- {
			<-passes
		}
	})

	running++
	go pass()
	select {
	case err := <-passes:
		running--
		if err != nil {
			t.Fatalf("pass: %v; diag %s", err, &diag)
		}
	case path := <-waited:
		t.Fatalf("the pass waited for %s, which another land holds through its deploy command", path)
	case <-time.After(30 * time.Second):
		t.Fatal("the pass did not return while the land lock was held")
	}
	if _, snap := approveJobs(t, store); acceptedShipJob(snap) == "" {
		t.Fatalf("no ship Job is accepted after the pass: the tick did not reach the land; stderr %s; diag %s", stderr, &diag)
	}
	if h := shipHistory(t, dir); strings.Contains(h, "grid land runs") {
		t.Fatalf("history says the land ran while another land held the lock:\n%s", h)
	}
	if got := shipGit(t, repo, "rev-parse", "main"); got != before {
		t.Fatalf("main moved to %s while another land held the lock", got)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after the skipped land, want waiting", rec.State, err)
	}

	fslock.Unlock(lock)
	pass()
	if err := <-passes; err != nil {
		t.Fatalf("second pass: %v; diag %s", err, &diag)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after the pass with the lock free, want closed; stderr %s; diag %s; history:\n%s", rec.State, err, stderr, &diag, shipHistory(t, dir))
	}
	shipGit(t, repo, "merge-base", "--is-ancestor", laneSHA, "main")
}
