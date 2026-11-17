package board

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

const (
	plBdTask    = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	plBdMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	// plBdChildEnv carries a plBdChild to the test binary re-run as a second process.
	plBdChildEnv = "GRID_PL_BD_CLAIM_CHILD"
	plBdWon      = "pl-bd-claim: won"
	plBdLost     = "pl-bd-claim: claimed"
)

var plBdKey = Key{Task: plBdTask, Job: "J-1", Attempt: 1}

// plBdChild is what a claiming process needs: the task folder, the worker's
// writer and the assign its ack names.
type plBdChild struct {
	Dir    string
	Writer feed.Writer
	Assign string
}

// plBdAssigned makes a task Feed with J-1 attempt 1 assigned to a worker, as the
// tick does: a task lease and handover, the Job record, a Job lease and the
// assign control.
func plBdAssigned(t *testing.T) plBdChild {
	t.Helper()
	dir := t.TempDir()
	s := feed.Store{TaskDir: dir, TaskID: plBdTask}
	task := feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@" + plBdMachine, Lease: "task"}
	now := time.Now().UTC()
	grant, err := (lease.TaskStore{Feed: s}).TakeTask(task, os.Getpid(), contract.FormatTime(now))
	if err != nil {
		t.Fatal(err)
	}
	task.Fence = grant.Fence
	if _, err := s.Handover(feed.Input{Body: map[string]any{}}, task); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "job-record", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var job map[string]any
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatal(err)
	}
	job["id"] = "J-1"
	if _, err := s.Post(feed.Input{Type: "job", Job: "J-1", Body: job}, task); err != nil {
		t.Fatal(err)
	}
	to := "worker@" + plBdMachine
	jl, err := (lease.TaskStore{Feed: s}).GrantJob(task, "J-1", to, 1)
	if err != nil {
		t.Fatal(err)
	}
	a := contract.Assignment{Job: "J-1", Attempt: 1, To: to, Fence: jl.Fence, Until: contract.FormatTime(now.Add(time.Hour))}
	assign, err := s.Post(feed.Input{Type: "control", Body: map[string]any{"assign": a}}, task)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	j := state.Jobs["J-1"]
	if j == nil || j.Status != contract.Assigned || j.AssignPost != assign.ID {
		t.Fatalf("J-1 not assigned: %+v", j)
	}
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: to, Lease: "J-1", Fence: j.Assign.Fence, Gen: j.Generation}
	return plBdChild{Dir: dir, Writer: w, Assign: assign.ID}
}

// plBdClaim claims J-1 attempt 1 with an ack, each call a new post id.
func plBdClaim(c plBdChild) (feed.Record, error) {
	b := JSONL{Feed: feed.Store{TaskDir: c.Dir, TaskID: plBdTask}}
	return b.Claim(plBdKey, feed.Input{Type: "ack", Job: "J-1", Attempt: 1, After: []string{c.Assign}, Body: map[string]any{}}, c.Writer)
}

// plBdAcks returns the ids of the acks of J-1 attempt 1 on the Feed.
func plBdAcks(t *testing.T, c plBdChild) []string {
	t.Helper()
	read, err := (JSONL{Feed: feed.Store{TaskDir: c.Dir, TaskID: plBdTask}}).Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range read.Records {
		if plBdKey.holds(e.Record, "ack") {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// plBdOneAck checks the Feed holds exactly the winner's ack and that it counts.
func plBdOneAck(t *testing.T, c plBdChild) {
	t.Helper()
	if acks := plBdAcks(t, c); len(acks) != 1 {
		t.Fatalf("want one ack of J-1 attempt 1 on the Feed, got %v", acks)
	}
	state, err := (feed.Store{TaskDir: c.Dir, TaskID: plBdTask}).State()
	if err != nil {
		t.Fatal(err)
	}
	if j := state.Jobs["J-1"]; j == nil || j.Status != contract.Running {
		t.Fatalf("J-1 after the claim: %+v", j)
	}
}

// Row 2 (V9), one process: Claim twice on one key gives ErrClaimed, and so do all
// but one of several Claims at once.
func TestPlBdClaimTwiceInProcess(t *testing.T) {
	t.Run("one after the other", func(t *testing.T) {
		c := plBdAssigned(t)
		first, err := plBdClaim(c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := plBdClaim(c); !errors.Is(err, ErrClaimed) || !strings.Contains(err.Error(), first.ID) {
			t.Fatalf("second Claim: %v, want ErrClaimed naming %s", err, first.ID)
		}
		plBdOneAck(t, c)
	})
	t.Run("at once", func(t *testing.T) {
		c := plBdAssigned(t)
		errs := make([]error, 8)
		var wg sync.WaitGroup
		for i := range errs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, errs[i] = plBdClaim(c)
			}()
		}
		wg.Wait()
		won := 0
		for _, err := range errs {
			switch {
			case err == nil:
				won++
			case !errors.Is(err, ErrClaimed):
				t.Fatalf("Claim: %v", err)
			}
		}
		if won != 1 {
			t.Fatalf("%d Claims won, want 1", won)
		}
		plBdOneAck(t, c)
	})
}

// TestPlBdClaimChild is the second process of TestPlBdClaimTwoProcesses: the test
// binary re-run with plBdChildEnv set. It prints whether its Claim won. Run as an
// ordinary test, without the variable, it does nothing.
func TestPlBdClaimChild(t *testing.T) {
	spec := os.Getenv(plBdChildEnv)
	if spec == "" {
		return
	}
	var c plBdChild
	if err := json.Unmarshal([]byte(spec), &c); err != nil {
		t.Fatal(err)
	}
	_, err := plBdClaim(c)
	switch {
	case err == nil:
		os.Stdout.WriteString(plBdWon + "\n")
	case errors.Is(err, ErrClaimed):
		os.Stdout.WriteString(plBdLost + "\n")
	default:
		t.Fatal(err)
	}
}

// plBdStart starts one claiming child process; its output is read after Wait.
func plBdStart(t *testing.T, c plBdChild) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	spec, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPlBdClaimChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), plBdChildEnv+"="+string(spec))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, &out
}

