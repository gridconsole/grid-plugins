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
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

const (
	pbStTask  = "01K9J0000000000000000000T1"
	pbStHomeA = "01K9J00000000000000000000A"
	// pbStChildEnv carries a pbStChild to the test binary re-run as a second process.
	pbStChildEnv = "GRID_PB_ST_CLAIM_CHILD"
	pbStWon      = "pb-st-claim: won"
	pbStLost     = "pb-st-claim: claimed"
)

var (
	pbStKey    = Key{Task: pbStTask, Job: "J-1", Attempt: 1}
	pbStWorker = feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}}
)

// pbStChild is what a claiming process needs: the task folder, the machine folder
// and the assign its ack names.
type pbStChild struct {
	Dir, Machine, Assign string
}

// board is the per-post board of the task as machine A sees it.
func (c pbStChild) board() PerPost {
	return PerPost{Feed: feed.Store{
		TaskDir: c.Dir,
		Machine: machinefolder.Folder{Path: c.Machine, Source: machinefolder.SourceEnv},
		Board:   &feed.BoardHome{Rel: "projects/demo/tasks/T-1", By: pbStHomeA + ":grid"},
	}}
}

// pbStAssigned makes a per-post board with J-1's record naming home A and the cli's
// assign of attempt 1 to worker@A, written on A.
func pbStAssigned(t *testing.T) pbStChild {
	t.Helper()
	c := pbStChild{Dir: t.TempDir(), Machine: t.TempDir()}
	md := "---\nid: " + pbStTask + "\ntitle: Board task\nhome: " + pbStHomeA + "\n---\n"
	if err := os.WriteFile(filepath.Join(c.Dir, "task.md"), []byte(md), 0o600); err != nil {
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
	job["id"], job["home"] = "J-1", pbStHomeA
	b := c.board()
	if _, err := b.Post(feed.Input{Type: "job", Job: "J-1", Body: job}, feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}}); err != nil {
		t.Fatal(err)
	}
	a := contract.Assignment{Job: "J-1", Attempt: 1, To: "worker@" + pbStHomeA, Fence: 1, Until: contract.FormatTime(time.Now().UTC().Add(time.Hour))}
	assign, err := b.Post(feed.Input{Type: "control", Body: map[string]any{"assign": a}}, feed.Writer{Author: feed.Author{ID: "orchestrator", Role: "cli"}})
	if err != nil {
		t.Fatal(err)
	}
	c.Assign = assign.ID
	return c
}

func (c pbStChild) ack() feed.Input {
	return feed.Input{Type: "ack", Job: "J-1", Attempt: 1, After: []string{c.Assign}, Body: map[string]any{}}
}

// pbStClaim claims J-1 attempt 1 with an ack, each call a new post id.
func pbStClaim(c pbStChild) (feed.Record, error) {
	return c.board().Claim(pbStKey, c.ack(), pbStWorker)
}

// pbStOneAck checks the board holds exactly one ack file of J-1 and that it counts.
func pbStOneAck(t *testing.T, c pbStChild) contract.State {
	t.Helper()
	acks, err := filepath.Glob(filepath.Join(c.Dir, "board", "*-J-1-ack.md"))
	if err != nil || len(acks) != 1 {
		t.Fatalf("want one ack file of J-1 on the board, got %v %v", acks, err)
	}
	state, err := c.board().Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := state.Jobs["J-1"]; j == nil || j.Status != contract.Running {
		t.Fatalf("J-1 after the claim: %+v", j)
	}
	return state
}

// TestPbStClaimChild is the second process of TestPbStClaimTwoProcesses: the test
// binary re-run with pbStChildEnv set. It prints whether its Claim won. Run as an
// ordinary test, without the variable, it does nothing.
func TestPbStClaimChild(t *testing.T) {
	spec := os.Getenv(pbStChildEnv)
	if spec == "" {
		return
	}
	var c pbStChild
	if err := json.Unmarshal([]byte(spec), &c); err != nil {
		t.Fatal(err)
	}
	_, err := pbStClaim(c)
	switch {
	case err == nil:
		os.Stdout.WriteString(pbStWon + "\n")
	case errors.Is(err, ErrClaimed):
		os.Stdout.WriteString(pbStLost + "\n")
	default:
		t.Fatal(err)
	}
}

