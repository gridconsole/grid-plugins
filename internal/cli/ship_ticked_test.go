package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// A person who holds no lease answers the ship person Job (GRID_MEMBER, no lease
// epoch): the answer is only recorded. The holder's next pass of grid worker next
// accepts it in its tick and, through the stream's Ticked call, lands the task:
// the answer is accepted, the land lines are in history and main holds the work.
func TestShipUnleasedAnswerLandsInWorkerPass(t *testing.T) {
	env, dir, laneSHA, store, stderr := shipLandFixture(t, "git merge -q --ff-only lane")
	machine := env.Machine

	// The person at a terminal: a team member with no lease; git needs PATH and HOME.
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
	// The answer is not accepted yet and nothing has landed.
	jobs, snap := approveJobs(t, store)
	if len(jobs) != 1 {
		t.Fatalf("%d ship person Jobs, want 1", len(jobs))
	}
	if st := snap.State.Jobs[jobs[0].ID]; st == nil || st.Status == contract.Accepted {
		t.Fatalf("ship Job %s is %+v before the worker pass, want an answer not yet accepted", jobs[0].ID, st)
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got == laneSHA {
		t.Fatal("main holds the work before the worker pass")
	}
	if h := shipHistory(t, dir); strings.Contains(h, " · land ") {
		t.Fatalf("history has a land line before the worker pass:\n%s", h)
	}

	// One pass of grid worker next as the lease holder, stopped once it has landed.
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
	// The pass runs at once; the stream is stopped when the land is written, or after
	// a few passes if it never is, so a missing land fails on the assertions below.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := time.AfterFunc(5*time.Second, cancel)
	defer stop.Stop()
	worker.Context = ctx
	var out bytes.Buffer
	worker.Stdout = &out
	go func() {
		for ctx.Err() == nil {
			if strings.Contains(shipHistory(t, dir), " · land landed · ") {
				cancel()
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()
	pid, start := taskLeaseIdentity(t)
	// The fixture's lease file is the one approvefix writes for the person who posted
	// the Job. The holder's own worker process takes the task lease as its tenure
	// (member@machine, its pid and start), which its stream reuses on every pass.
	holder := "dana@" + machine
	tenure, err := json.Marshal(lease.Tenure{
		Lease: feed.Lease{Lease: "task", Holder: &holder, Gen: 0, Fence: 1},
		Since: contract.FormatTime(time.Now()), PID: &pid, PIDStart: &start,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), "lease", "task.json"), append(tenure, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--member", "dana@" + machine, "--role", "member",
		"worker", "next", "--json", "--pid", strconv.Itoa(pid), "--pid-start", start}
	if code := Run(args, worker); code != 130 {
		t.Fatalf("worker next: exit %d, want 130 (stopped after the land); stderr %s", code, stderr)
	}

	if _, snap := approveJobs(t, store); acceptedShipJob(snap) == "" {
		t.Fatalf("no ship Job is accepted after the worker pass; stderr %s", stderr)
	}
	history := shipHistory(t, dir)
	for _, want := range []string{" · ship answered · ", " · land started · ", " · land landed · "} {
		if !strings.Contains(history, want) {
			t.Fatalf("history has no %q line after the worker pass:\n%s\nstderr %s", want, history, stderr)
		}
	}
	if got := shipGit(t, env.GridHome, "rev-parse", "main"); got != laneSHA {
		t.Fatalf("main is %s after the worker pass, want the delivered %s; stderr %s", got, laneSHA, stderr)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
		t.Fatalf("task state %q %v after landing, want closed", rec.State, err)
	}
}
