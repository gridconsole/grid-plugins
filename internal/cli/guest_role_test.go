package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// guestSnapshot is every file under dir, by relative path and content, so a
// refused write is shown to leave the whole folder as it was.
func guestSnapshot(t *testing.T, dir string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		b.WriteString("== " + rel + "\n")
		if !d.IsDir() {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			b.Write(data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

// guestTaskFixture is one task in a shared project that invites the guest "gus",
// with the environment of the given role. A guest that is not invited cannot see
// the task at all, so only an invited guest shows that a write is refused.
func guestTaskFixture(t *testing.T, role, lease string, closed bool) (*harness, task.Task) {
	t.Helper()
	h := newHarness(t)
	h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	fixture := taskFixtureProject(t, h, "example")
	h.write(filepath.Join(filepath.Dir(filepath.Dir(fixture.Path)), "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	if closed {
		if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "close"}); err != nil {
			t.Fatal(err)
		}
	}
	h.env.Getenv = func(k string) string {
		switch k {
		case "GRID_ROLE":
			return role
		case "GRID_MEMBER":
			return "gus"
		case "GRID_ACTOR":
			if role != "guest" {
				return "alex"
			}
		case "GRID_LEASE":
			return lease
		}
		return ""
	}
	return h, fixture
}

// The owner's rule is guest = answer and comment only. A guest may not close,
// reopen or stop a task; a member and an owner still may.
func TestGuestTaskMoveRoles(t *testing.T) {
	for _, role := range []string{"guest", "member", "owner"} {
		for _, verb := range []string{"close", "reopen", "stop"} {
			t.Run(role+"/"+verb, func(t *testing.T) {
				h, fixture := guestTaskFixture(t, role, "", verb == "reopen")
				before := guestSnapshot(t, fixture.Path)
				code := h.run(h.home, "task", verb, fixture.ID)
				after := guestSnapshot(t, fixture.Path)
				if role != "guest" {
					if code != 0 {
						t.Fatalf("%s task %s: exit %d, want 0: %s", role, verb, code, h.errb)
					}
					if after == before {
						t.Fatalf("%s task %s: exit 0 but the task folder is unchanged", role, verb)
					}
					return
				}
				if code != 1 {
					t.Fatalf("guest task %s: exit %d, want 1: %s", verb, code, h.errb)
				}
				if !strings.Contains(h.errb.String(), "guest") {
					t.Fatalf("guest task %s: refusal does not name the guest role: %q", verb, h.errb.String())
				}
				if after != before {
					t.Fatalf("guest task %s changed the task folder", verb)
				}
			})
		}
	}
}

// A guest's refusal is the same for a task it cannot see, so it does not tell
// an invited guest from an uninvited one.
func TestGuestTaskMoveRefusalIsEarly(t *testing.T) {
	for _, verb := range []string{"close", "reopen", "stop"} {
		t.Run(verb, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(k string) string {
				if k == "GRID_ROLE" {
					return "guest"
				}
				return ""
			}
			if code := h.run(h.home, "task", verb, "T-999"); code != 1 || !strings.Contains(h.errb.String(), "guest") {
				t.Fatalf("exit %d: %q", code, h.errb.String())
			}
		})
	}
}

// A guest posts kind message only; the other five kinds of a post are a person's
// moves on the task. A member and an owner may post all six.
func TestGuestFeedPostKinds(t *testing.T) {
	for _, role := range []string{"guest", "member", "owner"} {
		for _, kind := range []string{"message", "close", "reopen", "pickup", "ask-pickup", "ship"} {
			t.Run(role+"/"+kind, func(t *testing.T) {
				h, taskDir := feedHarness(t)
				t.Setenv("GRID_ROLE", role)
				store := feed.Store{TaskDir: taskDir, TaskID: feedTaskID}
				before, err := store.Read(feed.Filter{})
				if err != nil {
					t.Fatal(err)
				}
				body := `{"ref":"task","kind":"` + kind + `","text":"example"}`
				code := h.run(taskDir, "feed", "post", "--type", "post", "--body", body)
				after, err := store.Read(feed.Filter{})
				if err != nil {
					t.Fatal(err)
				}
				if role == "guest" && kind != "message" {
					if code != 3 {
						t.Fatalf("guest post %s: exit %d, want 3: %s", kind, code, h.errb)
					}
					if !strings.Contains(h.errb.String(), "guest") {
						t.Fatalf("guest post %s: refusal does not name the guest role: %q", kind, h.errb.String())
					}
					if len(after.Records) != len(before.Records) {
						t.Fatalf("guest post %s appended a record: %d -> %d", kind, len(before.Records), len(after.Records))
					}
					return
				}
				if code != 0 {
					t.Fatalf("%s post %s: exit %d, want 0: %s", role, kind, code, h.errb)
				}
				if len(after.Records) != len(before.Records)+1 {
					t.Fatalf("%s post %s: records %d -> %d, want one more", role, kind, len(before.Records), len(after.Records))
				}
			})
		}
	}
}

// The refusal reads the body's kind as the record schema does: a guest body with
// no kind, a kind that is not a string, or a body that is only a reference is
// not a message.
func TestGuestFeedPostNotAMessage(t *testing.T) {
	for name, body := range map[string]string{
		"no_kind":      `{"ref":"task","text":"example"}`,
		"number_kind":  `{"ref":"task","kind":1,"text":"example"}`,
		"upper_kind":   `{"ref":"task","kind":"Close","text":"example"}`,
		"body_ref":     `{"ref":"bodies/01ARZ3NDEKTSV4RRFFQ69G5FAV.json"}`,
		"spaced_close": `{"ref":"task","kind":" close","text":"example"}`,
	} {
		t.Run(name, func(t *testing.T) {
			h, taskDir := feedHarness(t)
			t.Setenv("GRID_ROLE", "guest")
			store := feed.Store{TaskDir: taskDir, TaskID: feedTaskID}
			before, err := store.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if code := h.run(taskDir, "feed", "post", "--type", "post", "--body", body); code != 3 {
				t.Fatalf("exit %d, want 3: %s", code, h.errb)
			}
			after, err := store.Read(feed.Filter{})
			if err != nil || len(after.Records) != len(before.Records) {
				t.Fatalf("record appended: %d -> %d, %v", len(before.Records), len(after.Records), err)
			}
		})
	}
}

// What a guest may still do: post a message, and answer a question addressed to
// it. A Grid-run session (GRID_LEASE=task) is exempt as it was before.
func TestGuestStillAnswersAndComments(t *testing.T) {
	t.Run("answer", func(t *testing.T) {
		h, taskDir := feedHarness(t)
		q := questionAdd(t, h, taskDir, "visitor")
		t.Setenv("GRID_ROLE", "guest")
		t.Setenv("GRID_MEMBER", "visitor")
		if code := h.run(taskDir, "feed", "post", "--type", "answer", "--re", q.Post, "--body", `{"answer":"Yes"}`); code != 0 {
			t.Fatalf("guest answer: exit %d: %s", code, h.errb)
		}
		store := feed.Store{TaskDir: taskDir, TaskID: feedTaskID}
		state, err := store.State()
		if err != nil || state.Questions[q.Post].Open {
			t.Fatalf("the question stays open: %+v %v", state.Questions[q.Post], err)
		}
	})
	t.Run("message", func(t *testing.T) {
		h, taskDir := feedHarness(t)
		t.Setenv("GRID_ROLE", "guest")
		if code := h.run(taskDir, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"a comment"}`); code != 0 {
			t.Fatalf("guest message: exit %d: %s", code, h.errb)
		}
	})
	// cli.go lets a Grid-run session (GRID_LEASE=task) through the guest verb
	// refusal; a guest-role task close under that lease still goes to the writer.
	t.Run("lease_task_task_close", func(t *testing.T) {
		h, fixture := guestTaskFixture(t, "guest", "task", false)
		before := guestSnapshot(t, fixture.Path)
		if code := h.run(h.home, "task", "close", fixture.ID); code != 0 {
			t.Fatalf("GRID_LEASE=task close: exit %d: %s", code, h.errb)
		}
		if guestSnapshot(t, fixture.Path) == before {
			t.Fatal("exit 0 but the task folder is unchanged")
		}
	})
}
