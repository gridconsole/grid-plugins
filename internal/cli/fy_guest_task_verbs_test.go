package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// fyGuestMoveRefusal is the line a guest's `grid task move` gets: the wording of
// a guest's new, close, reopen and stop (guestTaskMoveRefusal in cli.go).
const fyGuestMoveRefusal = `grid task move: role "guest" may not move a task; a guest answers and comments`

// fyMovableProject gives the fixture project a git folder with a remote, the one
// thing `grid task move` needs besides a task (movableProject in intent.go), so a
// move that is not refused enqueues an intent.
func fyMovableProject(t *testing.T, h *harness) {
	t.Helper()
	repo := filepath.Join(h.root, "repo")
	h.mkdir(repo)
	testgit.Init(t, repo, "https://example.com/org/repo.git", false)
	h.record(project.Record{Name: "example", ID: "01EXAMPLE", Folders: []project.Folder{{Machine: h.env.Machine, Path: repo}}, Repos: []project.Repo{{Path: "."}}})
}

// fyIntentFiles lists the intent files of a task folder; a task that never had an
// intent has no intents folder and lists nothing.
func fyIntentFiles(t *testing.T, fixture task.Task) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(fixture.Path, "intents"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		// The store's .lock is not an intent.
		if !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names
}

// A guest's `grid task move` is refused before any project is resolved, like its
// new, close, reopen and stop: the same exit and line for an invited guest, an
// uninvited guest and an unknown project, and no intent file is written.
func TestFyGuestTaskMoveRefusedEarly(t *testing.T) {
	for _, tc := range []struct {
		name, member string
		args         []string
	}{
		{"invited", "gus", []string{"task", "move", "laptop", "--task", "T-01", "--project", "example"}},
		{"invited_to_form", "gus", []string{"task", "move", "T-01", "--to", "laptop", "--project", "example"}},
		{"uninvited", "mallory", []string{"task", "move", "laptop", "--task", "T-01", "--project", "example"}},
		{"unknown_project", "gus", []string{"task", "move", "laptop", "--task", "T-01", "--project", "nosuch"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, fixture := guestTaskFixture(t, "guest", "", false)
			fyMovableProject(t, h)
			fvGuestEnv(h, "guest", tc.member, "")
			dir := fvGuestProjectDir(fixture)
			before := guestSnapshot(t, dir)
			intentsBefore := fyIntentFiles(t, fixture)
			code := h.run(h.home, tc.args...)
			if code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, h.errb)
			}
			if got := strings.TrimSpace(h.errb.String()); got != fyGuestMoveRefusal {
				t.Fatalf("stderr %q, want %q", got, fyGuestMoveRefusal)
			}
			if h.out.Len() != 0 {
				t.Fatalf("stdout %q, want none", h.out)
			}
			if after := fyIntentFiles(t, fixture); !slices.Equal(after, intentsBefore) {
				t.Fatalf("intent files before %v, after %v", intentsBefore, after)
			}
			if guestSnapshot(t, dir) != before {
				t.Fatal("the refused move changed the project folder")
			}
		})
	}
}

// A guest session that Grid itself runs (GRID_LEASE=task) is not a person's guest
// and is not refused early, as for new: its move enqueues one intent.
func TestFyGuestTaskMoveWithTaskLease(t *testing.T) {
	h, fixture := guestTaskFixture(t, "guest", "task", false)
	fyMovableProject(t, h)
	code := h.run(h.home, "task", "move", "laptop", "--task", "T-01", "--project", "example")
	if code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, h.errb)
	}
	if got := fyIntentFiles(t, fixture); len(got) != 1 {
		t.Fatalf("intent files %v, want one", got)
	}
}

// Rename, decline and move by guest, member and owner, as the invited person of
// the fixture. A guest is refused by all three, with the text main gave rename
// and decline after resolution and the early text for move; a member and an
// owner still change the task.
func TestFyGuestTaskVerbsByRole(t *testing.T) {
	const newTitle = "A title set by the test"
	const reason = "a reason set by the test"
	verbs := []struct {
		verb      string
		args      []string
		guestLine string
		// changed says whether the task folder holds what an allowed role wrote.
		changed func(t *testing.T, fixture task.Task) bool
	}{
		{"rename", []string{"task", "rename", "T-01", "--title", newTitle, "--project", "example"},
			"grid task rename: scope: role cannot rename tasks",
			func(t *testing.T, fixture task.Task) bool {
				return strings.Contains(readTaskTestFile(t, filepath.Join(fixture.Path, "task.md")), newTitle)
			}},
		// A decline records its reason in the task folder's history, not task.md.
		{"decline", []string{"task", "decline", "T-01", "--reason", reason, "--project", "example"},
			"grid task decline: scope: role cannot decline tasks",
			func(t *testing.T, fixture task.Task) bool {
				return strings.Contains(guestSnapshot(t, fixture.Path), reason)
			}},
		{"move", []string{"task", "move", "laptop", "--task", "T-01", "--project", "example"},
			fyGuestMoveRefusal,
			func(t *testing.T, fixture task.Task) bool { return len(fyIntentFiles(t, fixture)) == 1 }},
	}
	for _, v := range verbs {
		for _, role := range []string{"guest", "member", "owner"} {
			t.Run(role+"/"+v.verb, func(t *testing.T) {
				h, fixture := guestTaskFixture(t, role, "", false)
				fyMovableProject(t, h)
				dir := fvGuestProjectDir(fixture)
				before := guestSnapshot(t, dir)
				code := h.run(h.home, v.args...)
				if role != "guest" {
					if code != 0 {
						t.Fatalf("%s task %s: exit %d, want 0: %s", role, v.verb, code, h.errb)
					}
					if !v.changed(t, fixture) {
						t.Fatalf("%s task %s: exit 0 but the task does not hold the change", role, v.verb)
					}
					return
				}
				if code != 1 {
					t.Fatalf("guest task %s: exit %d, want 1: %s", v.verb, code, h.errb)
				}
				if got := fvGuestFirstLine(h.errb.String()); got != v.guestLine {
					t.Fatalf("guest task %s: first stderr line %q, want %q", v.verb, got, v.guestLine)
				}
				if v.changed(t, fixture) || guestSnapshot(t, dir) != before {
					t.Fatalf("guest task %s changed the project folder", v.verb)
				}
			})
		}
	}
}

// guestWriteVerb is the pre-resolution guard's list of task verbs a guest may not
// run. The walk over every `task` verb of the contract is TestFvGuestTaskVerbs;
// this one pins which of them are refused before resolution.
func TestFyGuestWriteVerbEarlyList(t *testing.T) {
	for _, verb := range []string{"close", "delete", "move", "new", "reopen", "restore", "stop"} {
		if !guestWriteVerb("task", []string{verb}) {
			t.Errorf("guestWriteVerb(task, %s) is false: a guest's %s is not refused before resolution", verb, verb)
		}
	}
	for _, verb := range []string{"show", "list", "history"} {
		if guestWriteVerb("task", []string{verb}) {
			t.Errorf("guestWriteVerb(task, %s) is true: a read is refused before resolution", verb)
		}
	}
}
