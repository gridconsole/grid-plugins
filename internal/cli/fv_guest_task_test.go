package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// fvGuestNewRefusal is the line a guest's `grid task new` gets: the wording of a
// guest's close, reopen and stop (guestTaskMoveRefusal in cli.go).
const fvGuestNewRefusal = `grid task new: role "guest" may not new a task; a guest answers and comments`

// fvGuestEnv sets the caller's environment on h. guestTaskFixture sets the same
// keys with the member fixed to "gus"; this one also names an uninvited member.
func fvGuestEnv(h *harness, role, member, lease string) {
	h.env.Getenv = func(k string) string {
		switch k {
		case "GRID_ROLE":
			return role
		case "GRID_MEMBER":
			return member
		case "GRID_LEASE":
			return lease
		case "GRID_ACTOR":
			if role != "guest" {
				return "alex"
			}
		}
		return ""
	}
}

// fvGuestProjectDir is the project folder of the fixture task: the folder that
// holds tasks/ and project-roles.json.
func fvGuestProjectDir(fixture task.Task) string {
	return filepath.Dir(filepath.Dir(fixture.Path))
}

// fvGuestFirstLine is the first line of the stderr text s.
func fvGuestFirstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// A guest's `grid task new` is refused before any project is resolved, like its
// close, reopen and stop: the same exit and line for an invited guest, an
// uninvited guest and an unknown project, and the project folder is untouched.
func TestFvGuestTaskNewRefusedEarly(t *testing.T) {
	for _, tc := range []struct {
		name, member string
		args         []string
	}{
		{"invited", "gus", []string{"task", "new", "a guest task", "--project", "example"}},
		{"uninvited", "mallory", []string{"task", "new", "a guest task", "--project", "example"}},
		{"unknown_project", "gus", []string{"task", "new", "a guest task", "--project", "nosuch"}},
		{"no_project_flag", "gus", []string{"task", "new", "a guest task"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, fixture := guestTaskFixture(t, "guest", "", false)
			fvGuestEnv(h, "guest", tc.member, "")
			dir := fvGuestProjectDir(fixture)
			before := guestSnapshot(t, dir)
			code := h.run(h.home, tc.args...)
			if code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, h.errb)
			}
			if got := strings.TrimSpace(h.errb.String()); got != fvGuestNewRefusal {
				t.Fatalf("stderr %q, want %q", got, fvGuestNewRefusal)
			}
			if h.out.Len() != 0 {
				t.Fatalf("stdout %q, want none", h.out)
			}
			if guestSnapshot(t, dir) != before {
				t.Fatal("the refused write changed the project folder")
			}
		})
	}
}

// A member, an owner, an empty role and a Grid-run session (GRID_LEASE=task, here
// with the guest role) still create tasks.
func TestFvGuestTaskNewStillCreates(t *testing.T) {
	for _, tc := range []struct{ name, role, lease string }{
		{"member", "member", ""},
		{"owner", "owner", ""},
		{"empty_role", "", ""},
		{"guest_with_task_lease", "guest", "task"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, fixture := guestTaskFixture(t, tc.role, tc.lease, false)
			dir := fvGuestProjectDir(fixture)
			before := guestSnapshot(t, dir)
			code := h.run(h.home, "task", "new", "another task", "--project", "example", "--json")
			if code != 0 {
				t.Fatalf("exit %d, want 0: %s", code, h.errb)
			}
			var made struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &made); err != nil {
				t.Fatalf("stdout %q: %v", h.out, err)
			}
			if made.ID == "" || made.ID == fixture.ID {
				t.Fatalf("new task id %q, fixture %q", made.ID, fixture.ID)
			}
			if info, err := os.Stat(made.Path); err != nil || !info.IsDir() {
				t.Fatalf("the task folder %q does not exist: %v", made.Path, err)
			}
			if guestSnapshot(t, dir) == before {
				t.Fatal("exit 0 but the project folder is unchanged")
			}
		})
	}
}

