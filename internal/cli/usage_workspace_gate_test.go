package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/usage"
)

// A workspace token budget write moves the hold on every task, so like the task write it is
// person-only. The caller role policy still runs first and keeps its own exit 3.

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

func hasGateReceipt(t *testing.T, h *harness) (string, bool) {
	t.Helper()
	dir, _ := h.env.PersonDir()
	b, err := os.ReadFile(filepath.Join(dir, "gate.log"))
	return string(b), err == nil
}

// wantWorkspaceRefused checks one refused write: exit 3, the person_required shape, the
// stderr line, no receipt and a Grid home that did not change.
func wantWorkspaceRefused(t *testing.T, h *harness, op, name, reason string, before map[string][32]byte) {
	t.Helper()
	if code := h.run(h.home, workspaceArgs(op, "--json")...); code != 3 {
		t.Fatalf("%s: exit %d: %s %s", name, code, h.out, h.errb)
	}
	var refusal map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &refusal); err != nil {
		t.Fatalf("%s: %v: %s", name, err, h.out)
	}
	if refusal["error"] != "person_required" || refusal["reason"] != reason || refusal["hint"] != personHint {
		t.Fatalf("%s: %v", name, refusal)
	}
	if want := "grid usage budget " + op + ": " + personHint; !strings.Contains(h.errb.String(), want) {
		t.Fatalf("%s: stderr %q lacks %q", name, h.errb, want)
	}
	if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
		t.Fatalf("%s: refused write changed the Grid home", name)
	}
	if b, ok := hasGateReceipt(t, h); ok {
		t.Fatalf("%s: refused write left a receipt: %s", name, b)
	}
}

func TestUsageWorkspaceBudgetPersonGate(t *testing.T) {
	for _, op := range []string{"set", "clear"} {
		t.Run(op, func(t *testing.T) {
			// The defect: an agent with no role set was let through.
			t.Run("agent ancestor with empty GRID_ROLE refused", func(t *testing.T) {
				h := workspaceGateHarness(t, nil)
				h.env.Procs = testProcs("codex")
				wantWorkspaceRefused(t, h, op, "empty role", "agent_ancestor", fileBytes(t, h.gridHome))
				h.env.Procs = testProcs("Terminal")
				if got := workspaceBudget(t, h); got != `{"tokens_per_task":100}` {
					t.Fatalf("the budget did not survive the refused write: %s", got)
				}
			})
			t.Run("agent ancestor with owner role refused", func(t *testing.T) {
				h := workspaceGateHarness(t, map[string]string{"GRID_ROLE": "owner"})
				h.env.Procs = testProcs("codex")
				wantWorkspaceRefused(t, h, op, "owner role", "agent_ancestor", fileBytes(t, h.gridHome))
			})
			t.Run("agent environment refused", func(t *testing.T) {
				for name, env := range map[string]map[string]string{
					"GRID_AGENT": {"GRID_AGENT": "builder"}, "GRID_LEASE": {"GRID_LEASE": "task"},
				} {
					h := workspaceGateHarness(t, env)
					wantWorkspaceRefused(t, h, op, name, "agent_env", fileBytes(t, h.gridHome))
				}
			})
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
			// With a role policy that lets these roles through, the person gate alone refuses them.
			t.Run("worker and builder role refused by the person gate", func(t *testing.T) {
				write := usage.WorkspaceBudgetWriteAllowed
				t.Cleanup(func() { usage.WorkspaceBudgetWriteAllowed = write })
				usage.WorkspaceBudgetWriteAllowed = func(string) bool { return true }
				for _, role := range []string{"worker", "builder"} {
					h := workspaceGateHarness(t, map[string]string{"GRID_ROLE": role})
					wantWorkspaceRefused(t, h, op, role, "agent_env", fileBytes(t, h.gridHome))
				}
			})
			t.Run("person allowed", func(t *testing.T) {
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
					if b, ok := hasGateReceipt(t, h); !ok || !strings.Contains(b, `"verb":"usage budget `+op+`"`) {
						t.Fatalf("role %q: receipt %q", role, b)
					}
				}
			})
			t.Run("syntax error from an agent still exits 2", func(t *testing.T) {
				h := workspaceGateHarness(t, nil)
				h.env.Procs = testProcs("codex")
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
		"agent ancestor": nil, "GRID_AGENT": {"GRID_AGENT": "builder"}, "GRID_LEASE": {"GRID_LEASE": "task"},
	} {
		h := workspaceGateHarness(t, env)
		if name == "agent ancestor" {
			h.env.Procs = testProcs("codex")
		}
		for _, args := range [][]string{{"usage", "budget", "show", "--workspace", "--json"}, {"usage", "budget", "show", "--workspace"}} {
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("%s %v: exit %d: %s %s", name, args, code, h.out, h.errb)
			}
		}
		if b, ok := hasGateReceipt(t, h); ok {
			t.Fatalf("%s: a read left a receipt: %s", name, b)
		}
	}
}

// A workflow person_verbs extension that names the verb (or the whole usage command) applies to
// the --workspace form as it does to the task form: fixes L148 review must-fix 1, where the
// early policy gate was skipped for every budget write and the handler gated only the task form.
func TestUsageWorkspaceBudgetWorkflowPolicy(t *testing.T) {
	for _, op := range []string{"set", "clear"} {
		for _, policy := range []string{"[usage budget " + op + "]", "[usage]", "{usage budget " + op + ": [owner]}"} {
			t.Run(op+" "+policy, func(t *testing.T) {
				h := workspaceGateHarness(t, map[string]string{"GRID_ROLE": "owner"})
				task := taskFixture(t, h).Path
				h.write(filepath.Join(task, "workflow.md"), "---\nperson_verbs: "+policy+"\n---\n")
				h.env.Procs = testProcs("codex")
				before := fileBytes(t, h.gridHome)
				if code := h.run(task, workspaceArgs(op, "--json")...); code != 3 {
					t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
				}
				var refusal map[string]string
				if err := json.Unmarshal(h.out.Bytes(), &refusal); err != nil {
					t.Fatal(err, h.out.String())
				}
				if refusal["error"] != "person_required" || refusal["reason"] != "agent_ancestor" {
					t.Fatal(refusal)
				}
				if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
					t.Fatal("refused write changed the Grid home")
				}
			})
		}
		// The role map reaches a person's workspace write: an empty role is a member, not an owner.
		t.Run(op+" role map applies to a person", func(t *testing.T) {
			policy := "{usage budget " + op + ": [owner]}"
			for role, want := range map[string]int{"": 3, "owner": 0} {
				h := workspaceGateHarness(t, map[string]string{"GRID_ROLE": role})
				task := taskFixture(t, h).Path
				h.write(filepath.Join(task, "workflow.md"), "---\nperson_verbs: "+policy+"\n---\n")
				before := fileBytes(t, h.gridHome)
				code := h.run(task, workspaceArgs(op, "--json")...)
				if code != want {
					t.Fatalf("role %q: exit %d, want %d: %s %s", role, code, want, h.out, h.errb)
				}
				if want == 3 {
					if !strings.Contains(h.out.String(), `"reason":"agent_env"`) || !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
						t.Fatalf("role %q: %s, home changed %v", role, h.out, !reflect.DeepEqual(before, fileBytes(t, h.gridHome)))
					}
				}
			}
		})
	}
}
