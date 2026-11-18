package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// approveJobs lists the person Jobs of kind approve on the task's Feed: the Jobs
// grid approve and the app answer.
func approveJobs(t *testing.T, store feed.Store) ([]job.Job, orchestrator.Snapshot) {
	t.Helper()
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	var out []job.Job
	for _, it := range snap.Items {
		if it.IsPerson() && it.Kind == "approve" {
			out = append(out, it)
		}
	}
	return out, snap
}

// openApproveJob is the one approve person Job of the Feed that is not accepted,
// cancelled or rejected; the test fails when there is not exactly one.
func openApproveJob(t *testing.T, store feed.Store) (job.Job, orchestrator.Snapshot) {
	t.Helper()
	jobs, snap := approveJobs(t, store)
	var open []job.Job
	for _, j := range jobs {
		switch snap.State.Jobs[j.ID].Status {
		case contract.Accepted, contract.Cancelled, contract.Rejected:
		default:
			open = append(open, j)
		}
	}
	if len(open) != 1 {
		t.Fatalf("want one open approve person Job, got %d of %d approve Jobs", len(open), len(jobs))
	}
	return open[0], snap
}

// A delivery posts one person Job for the ship step: the plan approve Job, open
// and ready, for the task's person; a task with no owner asks the owner.
func TestDeliverPostsShipPersonJob(t *testing.T) {
	for _, tc := range []struct {
		name, owner, want string
	}{
		{name: "task owner", owner: "sam", want: "sam"},
		{name: "no owner means the owner", owner: "", want: "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, dir, id, store := keeperHarness(t)
			setTaskOwner(t, dir, tc.owner)
			if code := h.deliverRun(dir, "--outcome", "Ready", "--task", id, "--json"); code != 0 {
				t.Fatalf("deliver: %d %s", code, h.errb)
			}
			var out struct {
				PersonJob string `json:"person_job"`
			}
			if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
				t.Fatalf("%v: %s", err, h.out)
			}
			j, snap := openApproveJob(t, store)
			if out.PersonJob != j.ID {
				t.Fatalf("deliver --json person_job = %q, want %q", out.PersonJob, j.ID)
			}
			if j.Kind != "approve" || !j.IsPerson() || j.PersonName() != tc.want || j.Continues != nil || len(j.After) != 0 {
				t.Fatalf("person Job fields: kind %q role %q person %q continues %v after %v", j.Kind, j.Role, j.PersonName(), j.Continues, j.After)
			}
			// The ship step has its own title (TestShipJobTitleNamesTheDeliveredWork); the rest is the plan Job's.
			if j.Title != "Approve the delivered work" || j.Intent.Item != "plan.md#approve" || len(j.Acceptance) != 1 || j.Acceptance[0].Text != "The delivered work is approved to land" {
				t.Fatalf("not the plan approve Job: title %q item %q acceptance %+v", j.Title, j.Intent.Item, j.Acceptance)
			}
			if err := orchestrator.PersonReady(snap, j.ID, time.Now()); err != nil {
				t.Fatalf("the person Job is not ready to answer: %v", err)
			}
			if personJobAnswered(snap, snap.State.Jobs[j.ID]) {
				t.Fatal("deliver answered its own person Job")
			}
		})
	}
}

