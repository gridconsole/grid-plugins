package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// r23Person is a person's own terminal: a Grid home with project p and task T-01
// made by grid task new, a plan on disk, and an environment of the given
// variables only (no GRID_MEMBER unless vars holds it).
func r23Person(t *testing.T, vars map[string]string) (h *harness, path, text string) {
	t.Helper()
	h = newTaskHarness(t)
	path = newTask(t, h)
	text = "# Plan\n\nAdd the search box.\n"
	h.write(filepath.Join(path, "plan.md"), text)
	h.write(filepath.Join(path, "intent.md"), "# Intent\n\nAdd the search box.\n")
	h.env.Getenv = func(k string) string { return vars[k] }
	return h, path, text
}

// A person approves in a terminal with no GRID_MEMBER, as SessionStart tells every
// session (hook.go:376). Their task writes already work there (who local, role
// owner); the approval is recorded as that same person on the plan approve Job.
func TestR23ApproveNeedsNoGridMember(t *testing.T) {
	for _, vars := range []map[string]string{
		{},
		{"GRID_ROLE": "cli"},
		{"GRID_ROLE": "owner"},
	} {
		h, path, text := r23Person(t, vars)
		name := vars["GRID_ROLE"]
		if code := h.run(h.home, "task", "status", "T-01", "waiting"); code != 0 {
			t.Fatalf("%q: task status in the same shell: exit %d %s", name, code, h.errb)
		}
		code := h.run(h.home, "approve", "T-01", "--plan-hash", plan.Hash(text), "--json")
		if code != 0 || strings.Contains(h.errb.String(), "GRID_MEMBER") {
			t.Fatalf("%q: approve exit %d, want 0; stderr %q", name, code, h.errb.String())
		}
		id, _, err := taskstore.FeedIdentity(path)
		if err != nil {
			t.Fatal(err)
		}
		snap, err := orchestrator.Load(feed.Store{TaskDir: path, TaskID: id})
		if err != nil {
			t.Fatal(err)
		}
		var answers int
		for _, r := range snap.Records {
			if r.Type != "answer" {
				continue
			}
			answers++
			if r.Author != (contract.Author{ID: "local", Role: "owner"}) {
				t.Fatalf("%q: the answer is by %+v, want local owner", name, r.Author)
			}
		}
		if answers != 1 {
			t.Fatalf("%q: %d answer records in the Feed, want 1", name, answers)
		}
		jobs, snap := approveJobs(t, feed.Store{TaskDir: path, TaskID: id})
		if len(jobs) != 1 || !personJobAnswered(snap, snap.State.Jobs[jobs[0].ID]) {
			t.Fatalf("%q: the approve did not answer the plan Job: %+v", name, jobs)
		}
	}
}

// An agent's environment stays refused with exit 3 and writes nothing, as before:
// only a person of the team approves, and with no GRID_MEMBER a worker mark or a
// role that is not a person's says the caller is not one.
func TestR23ApproveRefusesAgentEnvironment(t *testing.T) {
	for name, vars := range map[string]map[string]string{
		"worker id":    {"GRID_WORKER_ID": "w1"},
		"agent mark":   {"GRID_AGENT": "orchestrator"},
		"worker mark":  {"GRID_WORKER": "w@x"},
		"agent role":   {"GRID_ROLE": "worker"},
		"agent + role": {"GRID_AGENT": "1", "GRID_ROLE": "owner"},
	} {
		t.Run(name, func(t *testing.T) {
			h, path, text := r23Person(t, vars)
			if code := h.run(h.home, "approve", "T-01", "--plan-hash", plan.Hash(text), "--json"); code != 3 {
				t.Fatalf("exit %d, want 3; stdout %q stderr %q", code, h.out, h.errb)
			}
			if !strings.Contains(h.errb.String(), "approving is for a person of the team: set GRID_MEMBER") {
				t.Fatalf("stderr %q", h.errb)
			}
			id, _, err := taskstore.FeedIdentity(path)
			if err != nil {
				t.Fatal(err)
			}
			jobs, snap := approveJobs(t, feed.Store{TaskDir: path, TaskID: id})
			for _, r := range snap.Records {
				if r.Type == "answer" {
					t.Fatalf("the refused approve wrote an answer: %+v", r)
				}
			}
			if len(jobs) != 0 {
				t.Fatalf("the refused approve posted approve Jobs: %+v", jobs)
			}
		})
	}
}