// `grid approve` by a guest stops in approvalWriter (approve_record.go): no
// approval and no accepting control is written. A member on the same fixture
// does not get that line.
func TestFvGuestApproveRefused(t *testing.T) {
	const line = "approving is for a person of the team: set GRID_MEMBER"
	t.Run("guest", func(t *testing.T) {
		h, fixture := guestTaskFixture(t, "guest", "", false)
		dir := fvGuestProjectDir(fixture)
		before := guestSnapshot(t, dir)
		code := h.run(h.home, "approve", fixture.ID)
		if code == 0 {
			t.Fatalf("guest approve exit 0: %s", h.out)
		}
		if !strings.Contains(h.errb.String(), line) {
			t.Fatalf("guest approve: exit %d, stderr %q, want %q", code, h.errb, line)
		}
		if guestSnapshot(t, dir) != before {
			t.Fatal("a guest's approve changed the project folder")
		}
	})
	t.Run("member", func(t *testing.T) {
		h, fixture := guestTaskFixture(t, "member", "", false)
		code := h.run(h.home, "approve", fixture.ID)
		if strings.Contains(h.errb.String(), line) {
			t.Fatalf("member approve: exit %d, stderr %q names the guest refusal", code, h.errb)
		}
	})
}

// fvGuestVerbs is one row per `grid task` write verb, run as the invited guest
// of guestTaskFixture (a queued task T-01, a project with no git remote, no
// GRID_TASK). A row says the exit code, the first stderr line and whether the
// project folder changed. The rows for new to move are role refusals (the guard
// in cli.go answers move before the project is looked at, so its missing remote
// does not show); attach is refused here for a reason that is not the role (no
// assigned task folder), and status is a legal queued to running move that a
// guest makes: TestFvGuestTaskVerbsWhenValid shows attach with valid input and
// move with a remote.
var fvGuestVerbs = []struct {
	verb    string
	args    []string
	exit    int
	line    string
	changed bool
}{
	{"new", []string{"task", "new", "a guest task", "--project", "example"}, 1, fvGuestNewRefusal, false},
	{"stop", []string{"task", "stop", "T-01"}, 1, `grid task stop: role "guest" may not stop a task; a guest answers and comments`, false},
	{"close", []string{"task", "close", "T-01"}, 1, `grid task close: role "guest" may not close a task; a guest answers and comments`, false},
	{"reopen", []string{"task", "reopen", "T-01"}, 1, `grid task reopen: role "guest" may not reopen a task; a guest answers and comments`, false},
	{"delete", []string{"task", "delete", "T-01", "--project", "example"}, 3, "grid task delete: task edit rejected: a person with an allowed role must perform this task move", false},
	{"restore", []string{"task", "restore", "T-01", "--project", "example"}, 3, "grid task restore: task edit rejected: a person with an allowed role must perform this task move", false},
	{"rename", []string{"task", "rename", "T-01", "--title", "A guest title", "--project", "example"}, 1, "grid task rename: scope: role cannot rename tasks", false},
	{"decline", []string{"task", "decline", "T-01", "--reason", "a guest reason", "--project", "example"}, 1, "grid task decline: scope: role cannot decline tasks", false},
	{"summary", []string{"task", "summary", "T-01", "--for", "customer", "--text", "a guest summary"}, 1, "grid task summary: scope: role cannot write task summaries", false},
	{"move", []string{"task", "move", "laptop", "--task", "T-01", "--project", "example"}, 1, `grid task move: role "guest" may not move a task; a guest answers and comments`, false},
	{"lane", []string{"task", "lane", "T-01", "doing", "--project", "example"}, 1, `grid task lane: role "guest" may not move a task to a lane; a guest answers and comments`, false},
	{"merge", []string{"task", "merge", "--task", "T-01", "--project", "example"}, 1, "grid task merge: task T-01 has no branch in task.md; run grid deliver in the task's checkout to record it, then run grid task merge again", false},
	{"attach", []string{"task", "attach", "T-01"}, 1, "grid task attach: agent cannot attach T-01 without its assigned task folder", false},
	{"status", []string{"task", "status", "T-01", "running"}, 0, "", true},
}

