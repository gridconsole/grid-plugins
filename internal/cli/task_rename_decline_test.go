package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestTaskRenameDecline(t *testing.T) {
	for _, tc := range []struct {
		name, verb, value, role, actor, state, kind, diagnostic string
		code                                                    int
	}{
		{name: "rename_open", verb: "rename", value: "  Better\t title  "},
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
		{name: "decline_reason", verb: "decline", value: "  Out\t of\n scope  ", actor: "alex"},
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
			h.env.Getenv = func(key string) string {
				switch key {
				case "GRID_ROLE":
					return tc.role
				case "GRID_ACTOR":
					return tc.actor
				}
				return ""
			}
			fixture := taskFixture(t, h)
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
				if !strings.Contains(h.errb.String(), tc.diagnostic) || strings.Count(h.errb.String(), "\n") != 1 {
					t.Fatalf("want one diagnostic containing %q, got %q", tc.diagnostic, h.errb.String())
				}
				after := taskTestSnapshot(t, fixture.Path)
				for name, data := range before {
					if !bytes.Equal(data, after[name]) {
						t.Fatalf("refusal changed %s", name)
					}
				}
				if tc.role == "guest" && len(after) != len(before) {
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
			if !strings.HasPrefix(history, "- Earlier history\n") || !strings.Contains(history, historyWant) || strings.Count(history, "\n") != 2 {
				t.Fatalf("history = %s", history)
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
