package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestTaskRenameDecline(t *testing.T) {
	for _, tc := range []struct {
		name, verb, value, role, actor, state, kind, diagnostic string
		code                                                    int
	}{
		{name: "rename_open", verb: "rename", value: "  Better\u2003 title  "},
		{name: "rename_esc", verb: "rename", value: "before\x1bafter", code: 2, diagnostic: "title"},
		{name: "rename_bel", verb: "rename", value: "before\x07after", code: 2, diagnostic: "title"},
		{name: "rename_fffe", verb: "rename", value: "before\ufffeafter", code: 2, diagnostic: "title"},
		{name: "rename_ffff", verb: "rename", value: "before\uffffafter", code: 2, diagnostic: "title"},
		{name: "rename_bidi_override", verb: "rename", value: "before\u202eafter", code: 2, diagnostic: "title"},
		{name: "rename_bidi_isolate", verb: "rename", value: "before\u2066after", code: 2, diagnostic: "title"},
		{name: "rename_tab", verb: "rename", value: "before\tafter", code: 2, diagnostic: "title"},
		{name: "rename_newline", verb: "rename", value: "before\nafter", code: 2, diagnostic: "title"},
		{name: "decline_esc", verb: "decline", value: "before\x1bafter", code: 2, diagnostic: "reason"},
		{name: "decline_bel", verb: "decline", value: "before\x07after", code: 2, diagnostic: "reason"},
		{name: "decline_fffe", verb: "decline", value: "before\ufffeafter", code: 2, diagnostic: "reason"},
		{name: "decline_ffff", verb: "decline", value: "before\uffffafter", code: 2, diagnostic: "reason"},
		{name: "decline_bidi_override", verb: "decline", value: "before\u202eafter", code: 2, diagnostic: "reason"},
		{name: "decline_bidi_isolate", verb: "decline", value: "before\u2066after", code: 2, diagnostic: "reason"},
		{name: "decline_tab", verb: "decline", value: "before\tafter", code: 2, diagnostic: "reason"},
		{name: "decline_newline", verb: "decline", value: "before\nafter", code: 2, diagnostic: "reason"},
		{name: "rename_member", verb: "rename", value: "Member title", role: "member"},
		{name: "rename_owner", verb: "rename", value: "Owner title", role: "owner"},
		{name: "rename_same", verb: "rename", value: "  Fix the label  ", code: 3, diagnostic: "already has that title"},
		{name: "rename_closed", verb: "rename", value: "Better title", state: "closed", code: 3, diagnostic: "reopen it before renaming"},
		{name: "rename_empty", verb: "rename", code: 2, diagnostic: "title"},
		{name: "rename_whitespace", verb: "rename", value: " \t ", code: 2, diagnostic: "title"},
		{name: "rename_multiline", verb: "rename", value: "one\ntwo", code: 2, diagnostic: "title"},
		{name: "rename_carriage_return", verb: "rename", value: "one\rtwo", code: 2, diagnostic: "title"},
		{name: "rename_too_long", verb: "rename", value: strings.Repeat("a", 201), code: 2, diagnostic: "title"},
		{name: "rename_unicode_limit", verb: "rename", value: strings.Repeat("界", 200)},
		{name: "rename_yaml", verb: "rename", value: `A: "quoted" # title`},
		{name: "rename_guest", verb: "rename", value: "Guest title", role: "guest", code: 1, diagnostic: "scope"},
		{name: "decline_reason", verb: "decline", value: "  Out\u2003 of  scope  ", actor: "alex"},
		{name: "decline_member", verb: "decline", value: "Later", role: "member"},
		{name: "decline_owner", verb: "decline", value: "Later", role: "owner"},
		{name: "decline_missing", verb: "decline", code: 2, diagnostic: "reason"},
		{name: "decline_whitespace", verb: "decline", value: " \t\n ", code: 2, diagnostic: "reason"},
		{name: "decline_closed", verb: "decline", value: "Later", state: "closed", code: 3, diagnostic: "already closed"},
		{name: "decline_planner", verb: "decline", value: "Later", kind: "planner", code: 3, diagnostic: "Planner"},
		{name: "decline_guest", verb: "decline", value: "Later", role: "guest", code: 1, diagnostic: "scope"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			// The fixture is created as a local caller: main refuses a guest the Personal project.
			fixture := taskFixture(t, h)
			h.env.Getenv = func(key string) string {
				switch key {
				case "GRID_ROLE":
					return tc.role
				case "GRID_ACTOR":
					return tc.actor
				}
				return ""
			}
			path := filepath.Join(fixture.Path, "task.md")
			original := readTaskTestFile(t, path)
			original = strings.Replace(original, "state: queued", "state: queued\nupdated: '2026-01-01T00:00:00Z'\nsummary:  'Keep this exact spacing' # comment\nextra: { one: 1, two: 2 }", 1)
			if tc.state != "" {
				original = strings.Replace(original, "state: queued", "state: "+tc.state, 1)
			}
			if tc.kind != "" {
				original = strings.Replace(original, "state:", "kind: "+tc.kind+"\nstate:", 1)
			}
			original += "\n## Result\nKeep this body exactly.\n"
			h.write(path, original)
			h.write(filepath.Join(fixture.Path, "history.md"), "- Earlier history\n")
			before := taskTestSnapshot(t, fixture.Path)
			flag := "--title"
			if tc.verb == "decline" {
				flag = "--reason"
			}
			args := []string{"task", tc.verb, fixture.ID, "--project", "personal", "--json"}
			if tc.value != "" {
				args = append(args, flag, tc.value)
			}
			if code := h.run(h.home, args...); code != tc.code {
				t.Fatalf("%s exit = %d, want %d: %s", tc.name, code, tc.code, h.errb)
			}
			if tc.code != 0 {
				if tc.code == 2 {
					want := "invalid task edit: title must have 1-200 characters on one line without control or bidi override/isolate characters\n"
					if tc.verb == "decline" {
						want = "invalid task edit: a decline needs a reason without control or bidi override/isolate characters\n"
					}
					want = "grid task " + tc.verb + ": " + want
					if h.errb.String() != want || h.out.Len() != 0 {
						t.Fatalf("refusal stdout = %q, stderr = %q; want empty stdout and %q", h.out.String(), h.errb.String(), want)
					}
				}
				if !strings.Contains(h.errb.String(), tc.diagnostic) || strings.Count(h.errb.String(), "\n") != 1 {
					t.Fatalf("want one diagnostic containing %q, got %q", tc.diagnostic, h.errb.String())
				}
				after := taskTestSnapshot(t, fixture.Path)
				for name, data := range before {
					if !bytes.Equal(data, after[name]) {
						t.Fatalf("refusal changed %s", name)
					}
				}
				if (tc.role == "guest" || tc.code == 2) && len(after) != len(before) {
					t.Fatal("guest refusal created files")
				}
				return
			}
			var event task.Event
			if err := json.Unmarshal(h.out.Bytes(), &event); err != nil {
				t.Fatal(err)
			}
			if event.What != tc.verb || event.Task != fixture.ID {
				t.Fatalf("receipt = %+v", event)
			}
			got, err := task.Read(fixture.Path)
			if err != nil {
				t.Fatal(err)
			}
			value := strings.Join(strings.Fields(tc.value), " ")
			historyWant := "renamed · " + value
			changed := map[string]bool{"title": true, "updated": true}
			if tc.verb == "rename" {
				if got.Title != value || got.State != fixture.State {
					t.Fatalf("rename result = %+v", got)
				}
			} else {
				if got.State != "closed" || got.Title != fixture.Title {
					t.Fatalf("decline result = %+v", got)
				}
				who := tc.actor
				if who == "" {
					who = "you"
				}
				if !strings.Contains(readTaskTestFile(t, path), "declined by "+who) {
					t.Fatalf("missing next: declined by %s", who)
				}
				historyWant = "closed · declined · " + value
				changed = map[string]bool{"state": true, "next": true, "updated": true}
			}
			text := readTaskTestFile(t, path)
			if strings.Contains(text, "updated: '2026-01-01T00:00:00Z'") || !strings.Contains(text, event.When) {
				t.Fatal("updated did not match receipt time")
			}
			if omitTaskLines(original, changed) != omitTaskLines(text, changed) {
				t.Fatalf("unrelated task lines changed:\n%s", text)
			}
			if got.Intent != fixture.Intent {
				t.Fatal("intent changed")
			}
			history := readTaskTestFile(t, filepath.Join(fixture.Path, "history.md"))
			if !strings.HasPrefix(history, "- Earlier history\n") || strings.Count(history, "\n") != 2 {
				t.Fatalf("history = %s", history)
			}
			var recorded task.Event
			line := strings.Split(history, "\n")[1]
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "- ")), &recorded); err != nil || recorded != event || recorded.Note != historyWant {
				t.Fatalf("history receipt = %+v, %v", recorded, err)
			}
			if tc.role == "member" && event.Who.Role != "member" {
				t.Fatalf("lost member audit role: %+v", event.Who)
			}
		})
	}
}

