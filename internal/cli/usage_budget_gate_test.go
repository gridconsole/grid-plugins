package cli

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/usage"
)

// Usage is a number: there is no verb that sets or clears a task budget, and a
// budget a task already carries is only read.

// setTaskBudget gives a task a budget the way an older task already has one.
func setTaskBudget(t *testing.T, h *harness, task string, change usage.Change) {
	t.Helper()
	change.Budget = true
	if _, err := (usage.Store{TaskPath: task, GridHome: h.gridHome}).Configure(change); err != nil {
		t.Fatal(err)
	}
}

func TestUsageBudgetSetAndClearAreUnknownVerbs(t *testing.T) {
	for _, args := range [][]string{
		{"usage", "budget", "set", "--tokens", "100"},
		{"usage", "budget", "set", "--workspace", "--tokens", "100"},
		{"usage", "budget", "clear", "--tokens"},
		{"usage", "budget", "clear", "--workspace"},
	} {
		h := newHarness(t)
		h.env.Getenv = func(string) string { return "" }
		task := taskFixture(t, h).Path
		if code := h.run(task, args...); code != 2 || !strings.Contains(h.errb.String(), `usage budget: unknown command "`+args[2]+`"`) || h.out.Len() != 0 {
			t.Fatalf("%v: exit %d: %s %s", args, code, h.out, h.errb)
		}
		if code := h.run(task, "usage", "--help"); code != 0 || strings.Contains(h.out.String(), "budget set") || strings.Contains(h.out.String(), "budget show | clear") {
			t.Fatalf("help still lists the budget verbs: %s", h.out)
		}
	}
}
