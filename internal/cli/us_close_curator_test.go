package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// usCloseReads is the read pointers of a curator body, as text.
func usCloseReads(t *testing.T, body map[string]any) []string {
	t.Helper()
	var out []string
	reads, _ := body["read"].([]any)
	for _, r := range reads {
		s, _ := r.(string)
		out = append(out, s)
	}
	return out
}

// usCloseProposal is a home-level proposal of the brain, in the shape the keeper's queue reads strictly.
func usCloseProposal(name, source, originID string) string {
	return "---\nname: " + name + "\ntopic: testing\ntype: reference\ndescription: A fact\nsummary: A fact\nupdated: 2026-01-01T00:00:00Z\nsource: " + source +
		"\nsuperseded_by: \"\"\nlinks: []\nlevel: home\nstatus: proposed\nreason: \"\"\norigin:\n  kind: job\n  id: " + originID + "\n  tier: agent\n---\nA fact.\n"
}

// Row 3: the hook the worker sets answers the curator body deliver would post, naming every waiting
// proposal of the task, under the Job id the stream gives it and the T-NN id of the task.
func TestUsCloseHookNamesEachWaitingProposal(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact", "second-fact")
	// The stream passes the task's Feed id; the Job names the T-NN id the keeper's commands take.
	body, err := workerClosedCurator(h.env)(dir, store.TaskID, "J-7")
	if err != nil || body == nil {
		t.Fatalf("hook answered %v, %v", body, err)
	}
	want := []string{"brain-proposals/first-fact.md", "brain-proposals/second-fact.md"}
	if got := usCloseReads(t, body); !reflect.DeepEqual(got, want) {
		t.Fatalf("read pointers %v, want %v", got, want)
	}
	if !reflect.DeepEqual(body, curatorJobBody("J-7", id, []string{"first-fact", "second-fact"})) {
		t.Errorf("the body is not the one deliver's curator Job gets: %v", body)
	}
	if body["id"] != "J-7" || body["role"] != "curator" {
		t.Errorf("id %v, role %v", body["id"], body["role"])
	}
	accept, _ := json.Marshal(body["acceptance"])
	if !strings.Contains(string(accept), "grid brain keep --task "+id) || strings.Contains(string(accept), store.TaskID) {
		t.Errorf("acceptance does not name the task as %s: %s", id, accept)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Errorf("the body is not a valid job-record: %v", err)
	}
}

// Row 3: a task with no proposal waiting gets nil, so the stream posts nothing.
func TestUsCloseHookAnswersNilWhenNothingWaits(t *testing.T) {
	h, dir, _, store := keeperHarness(t)
	body, err := workerClosedCurator(h.env)(dir, store.TaskID, "J-7")
	if err != nil || body != nil {
		t.Fatalf("hook answered %v, %v for a task with no proposal waiting", body, err)
	}
}

// A home-level proposal counts for the task it names and not for another task: WaitingForTask's rule,
// which deliver uses, so `grid brain keep` still lists the rest.
func TestUsCloseHookCountsOnlyThisTasksHomeProposals(t *testing.T) {
	h, dir, _, store := keeperHarness(t)
	h.write(filepath.Join(h.gridHome, "brain-proposals", "by-task.md"), usCloseProposal("by-task", "T-01", "T-01/J-01"))
	h.write(filepath.Join(h.gridHome, "brain-proposals", "other-task.md"), usCloseProposal("other-task", "T-70", "T-70/J-01"))
	body, err := workerClosedCurator(h.env)(dir, store.TaskID, "J-7")
	if err != nil || body == nil {
		t.Fatalf("hook answered %v, %v", body, err)
	}
	// A home proposal is not in the task folder, so the Job names it by its keep path, not a read.
	if got := usCloseReads(t, body); len(got) != 0 {
		t.Fatalf("read pointers %v, want none", got)
	}
	acc, _ := body["acceptance"].([]any)
	text, _ := acc[0].(map[string]any)["text"].(string)
	if !strings.Contains(text, "brain:home/brain-proposals/by-task.md") || strings.Contains(text, "other-task") {
		t.Fatalf("acceptance names the wrong proposals: %s", text)
	}
}

// Row 2 on the real worker: `grid worker next` in a child process, on a task a person declined with a brain
// proposal waiting. It cancels J-1, posts the curator Job J-2 under the task lease from the hook worker.go
// sets, and prints the spawn line of J-2.
func TestUsCloseWorkerNextCuratesADeclinedTask(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact")
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
	h.write(filepath.Join(h.gridHome, "machine"), feedMachine+"\n")
	// keeperHarness leaves the task lease with its owner; give it back, and post J-1 as grid task new does.
	w := feed.Writer{Author: feed.Author{ID: "local", Role: "owner"}, Worker: "local@" + feedMachine, Lease: "task"}
	ts := lease.TaskStore{Feed: store}
	tn, err := ts.Read("task")
	if err != nil {
		t.Fatal(err)
	}
	w.Gen, w.Fence = tn.Gen, tn.Fence
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
	ownerPost(t, dir, store.TaskID, "J-1")
	if code := h.run(h.home, "task", "decline", id, "--reason", "not needed"); code != 0 {
		t.Fatalf("task decline: %d %s", code, h.errb)
	}

	pid, start := taskLeaseIdentity(t)
	var stderr bytes.Buffer
	cmd, lines := koWnStart(t, h.gridHome, pid, start, &stderr)
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	defer func() { _ = cmd.Process.Kill() }()
	sp := koWnSpawn(t, lines, &stderr)
	// The spawn line is printed after the Job is assigned: the Feed holds the whole outcome now.
	jobs, snap := curatorJobs(t, store)
	if len(jobs) != 1 || strings.Join(jobs[0].Read, ",") != "brain-proposals/first-fact.md" {
		t.Fatalf("curator Jobs on the Feed: %+v\nstderr: %s", jobs, &stderr)
	}
	curator := jobs[0].ID
	if sp["task"] != store.TaskID || sp["id"] != curator {
		t.Fatalf("spawn line %v, want the curator Job %s of the task\nstderr: %s", sp, curator, &stderr)
	}
	if st := snap.State.Jobs[curator]; st == nil || st.Status != contract.Assigned {
		t.Errorf("curator Job %s: %+v", curator, st)
	}
	if st := snap.State.Jobs["J-1"]; st == nil || st.Status != contract.Cancelled {
		t.Errorf("J-1: %+v", st)
	}
	for _, it := range snap.Items {
		if st := snap.State.Jobs[it.ID]; it.ID != curator && st != nil && st.Status != contract.Accepted && st.Status != contract.Cancelled && st.Status != contract.Rejected {
			t.Errorf("Job %s (%s) of the closed task is still open: %s", it.ID, it.Role, st.Status)
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-exited:
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 130 {
			t.Fatalf("SIGINT exit: %v\nstderr: %s", err, &stderr)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("the stream did not stop on SIGINT\nstderr: %s", &stderr)
	}
}
