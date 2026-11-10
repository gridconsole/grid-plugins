package cli

import (
	"strings"
	"testing"
)

// A task budget set or clear is an ordinary write: agents and people run the same verb.

func budgetGateHarness(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	return h, taskFixture(t, h).Path
}

func budgetShow(t *testing.T, h *harness, task string) string {
	t.Helper()
	if code := h.run(task, "usage", "budget", "show", "--json"); code != 0 {
		t.Fatalf("show: exit %d: %s %s", code, h.out, h.errb)
	}
	return h.out.String()
}

func TestUsageBudgetSetAndClear(t *testing.T) {
	h, task := budgetGateHarness(t)
	if code := h.run(task, "usage", "budget", "set", "--tokens", "100"); code != 0 {
		t.Fatalf("set: %d %s", code, h.errb)
	}
	if !strings.Contains(budgetShow(t, h, task), `"budget":{"tokens":100}`) {
		t.Fatal("set did not store the budget")
	}
	if code := h.run(task, "usage", "budget", "clear", "--tokens"); code != 0 {
		t.Fatalf("clear: %d %s", code, h.errb)
	}
	if strings.Contains(budgetShow(t, h, task), `"tokens":100`) {
		t.Fatal("clear left the budget")
	}
}

func TestUsageBudgetSyntaxError(t *testing.T) {
	h, task := budgetGateHarness(t)
	if code := h.run(task, "usage", "budget", "set", "--tokens", "lots"); code != 2 || !strings.Contains(h.errb.String(), "non-negative integer") {
		t.Fatalf("syntax error: exit %d: %s", code, h.errb)
	}
}