// TestFvGuestTaskVerbsWhenValid runs the verbs the table above does not settle
// with the input that gets past their non-role checks: a task bound to the
// session (GRID_TASK) for attach, a project with a git remote for move. A row
// that changes the project folder is a guest write the CLI layer does not refuse;
// it is asserted as seen here and is a follow-up, not fixed in this lane. Move was
// such a row until the guard refused it (C-fy-plugin-guest-task-move); it now
// shows the refusal with input that would otherwise enqueue an intent.
func TestFvGuestTaskVerbsWhenValid(t *testing.T) {
	for _, row := range []struct {
		name    string
		args    []string
		bound   bool // GRID_TASK names the fixture task
		remote  bool // the project folder is a git repo with a remote
		exit    int
		line    string
		changed bool
	}{
		{"attach_bound_task", []string{"task", "attach", "T-01"}, true, false, 0, "", true},
		{"move_with_remote", []string{"task", "move", "laptop", "--task", "T-01", "--project", "example"}, false, true, 1, `grid task move: role "guest" may not move a task; a guest answers and comments`, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			h, fixture := guestTaskFixture(t, "guest", "", false)
			if row.remote {
				repo := filepath.Join(h.root, "repo")
				h.mkdir(repo)
				testgit.Init(t, repo, "https://example.com/org/repo.git", false)
				h.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: h.env.Machine, Path: repo}}, Repos: []project.Repo{{Path: "."}}})
			}
			if row.bound {
				// attach reads the machine identity, which must be a ULID.
				h.write(filepath.Join(h.gridHome, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAV\n")
				env := h.env.Getenv
				h.env.Getenv = func(k string) string {
					if k == "GRID_TASK" {
						return fixture.ID
					}
					return env(k)
				}
			}
			dir := fvGuestProjectDir(fixture)
			before := guestSnapshot(t, dir)
			code := h.run(h.home, row.args...)
			changed := guestSnapshot(t, dir) != before
			line := fvGuestFirstLine(h.errb.String())
			if code != row.exit || line != row.line || changed != row.changed {
				t.Errorf("guest %s: exit %d, first stderr line %q, changed %v; table has exit %d, %q, changed %v\nstdout: %s",
					row.name, code, line, changed, row.exit, row.line, row.changed, h.out)
			}
		})
	}
}

// fvGuestReadVerbs are the `grid task` verbs that only read.
var fvGuestReadVerbs = []string{"show", "list", "history"}

func TestFvGuestTaskVerbs(t *testing.T) {
	for _, row := range fvGuestVerbs {
		t.Run(row.verb, func(t *testing.T) {
			h, fixture := guestTaskFixture(t, "guest", "", false)
			if fixture.ID != "T-01" {
				t.Fatalf("fixture task is %q, rows name T-01", fixture.ID)
			}
			dir := fvGuestProjectDir(fixture)
			before := guestSnapshot(t, dir)
			code := h.run(h.home, row.args...)
			changed := guestSnapshot(t, dir) != before
			line := fvGuestFirstLine(h.errb.String())
			if code != row.exit || line != row.line || changed != row.changed {
				t.Errorf("guest task %s: exit %d, first stderr line %q, changed %v; table has exit %d, %q, changed %v\nstdout: %s",
					row.verb, code, line, changed, row.exit, row.line, row.changed, h.out)
			}
		})
	}
	// A new `task` verb in the contract must get a row here, or be a read.
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inv struct {
		Commands []struct {
			Name        string
			Subcommands []struct{ Name string }
		}
	}
	if err := json.Unmarshal(data, &inv); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, row := range fvGuestVerbs {
		known[row.verb] = true
	}
	for _, v := range fvGuestReadVerbs {
		known[v] = true
	}
	seen := map[string]bool{}
	for _, c := range inv.Commands {
		if c.Name != "task" {
			continue
		}
		for _, s := range c.Subcommands {
			seen[s.Name] = true
			if !known[s.Name] {
				t.Errorf("grid task %s is in contract/commands.json but in neither fvGuestVerbs nor fvGuestReadVerbs", s.Name)
			}
		}
	}
	for v := range known {
		if !seen[v] {
			t.Errorf("grid task %s is in this test but not in contract/commands.json", v)
		}
	}
}