// setTaskOwner sets the owner line of task.md, or removes it for "".
func setTaskOwner(t *testing.T, dir, owner string) {
	t.Helper()
	path := filepath.Join(dir, "task.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	var out []string
	for i, line := range lines {
		if strings.HasPrefix(line, "owner:") {
			continue
		}
		out = append(out, line)
		if i == 0 && line == "---" && owner != "" {
			out = append(out, "owner: "+owner)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(out, "\n")), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, got, err := task.FeedIdentity(dir); err != nil || got != owner {
		t.Fatalf("task owner = %q, %v; want %q", got, err, owner)
	}
}

// The built-in flow shows the person step deliver posts, after Check: an approve
// step whose empty approvers list is the task's person.
func TestBuiltInFlowShowsShipPersonStep(t *testing.T) {
	steps := nhWfBuiltIn("T-1").Steps
	if len(steps) != 3 || steps[1].Name != "Check" {
		t.Fatalf("built-in steps %+v, want Run, Check, then the person step", steps)
	}
	s := steps[2]
	if s.Name != "Ship" || s.Kind != "approve" || s.Agent != nil || s.Approvers == nil || len(s.Approvers) != 0 {
		t.Fatalf("person step %+v, want Ship (approve) with an empty approvers list", s)
	}
	if text := nhWfText(nhWfBuiltIn("T-1")); !strings.HasSuffix(text, "step: Check (check, grid-reviewer)\nstep: Ship (approve)\n") {
		t.Fatalf("text answer:\n%s", text)
	}
}

// A second delivery of a task whose person Job is still open posts no second one.
func TestRedeliverPostsNoSecondShipPersonJob(t *testing.T) {
	h, dir, id, store := keeperHarness(t)
	if code := h.deliverRun(dir, "--outcome", "Ready", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	first, _ := openApproveJob(t, store)
	if code := h.run(h.home, "task", "status", id, "running"); code != 0 {
		t.Fatalf("task status running: %s", h.errb)
	}
	if code := h.deliverRun(dir, "--outcome", "Ready again", "--task", id, "--json"); code != 0 {
		t.Fatalf("second deliver: %d %s", code, h.errb)
	}
	if again, _ := openApproveJob(t, store); again.ID != first.ID {
		t.Fatalf("open person Job %s after redelivery, want %s", again.ID, first.ID)
	}
	if jobs, _ := approveJobs(t, store); len(jobs) != 1 {
		t.Fatalf("want one approve person Job after two deliveries, got %d", len(jobs))
	}
	if strings.Contains(h.out.String(), `"person_job":"J-`) {
		t.Fatalf("the second delivery names a new person Job: %s", h.out)
	}
}

// grid approve on a delivered task answers the person Job the delivery posted and
// posts no second plan-revision Job: leased or as a person of the team with no
// lease, and whether or not the plan Job was accepted before the delivery.
func TestApproveAnswersShipPersonJob(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		leased, planAccepted bool
	}{
		{name: "leased", leased: true},
		{name: "person without lease", leased: false},
		{name: "leased, plan Job accepted before delivery", leased: true, planAccepted: true},
		{name: "person without lease, plan Job accepted before delivery", leased: false, planAccepted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, dir, id, store := keeperHarness(t)
			before := 0
			if tc.planAccepted {
				w, err := approvalWriter(h.env)
				if err != nil {
					t.Fatal(err)
				}
				if _, why, err := migrate.ReceiptToJob(dir, w); err != nil {
					t.Fatalf("plan Job before delivery: %v %s", err, why)
				}
				if !task.Approved(dir) {
					t.Fatal("the plan Job accepted before delivery must approve the plan")
				}
				before = 1
			}
			if code := h.deliverRun(dir, "--outcome", "Ready", "--task", id); code != 0 {
				t.Fatalf("deliver: %d %s", code, h.errb)
			}
			ship, _ := openApproveJob(t, store)
			if !tc.leased {
				// A person of the team in a terminal: no lease epoch, the home's machine id.
				h.write(filepath.Join(h.gridHome, "machine"), feedMachine+"\n")
				t.Setenv("GRID_LEASE_EPOCH", "")
				t.Setenv("GRID_MEMBER", "sam")
			}
			h.out.Reset()
			h.errb.Reset()
			if code := h.run(h.home, "approve", id, "--json"); code != 0 {
				t.Fatalf("approve: %d %s", code, h.errb)
			}
			jobs, snap := approveJobs(t, store)
			if len(jobs) != before+1 {
				t.Fatalf("approve posted another approve Job: %d approve Jobs, want %d", len(jobs), before+1)
			}
			j := snap.State.Jobs[ship.ID]
			if tc.leased {
				if j.Status != contract.Accepted {
					t.Fatalf("person Job %s status %q, want accepted", ship.ID, j.Status)
				}
				if a, err := task.ApprovalFromJob(dir); err != nil || a.Job != ship.ID {
					t.Fatalf("plan approval %+v, %v; want the result of %s", a, err, ship.ID)
				}
			} else if !personJobAnswered(snap, j) {
				t.Fatalf("the person's approve did not answer %s", ship.ID)
			}
			// A person's answer counts once the lease holder's next pass accepts it
			// (holderPass); an earlier accepted plan Job keeps counting meanwhile.
			if (tc.leased || tc.planAccepted) && !task.Approved(dir) {
				t.Fatal("the plan approval must count")
			}
		})
	}
}
