package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A workspace token budget write is limited by the caller role policy (exit 3); agents and
// people run the same verb, so only the role decides.

func workspaceGateHarness(t *testing.T, env map[string]string) *harness {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(k string) string { return env[k] }
	h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\nBody\n")
	return h
}

func workspaceArgs(op string, extra ...string) []string {
	args := []string{"usage", "budget", op}
	if op == "set" {
		args = append(args, "--tokens", "200")
	}
	return append(append(args, "--workspace"), extra...)
}

func workspaceBudget(t *testing.T, h *harness) string {
	t.Helper()
	if code := h.run(h.home, "usage", "budget", "show", "--workspace", "--json"); code != 0 {
		t.Fatalf("show: exit %d: %s %s", code, h.out, h.errb)
	}
	return strings.TrimSpace(h.out.String())
}

func TestUsageWorkspaceBudgetCallerRole(t *testing.T) {
	for _, op := range []string{"set", "clear"} {
		t.Run(op, func(t *testing.T) {
			t.Run("worker and builder role refused by the role policy", func(t *testing.T) {
				for _, role := range []string{"worker", "builder"} {
					h := workspaceGateHarness(t, map[string]string{"GRID_ROLE": role})
					before := fileBytes(t, h.gridHome)
					if code := h.run(h.home, workspaceArgs(op, "--json")...); code != 3 || h.out.Len() != 0 {
						t.Fatalf("%s: exit %d: %s %s", role, code, h.out, h.errb)
					}
					if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
						t.Fatalf("%s: refused write changed the Grid home", role)
					}
				}
			})
			t.Run("empty and owner role allowed", func(t *testing.T) {
				for _, role := range []string{"", "owner"} {
					h := workspaceGateHarness(t, map[string]string{"GRID_ROLE": role})
					if code := h.run(h.home, workspaceArgs(op, "--json")...); code != 0 {
						t.Fatalf("role %q: exit %d: %s %s", role, code, h.out, h.errb)
					}
					want := `{"tokens_per_task":200}`
					if op == "clear" {
						want = `{"tokens_per_task":null}`
					}
					if got := strings.TrimSpace(h.out.String()); got != want {
						t.Fatalf("role %q: stdout %s, want %s", role, got, want)
					}
					if got := workspaceBudget(t, h); got != want {
						t.Fatalf("role %q: budget after %s: %s", role, op, got)
					}
				}
			})
			t.Run("syntax error still exits 2", func(t *testing.T) {
				h := workspaceGateHarness(t, nil)
				before := fileBytes(t, h.gridHome)
				if code := h.run(h.home, "usage", "budget", op, "--workspace", "--dollars", "1"); code != 2 {
					t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
				}
				if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
					t.Fatal("a call that failed to parse changed the Grid home")
				}
			})
		})
	}
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
