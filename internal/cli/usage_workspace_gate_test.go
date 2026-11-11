package cli

import (
	"path/filepath"
	"testing"
)

// The workspace token budget is only read: show stays open to every caller, and the
// set and clear writes are gone (see TestUsageBudgetSetAndClearAreUnknownVerbs).

func workspaceGateHarness(t *testing.T, env map[string]string) *harness {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(k string) string { return env[k] }
	h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\nBody\n")
	return h
}

func TestUsageWorkspaceBudgetShowStaysOpen(t *testing.T) {
	for name, env := range map[string]map[string]string{
		"no role": nil, "GRID_AGENT": {"GRID_AGENT": "builder"}, "GRID_LEASE": {"GRID_LEASE": "task"},
	} {
		h := workspaceGateHarness(t, env)
		for _, args := range [][]string{{"usage", "budget", "show", "--workspace", "--json"}, {"usage", "budget", "show", "--workspace"}} {
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("%s %v: exit %d: %s %s", name, args, code, h.out, h.errb)
			}
		}
	}
}
