package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// holderPass is the next pass of the task-lease holder: it takes the task lease
// and ticks once. A person with no lease epoch answers the approve Job with an
// answer, and this pass turns it into the accepting control, so the approval
// takes effect here, as a worker's tick does in production.
func holderPass(t *testing.T, dir string) {
	t.Helper()
	w, store := approvefix.Lease(t, dir)
	s := orchestrator.Store{Feed: store, Writer: w, Assignee: w.Worker}
	if _, err := s.Tick(); err != nil {
		t.Fatal(err)
	}
}

// leased makes the harness a person who holds the task lease of dir (as the lease
// holder of approvefix), so grid approve posts the accepting result at once, in
// one write, instead of leaving an answer for the holder's next pass. A task
// whose plan.md is not in the Job format cannot be ticked, so its tests use this.
func (h *harness) leased(dir string) { leasedEnv(h.t, &h.env, dir) }

// leasedEnv is leased for a test that builds its own Env.
func leasedEnv(t *testing.T, env *Env, dir string) {
	t.Helper()
	w, _ := approvefix.Lease(t, dir)
	epoch := fmt.Sprintf("%d.%d", w.Gen, w.Fence)
	env.Machine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
	env.Getenv = func(key string) string {
		switch key {
		case "GRID_MEMBER":
			return "dana"
		case "GRID_ROLE":
			return "owner"
		case "GRID_LEASE_EPOCH":
			return epoch
		}
		return ""
	}
}

// taskHeader gives a task folder the task.md a Feed identity needs, as a task
// made by grid has.
func taskHeader(t *testing.T, dir string) {
	t.Helper()
	md := "---\nid: T-07\nulid: " + feedTaskID + "\ntitle: Example\nproject: example\nstate: waiting\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
}

// memberShell is Env.Getenv for a person at a terminal who approves: the member
// id of the team and nothing else. grid approve records the approval as a Feed
// write, so the caller must be a person of the team (GRID_MEMBER) or hold a lease.
func memberShell(key string) string {
	if key == "GRID_MEMBER" {
		return "sam"
	}
	return ""
}

// person makes the harness a person at a terminal who approves: memberShell, and
// a machine id the Feed accepts (the harness machine, 01LAPTOP, is not a ULID).
func (h *harness) person() {
	h.t.Helper()
	h.env.Getenv = memberShell
	h.write(filepath.Join(h.gridHome, "machine"), feedMachine+"\n")
}