// plBdWait waits for the child to exit and returns whether its Claim won.
func plBdWait(t *testing.T, cmd *exec.Cmd, out *bytes.Buffer) bool {
	t.Helper()
	err := cmd.Wait()
	won, lost := strings.Contains(out.String(), plBdWon), strings.Contains(out.String(), plBdLost)
	if err != nil || won == lost {
		t.Fatalf("child: %v\n%s", err, out)
	}
	return won
}

// plBdRun runs one claiming child to its exit and returns whether its Claim won.
func plBdRun(t *testing.T, c plBdChild) bool {
	t.Helper()
	cmd, out := plBdStart(t, c)
	return plBdWait(t, cmd, out)
}

// Row 2 (V9), two processes: the second caller gets ErrClaimed also when the
// first is another process, and of processes claiming at once one wins.
func TestPlBdClaimTwoProcesses(t *testing.T) {
	t.Run("this process first, then a child", func(t *testing.T) {
		c := plBdAssigned(t)
		if _, err := plBdClaim(c); err != nil {
			t.Fatal(err)
		}
		if plBdRun(t, c) {
			t.Fatal("the child's Claim won after this process claimed")
		}
		plBdOneAck(t, c)
	})
	t.Run("a child first, then this process", func(t *testing.T) {
		c := plBdAssigned(t)
		if !plBdRun(t, c) {
			t.Fatal("the child's Claim lost on an unclaimed key")
		}
		if _, err := plBdClaim(c); !errors.Is(err, ErrClaimed) {
			t.Fatalf("Claim after the child: %v, want ErrClaimed", err)
		}
		plBdOneAck(t, c)
	})
	t.Run("children at once", func(t *testing.T) {
		c := plBdAssigned(t)
		type child struct {
			cmd *exec.Cmd
			out *bytes.Buffer
		}
		var children []child
		for range 4 {
			cmd, out := plBdStart(t, c)
			children = append(children, child{cmd, out})
		}
		won := 0
		for _, ch := range children {
			if plBdWait(t, ch.cmd, ch.out) {
				won++
			}
		}
		if won != 1 {
			t.Fatalf("%d child Claims won, want 1", won)
		}
		plBdOneAck(t, c)
	})
}

// The key a claim post must carry, and which posts hold it.
func TestPlBdKey(t *testing.T) {
	firing := "2026-11-17T06:00:00.000Z"
	k := Key{Task: plBdTask, Job: "J-1", Attempt: 1, Firing: firing}
	started := feed.Input{Type: "started", Job: "J-1", Attempt: 1, Body: map[string]any{"firing": firing}}
	if why := k.fits(plBdTask, started); why != "" {
		t.Fatalf("a started post with the key's firing: %s", why)
	}
	for name, in := range map[string]feed.Input{
		"another Job":     {Type: "started", Job: "J-2", Attempt: 1, Body: started.Body},
		"another attempt": {Type: "started", Job: "J-1", Attempt: 2, Body: started.Body},
		"no firing":       {Type: "started", Job: "J-1", Attempt: 1, Body: map[string]any{}},
		"no type":         {Job: "J-1", Attempt: 1, Body: started.Body},
	} {
		if k.fits(plBdTask, in) == "" {
			t.Errorf("%s: a post that does not carry the key was let through", name)
		}
	}
	if k.fits("01ARZ3NDEKTSV4RRFFQ69G5FAX", started) == "" {
		t.Error("a key of another task was let through")
	}
	job, attempt := "J-1", 1
	r := feed.Record{Task: plBdTask, Type: "started", Job: &job, Attempt: &attempt, Body: feed.Body{"firing": firing}}
	if !k.holds(r, "started") {
		t.Fatal("the post with the key does not hold it")
	}
	if k.holds(r, "ack") {
		t.Error("a post of another type holds the key")
	}
	other := r
	other.Body = feed.Body{"firing": "2026-11-17T07:00:00.000Z"}
	if k.holds(other, "started") {
		t.Error("a post of another firing holds the key")
	}
	if !(Key{Task: plBdTask, Job: "J-1", Attempt: 1}).holds(other, "started") {
		t.Error("a key with no firing does not match on task, Job and attempt alone")
	}
	two := 2
	other.Attempt = &two
	if k.holds(other, "started") {
		t.Error("a post of another attempt holds the key")
	}
	_, err := (JSONL{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: plBdTask}}).Claim(k, feed.Input{Type: "ack", Job: "J-1", Attempt: 1, Body: map[string]any{}}, feed.Writer{})
	if err == nil || errors.Is(err, ErrClaimed) {
		t.Fatalf("an ack cannot carry a firing key: %v", err)
	}
}
