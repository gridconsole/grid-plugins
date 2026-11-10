package guard

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const namedInline = "workflow: [{step: plan, kind: approve}, {step: preview, kind: approve}, {step: build, kind: run}]"

// inline swaps the named workflow for the same steps as an inline list.
func (n *namedTask) inline(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "workflow: ") {
			lines[i] = namedInline
		}
	}
	return strings.Join(lines, "\n")
}

func (n *namedTask) stage(text string) string {
	return planStageWith(stepsFor(n.f.env, n.dir), text, "")
}

func (n *namedTask) held() bool {
	return held(stepsFor(n.f.env, n.dir), n.dir, read(filepath.Join(n.dir, "task.md")), read(filepath.Join(n.dir, "history.md")))
}

// The guard's stage and hold, and the task package's own answer with the
// store's resolver, on the rows TestNamedWorkflowHoldGuardAndDeliver drives.
func TestNamedWorkflowPlanStage(t *testing.T) {
	for i, row := range namedRows {
		t.Run(row.name, func(t *testing.T) {
			n := newNamedRow(t, i)
			text := read(filepath.Join(n.dir, "task.md"))
			if got := n.stage(text); got != row.stage {
				t.Errorf("stage %q, want %q", got, row.stage)
			}
			if got := n.held(); got != row.held {
				t.Errorf("guard held %v, want %v", got, row.held)
			}
			if got := taskstore.PlanHeldWith(taskstore.StepsFor(n.store.Resolver, n.dir), n.dir, text, ""); got != row.held {
				t.Errorf("task held %v, want %v", got, row.held)
			}
		})
	}
}

// The same steps as an inline list read the same stage as the named template.
func TestNamedWorkflowMatchesInline(t *testing.T) {
	for _, step := range []string{"plan", "build"} {
		n := newNamed(t, "large", "Large", "", namedWorkflows(namedLarge))
		named := n.at("waiting", step)
		if got, want := n.stage(named), n.stage(n.inline(named)); got != want || got == "unreadable" {
			t.Errorf("step %s: named %q, inline %q", step, got, want)
		}
	}
}

// The guard decides a task.md edit on the text it would hold, so the steps
// come from the proposed workflow, not the one on disk.
func TestNamedWorkflowProposedEdit(t *testing.T) {
	for _, tc := range []struct {
		name, from, to string
		stage, want    string
	}{
		{"names a template with no plan gate", "workflow: Large", "workflow: Quick", "", "deny"},
		{"keeps the template", "title: Named steps", "title: Named steps again", "gate", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := newNamed(t, "large", "Large", "", namedWorkflows(namedLarge, namedQuick))
			text := n.at("waiting", "plan")
			n.put(text)
			next := strings.Replace(text, tc.from, tc.to, 1)
			if next == text {
				t.Fatalf("%q not in task.md", tc.from)
			}
			if got := n.stage(next); got != tc.stage {
				t.Fatalf("stage of the proposed text %q, want %q", got, tc.stage)
			}
			i := n.f.input("Write", filepath.Join(n.dir, "task.md"))
			i.Cwd = n.dir
			i.ToolInput["content"] = next
			d := EvaluatePlan(i, n.f.root, n.f.env)
			expect(t, d, tc.want)
			checkAdapter(t, PlanGuard, i, n.f.env, d)
		})
	}
}

// namedRole is a caller of the named-workflow tests; filtered is a guest the
// project's invitations do not cover, whom the resolver cannot read it for.
type namedRole struct {
	name, role, member string
	invited, filtered  bool
}

var namedRoles = []namedRole{
	{name: "owner", role: "owner"},
	{name: "member", role: "member"},
	{name: "guest", role: "guest", member: "gus", invited: true},
	{name: "signed out"},
	{name: "guest filtered", role: "guest", member: "gus", filtered: true},
}