func TestTaskRenameDeclineSymlink(t *testing.T) {
	for _, verb := range []string{"rename", "decline"} {
		t.Run(verb, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			fixture := taskFixture(t, h)
			path := filepath.Join(fixture.Path, "task.md")
			original := readTaskTestFile(t, path)
			target := filepath.Join(h.root, "outside.md")
			h.write(target, original)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			flag := "--title"
			if verb == "decline" {
				flag = "--reason"
			}
			if code := h.run(h.home, "task", verb, fixture.ID, flag, "Changed"); code != 1 || !strings.Contains(h.errb.String(), "symlink") {
				t.Fatalf("%s symlink exit = %d, want 1: %s", verb, code, h.errb)
			}
			if readTaskTestFile(t, target) != original {
				t.Fatal("symlink target changed")
			}
		})
	}
}

func TestTaskRenameDeclineHelp(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "task", "--help"); code != 0 {
		t.Fatalf("help exit = %d", code)
	}
	golden.Assert(t, "task-help", h.out.String())
}

func readTaskTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func taskTestSnapshot(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[strings.TrimPrefix(path, dir)] = data
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func omitTaskLines(text string, keys map[string]bool) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		key, _, _ := strings.Cut(line, ":")
		if !keys[key] {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

func TestTaskRenameDeclineCustomPolicy(t *testing.T) {
	oldRename, oldDecline := task.RenameAllowed, task.DeclineAllowed
	t.Cleanup(func() { task.RenameAllowed, task.DeclineAllowed = oldRename, oldDecline })
	for _, verb := range []string{"rename", "decline"} {
		t.Run(verb, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(key string) string {
				if key == "GRID_ROLE" {
					return "member"
				}
				return ""
			}
			fixture := taskFixture(t, h)
			seen := ""
			policy := func(role string) bool { seen = role; return role != "member" }
			task.RenameAllowed, task.DeclineAllowed = policy, policy
			before := taskTestSnapshot(t, fixture.Path)
			flag := "--title"
			if verb == "decline" {
				flag = "--reason"
			}
			if code := h.run(h.home, "task", verb, fixture.ID, flag, "Changed"); code != 1 || seen != "member" || !strings.Contains(h.errb.String(), "scope") {
				t.Fatalf("custom %s policy: exit %d, role %q: %s", verb, code, seen, h.errb)
			}
			after := taskTestSnapshot(t, fixture.Path)
			if len(after) != len(before) {
				t.Fatal("policy refusal created files")
			}
			for name, data := range before {
				if !bytes.Equal(data, after[name]) {
					t.Fatalf("policy refusal changed %s", name)
				}
			}
		})
	}
}

func TestTaskRenameDeclineProject(t *testing.T) {
	for _, verb := range []string{"rename", "decline"} {
		for _, selection := range []string{"", "example", "01EXAMPLE", "personal", "missing"} {
			t.Run(verb+"/"+selection, func(t *testing.T) {
				h := newHarness(t)
				h.env.Getenv = func(string) string { return "" }
				h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
				fixture, err := taskWriter(h.env).New(task.NewOptions{Cwd: h.home, Project: "example", Words: "Example task", Who: "alex"})
				if err != nil {
					t.Fatal(err)
				}
				before := taskTestSnapshot(t, fixture.Path)
				flag := "--title"
				if verb == "decline" {
					flag = "--reason"
				}
				args := []string{"task", verb, flag, "Changed", fixture.ID}
				if selection != "" {
					args = append(args, "--project", selection)
				}
				want := 0
				if selection == "personal" || selection == "missing" {
					want = 1
				}
				if code := h.run(h.home, args...); code != want {
					t.Fatalf("project %s exit %d, want %d: %s", selection, code, want, h.errb)
				}
				if want != 0 {
					after := taskTestSnapshot(t, fixture.Path)
					for name, data := range before {
						if !bytes.Equal(data, after[name]) {
							t.Fatalf("scope refusal changed %s", name)
						}
					}
				}
			})
		}
	}
}

func TestTaskDeclineHostPerson(t *testing.T) {
	for _, tc := range []struct{ actor, who, want string }{
		{"alex", "sam", "alex"}, {"", "sam", "sam"}, {"", "", "you"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(key string) string {
				switch key {
				case "GRID_ACTOR":
					return tc.actor
				case "GRID_WHO":
					return tc.who
				}
				return ""
			}
			fixture := taskFixture(t, h)
			if code := h.run(h.home, "task", "decline", fixture.ID, "--reason", "Later"); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if text := readTaskTestFile(t, filepath.Join(fixture.Path, "task.md")); !strings.Contains(text, "declined by "+tc.want) {
				t.Fatalf("host person lost: %s", text)
			}
		})
	}
}