// pbStStart starts one claiming child process; its output is read after Wait.
func pbStStart(t *testing.T, c pbStChild) (*exec.Cmd, *bytes.Buffer) {
	t.Helper()
	spec, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPbStClaimChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), pbStChildEnv+"="+string(spec))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd, &out
}

// pbStWait waits for the child to exit and returns whether its Claim won.
func pbStWait(t *testing.T, cmd *exec.Cmd, out *bytes.Buffer) bool {
	t.Helper()
	err := cmd.Wait()
	won, lost := strings.Contains(out.String(), pbStWon), strings.Contains(out.String(), pbStLost)
	if err != nil || won == lost {
		t.Fatalf("child: %v\n%s", err, out)
	}
	return won
}

// Row 5 (V9): Claim twice from two processes gives one post and one ErrClaimed, in
// either order, and of processes claiming at once one wins.
func TestPbStClaimTwoProcesses(t *testing.T) {
	t.Run("this process first, then a child", func(t *testing.T) {
		c := pbStAssigned(t)
		if _, err := pbStClaim(c); err != nil {
			t.Fatal(err)
		}
		if cmd, out := pbStStart(t, c); pbStWait(t, cmd, out) {
			t.Fatal("the child's Claim won after this process claimed")
		}
		pbStOneAck(t, c)
	})
	t.Run("a child first, then this process", func(t *testing.T) {
		c := pbStAssigned(t)
		if cmd, out := pbStStart(t, c); !pbStWait(t, cmd, out) {
			t.Fatal("the child's Claim lost on an unclaimed key")
		}
		first, err := pbStClaim(c)
		if !errors.Is(err, ErrClaimed) {
			t.Fatalf("Claim after the child: %v %+v, want ErrClaimed", err, first)
		}
		pbStOneAck(t, c)
	})
	t.Run("four children at once", func(t *testing.T) {
		c := pbStAssigned(t)
		type child struct {
			cmd *exec.Cmd
			out *bytes.Buffer
		}
		var children []child
		for range 4 {
			cmd, out := pbStStart(t, c)
			children = append(children, child{cmd, out})
		}
		won := 0
		for _, ch := range children {
			if pbStWait(t, ch.cmd, ch.out) {
				won++
			}
		}
		if won != 1 {
			t.Fatalf("%d child Claims won, want 1", won)
		}
		pbStOneAck(t, c)
	})
}

// Row 5 (V9), in one process: of goroutines claiming at once one wins, and the
// error of a lost Claim names the post that holds the key.
func TestPbStClaimInProcess(t *testing.T) {
	c := pbStAssigned(t)
	errs := make([]error, 8)
	recs := make([]feed.Record, 8)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recs[i], errs[i] = pbStClaim(c)
		}()
	}
	wg.Wait()
	winner := ""
	for i, err := range errs {
		switch {
		case err == nil && winner == "":
			winner = recs[i].ID
		case err == nil:
			t.Fatalf("two Claims won: %s and %s", winner, recs[i].ID)
		case !errors.Is(err, ErrClaimed):
			t.Fatalf("Claim: %v", err)
		}
	}
	for _, err := range errs {
		if err != nil && !strings.Contains(err.Error(), winner) {
			t.Fatalf("lost Claim %q does not name the winner %s", err, winner)
		}
	}
	pbStOneAck(t, c)
}

// Row 5 (V9), the fold half through the board: a second ack of the same attempt that
// arrives without Claim (a synced copy) folds ignored, "duplicate claim", and the Job
// still runs on the first.
func TestPbStDuplicateClaimFoldsIgnored(t *testing.T) {
	c := pbStAssigned(t)
	first, err := pbStClaim(c)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.board().Post(c.ack(), pbStWorker)
	if err != nil {
		t.Fatal(err)
	}
	state, err := c.board().Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := state.Jobs["J-1"]; j == nil || j.Status != contract.Running {
		t.Fatalf("J-1 with two acks: %+v", j)
	}
	kinds := map[string]contract.Verdict{}
	for _, v := range state.Verdicts {
		kinds[v.ID] = v
	}
	if v := kinds[first.ID]; v.Kind != "counted" {
		t.Fatalf("the claimed ack folds %+v, want counted", v)
	}
	if v := kinds[second.ID]; v.Kind != "ignored" || v.Reason != "duplicate claim" {
		t.Fatalf("the second ack folds %+v, want ignored, duplicate claim", v)
	}
}