// as makes the guard's environment and the store's resolver the role's.
func (n *namedTask) as(tc namedRole) {
	delete(n.f.env, "GRID_ROLE")
	delete(n.f.env, "GRID_MEMBER")
	if tc.role != "" {
		n.f.env["GRID_ROLE"] = tc.role
	}
	if tc.member != "" {
		n.f.env["GRID_MEMBER"] = tc.member
	}
	if tc.invited {
		n.f.write(filepath.Join(n.f.projects, "alpha", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	}
	// The caller is read once, so the store takes the role as the hook does.
	n.store.Resolver.Caller = where.CallerFrom(func(k string) string { return n.f.env[k] })
}

// Every caller is held at the plan gate of a named workflow exactly as at an
// inline one; a guest the resolver does not let read the project stays held.
func TestNamedWorkflowRoles(t *testing.T) {
	for _, tc := range namedRoles {
		t.Run(tc.name, func(t *testing.T) {
			n := newNamed(t, "large", "Large", "", namedWorkflows(namedLarge))
			n.as(tc)
			n.put(n.inline(n.at("waiting", "plan")))
			inline := n.repoWrite()
			expect(t, inline, "deny")
			n.put(n.at("waiting", "plan"))
			if got := n.repoWrite(); got.Behavior != inline.Behavior || got.Reason != inline.Reason {
				t.Fatalf("named at plan %q %q, inline %q %q", got.Behavior, got.Reason, inline.Behavior, inline.Reason)
			}
			stage, want := "past", ""
			if tc.filtered {
				stage, want = "unreadable", "deny"
			}
			if got := n.stage(n.at("waiting", "build")); got != stage {
				t.Fatalf("stage at build %q, want %q", got, stage)
			}
			n.put(n.at("waiting", "build"))
			expect(t, n.repoWrite(), want)
		})
	}
}

// Deliver no longer asks the plan hold (S6c3), so it delivers at the plan gate
// as well as past it; the guard still denies a repo write at the plan gate.
// A guest the resolver filters out is refused by Deliver at the task lookup and
// is denied by the guard at both steps. A refused Deliver changes nothing.
func TestNamedWorkflowDeliverRoles(t *testing.T) {
	for _, tc := range namedRoles {
		for _, step := range []string{"plan", "build"} {
			t.Run(tc.name+"/"+step, func(t *testing.T) {
				n := newNamed(t, "large", "Large", "", namedWorkflows(namedLarge))
				n.as(tc)
				n.deliverable(n.at("running", step), "")
				before := files(t, n.dir)
				_, err := n.store.Deliver(taskstore.DeliverOptions{Cwd: n.dir, Task: n.id, Outcome: "Ready", By: "grid"})
				switch {
				case tc.filtered:
					if !errors.Is(err, where.ErrTaskNotFound) {
						t.Errorf("Deliver err=%v; want the task lookup to refuse a filtered guest", err)
					}
					if !reflect.DeepEqual(files(t, n.dir), before) {
						t.Error("a refused Deliver changed the task")
					}
				case err != nil:
					t.Errorf("Deliver: %v; Deliver no longer asks the plan hold", err)
				}
			})
		}
	}
}

// The premise of the test above: for a filtered guest only the task lookup
// stands between Deliver and a different hold, since the plan hold itself
// reads the task unreadable with the caller and past its gate without.
func TestNamedWorkflowFilteredGuestHoldLookup(t *testing.T) {
	n := newNamed(t, "large", "Large", "", namedWorkflows(namedLarge))
	n.as(namedRoles[len(namedRoles)-1])
	text := n.at("running", "build")
	if got := taskstore.PlanStageWith(taskstore.StepsFor(n.store.Resolver, n.dir), text, ""); got != "unreadable" {
		t.Fatalf("filtered guest: stage %q, want unreadable", got)
	}
	open := n.store.Resolver
	open.Caller = where.Caller{}
	if got := taskstore.PlanStageWith(taskstore.StepsFor(open, n.dir), text, ""); got != "past" {
		t.Fatalf("no caller: stage %q, want past", got)
	}
}
