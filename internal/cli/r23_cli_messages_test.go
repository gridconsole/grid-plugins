package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// Row 6: the task state refusals say one plain sentence per action and keep
// the task_state code; none prints the state table's words ("current state",
// `to ""`).
func TestR23StateRefusalsArePlain(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"close of a closed task", []string{"task", "close", "T-01"}, "grid: T-01 is already closed\n"},
		{"stop of a closed task", []string{"task", "stop", "T-01"}, "grid: T-01 is already closed\n"},
		{"status to the state it has", []string{"task", "status", "T-01", "closed"}, "grid: T-01 is already closed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := yeTsHarness(t, true)
			if code := h.run(h.home, tc.args...); code != 1 || h.errb.String() != tc.want || h.out.Len() != 0 {
				t.Fatalf("exit %d stdout %q stderr %q, want exit 1 stderr %q", code, h.out.String(), h.errb.String(), tc.want)
			}
			h2, _ := yeTsHarness(t, true)
			if code := h2.run(h2.home, append(tc.args[:len(tc.args):len(tc.args)], "--json")...); code != 1 || h2.out.String() != yeTsEnvelope("task_state", tc.want) {
				t.Fatalf("--json: exit %d stdout %q, want the task_state envelope of %q", code, h2.out.String(), tc.want)
			}
		})
	}
	t.Run("reopen and a move the table refuses name no internals", func(t *testing.T) {
		h, _ := yeTsHarness(t, false)
		if code := h.run(h.home, "task", "reopen", "T-01"); code != 1 || h.errb.String() != "grid: T-01 is queued; only a closed task can be reopened\n" {
			t.Fatalf("reopen: exit %d stderr %q", code, h.errb.String())
		}
		h, _ = yeTsHarness(t, true)
		if code := h.run(h.home, "task", "status", "T-01", "running"); code != 1 || h.errb.String() != "grid: T-01 is closed and cannot move to running\n" {
			t.Fatalf("status: exit %d stderr %q", code, h.errb.String())
		}
	})
	t.Run("deliver on a waiting task, plain and --json", func(t *testing.T) {
		h, path, _ := jwDaHarness(t)
		tk, err := task.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		want := "deliver: " + tk.ID + " is already waiting\n"
		for _, args := range [][]string{{"deliver", "Result again"}, {"deliver", "--dry-run", "Result again"}} {
			h.out.Reset()
			h.errb.Reset()
			if code := h.run(path, args...); code != 1 || h.out.Len() != 0 || h.errb.String() != want {
				t.Fatalf("%v: exit %d stdout %q stderr %q, want stderr %q", args, code, h.out.String(), h.errb.String(), want)
			}
		}
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(path, "deliver", "--json", "Result again"); code != 1 || h.out.String() != yeTsEnvelope("task_state", want) {
			t.Fatalf("--json: exit %d stdout %q", code, h.out.String())
		}
	})
}

// Row 7: grid task list --state names the allowed states and exits 2 for a word
// that is no state; a real state with no task still lists nothing, exit 0.
func TestR23TaskListRefusesUnknownState(t *testing.T) {
	for _, args := range [][]string{{"task", "list", "--state", "bogus"}, {"task", "list", "--state", "bogus", "--json"}} {
		h := newHarness(t)
		taskFixture(t, h)
		code := h.run(h.home, args...)
		if code != 2 || h.out.Len() != 0 && !strings.Contains(h.out.String(), `"error"`) {
			t.Fatalf("%v: exit %d stdout %q stderr %q, want exit 2", args, code, h.out.String(), h.errb.String())
		}
		msg := h.errb.String()
		for _, state := range []string{"queued", "running", "waiting", "paused", "closed"} {
			if !strings.Contains(msg, state) {
				t.Fatalf("%v: stderr %q does not name the state %s", args, msg, state)
			}
		}
		if !strings.Contains(msg, `"bogus"`) {
			t.Fatalf("%v: stderr %q does not name the word", args, msg)
		}
	}
	h := newHarness(t)
	taskFixture(t, h)
	if code := h.run(h.home, "task", "list", "--state", "closed", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != "[]" {
		t.Fatalf("closed filter: exit %d stdout %q", code, h.out.String())
	}
	var list []map[string]any
	h = newHarness(t)
	taskFixture(t, h)
	if code := h.run(h.home, "task", "list", "--state", "queued", "--json"); code != 0 || json.Unmarshal(h.out.Bytes(), &list) != nil || len(list) != 1 {
		t.Fatalf("queued filter: exit %d stdout %q", code, h.out.String())
	}
}

// Row 8: a task with no plan.md gets one sentence, with no path, from every
// command that reads the plan.
func TestR23MissingPlanSaysNoPath(t *testing.T) {
	for _, args := range [][]string{
		{"plan", "items", "--json"},
		{"plan", "hash", "--json"},
		{"proof", "add", "1", "--text", "checked"},
		{"proof", "index"},
		{"job", "verify", "J-1"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			code := h.run(fixture.Path, args...)
			msg := h.errb.String() + h.out.String()
			if code == 0 {
				t.Fatalf("exit 0, want a refusal; stdout %q", h.out.String())
			}
			if strings.Contains(msg, "lstat") || strings.Contains(msg, fixture.Path) || strings.Contains(msg, "no such file") {
				t.Fatalf("exit %d, the refusal prints the path or the OS error: %q", code, msg)
			}
			if !strings.Contains(msg, "plan.md is missing; write the plan first") {
				t.Fatalf("exit %d, want the one sentence about plan.md: %q", code, msg)
			}
		})
	}
}
