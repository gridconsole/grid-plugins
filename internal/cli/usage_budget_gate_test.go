package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// A task budget set or clear lifts or moves the hold on an agent, so it is a person-only write.
// Each verb has its own rows: agent refused, person allowed, read allowed.

func budgetGateHarness(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	return h, taskFixture(t, h).Path
}

func budgetArgs(op string, extra ...string) []string {
	args := []string{"usage", "budget", op}
	if op == "set" {
		args = append(args, "--tokens", "100")
	} else {
		args = append(args, "--tokens")
	}
	return append(args, extra...)
}

func budgetShow(t *testing.T, h *harness, task string) string {
	t.Helper()
	if code := h.run(task, "usage", "budget", "show", "--json"); code != 0 {
		t.Fatalf("show: exit %d: %s %s", code, h.out, h.errb)
	}
	return h.out.String()
}

func TestUsageBudgetPersonGate(t *testing.T) {
	for _, op := range []string{"set", "clear"} {
		verb := "usage budget " + op
		t.Run(op, func(t *testing.T) {
			t.Run("agent refused", func(t *testing.T) {
				h, task := budgetGateHarness(t)
				if code := h.run(task, "usage", "budget", "set", "--tokens", "100"); code != 0 {
					t.Fatalf("seed: %d %s", code, h.errb)
				}
				h.env.Procs = testProcs("codex")
				before, beforeHome := fileBytes(t, task), fileBytes(t, h.gridHome)
				if code := h.run(task, budgetArgs(op, "--json")...); code != 3 {
					t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
				}
				var refusal map[string]string
				if err := json.Unmarshal(h.out.Bytes(), &refusal); err != nil {
					t.Fatal(err, h.out.String())
				}
				if refusal["error"] != "person_required" || refusal["reason"] != "agent_ancestor" || refusal["hint"] != personHint {
					t.Fatal(refusal)
				}
				if want := "grid " + verb + ": " + personHint; !strings.Contains(h.errb.String(), want) {
					t.Fatalf("stderr %q lacks %q", h.errb, want)
				}
				if !reflect.DeepEqual(before, fileBytes(t, task)) || !reflect.DeepEqual(beforeHome, fileBytes(t, h.gridHome)) {
					t.Fatal("refused budget write changed files")
				}
				h.env.Procs = testProcs("Terminal")
				if !strings.Contains(budgetShow(t, h, task), `"tokens":100`) {
					t.Fatal("the budget did not survive the refused write")
				}
			})
			t.Run("person allowed", func(t *testing.T) {
				h, task := budgetGateHarness(t)
				if op == "clear" {
					if code := h.run(task, "usage", "budget", "set", "--tokens", "100"); code != 0 {
						t.Fatalf("seed: %d %s", code, h.errb)
					}
				}
				if code := h.run(task, budgetArgs(op, "--json")...); code != 0 {
					t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
				}
				got := budgetShow(t, h, task)
				if has := strings.Contains(got, `"budget":{"tokens":100}`); has != (op == "set") {
					t.Fatalf("budget after %s: %s", op, got)
				}
				dir, _ := h.env.PersonDir()
				b, err := os.ReadFile(filepath.Join(dir, "gate.log"))
				if err != nil || !strings.Contains(string(b), `"verb":"`+verb+`"`) {
					t.Fatalf("receipt %s %v", b, err)
				}
			})
			t.Run("read allowed", func(t *testing.T) {
				h, task := budgetGateHarness(t)
				h.env.Procs = testProcs("codex")
				for _, args := range [][]string{{"usage", "budget", "show"}, {"usage", "budget", "show", "--json"}} {
					if code := h.run(task, args...); code != 0 {
						t.Fatalf("%v: exit %d: %s %s", args, code, h.out, h.errb)
					}
				}
			})
			t.Run("agent environment refused", func(t *testing.T) {
				// The role an old fixture ran as; GRID_ROLE is never a way into the gate.
				for name, env := range map[string]map[string]string{
					"GRID_AGENT": {"GRID_AGENT": "builder"}, "GRID_ROLE worker": {"GRID_ROLE": "worker"},
					"GRID_ROLE builder": {"GRID_ROLE": "builder"}, "GRID_LEASE": {"GRID_LEASE": "task"},
				} {
					h, task := budgetGateHarness(t)
					h.env.Getenv = func(k string) string { return env[k] }
					before := fileBytes(t, task)
					if code := h.run(task, budgetArgs(op, "--json")...); code != 3 || !strings.Contains(h.out.String(), `"reason":"agent_env"`) {
						t.Fatalf("%s: exit %d: %s %s", name, code, h.out, h.errb)
					}
					if !reflect.DeepEqual(before, fileBytes(t, task)) {
						t.Fatalf("%s: refused write changed the task", name)
					}
				}
			})
			t.Run("every task selector refused", func(t *testing.T) {
				h, task := budgetGateHarness(t)
				h.env.Procs = testProcs("codex")
				before := fileBytes(t, task)
				for _, tc := range []struct {
					name string
					cwd  string
					argv []string
				}{
					{"cwd", task, nil},
					{"task path", h.home, []string{"--task", task}},
					{"task id", h.home, []string{"--task", "T-01"}},
					{"task=", h.home, []string{"--task=" + task}},
					// Go's flag takes --workspace as the value of --task, so this is a task write.
					{"workspace as task value", h.home, []string{"--task", "--workspace"}},
					{"unexpected operand", task, []string{"T-01"}},
				} {
					if code := h.run(tc.cwd, budgetArgs(op, append(tc.argv, "--json")...)...); code != 3 || !strings.Contains(h.out.String(), `"reason":"agent_ancestor"`) {
						t.Fatalf("%s: exit %d: %s %s", tc.name, code, h.out, h.errb)
					}
				}
				if !reflect.DeepEqual(before, fileBytes(t, task)) {
					t.Fatal("refused writes changed the task")
				}
			})
		})
	}
}

func TestUsageBudgetGateKeepsSyntaxAndWorkspaceAnswers(t *testing.T) {
	h, task := budgetGateHarness(t)
	h.env.Procs = testProcs("codex")
	// A call that fails to parse writes nothing, so it still answers exit 2.
	if code := h.run(task, "usage", "budget", "set", "--tokens", "lots"); code != 2 || !strings.Contains(h.errb.String(), "non-negative integer") {
		t.Fatalf("syntax error: exit %d: %s", code, h.errb)
	}
	// --workspace is not a task write: it keeps its own role gate and is not refused here.
	if code := h.run(h.home, "usage", "budget", "set", "--tokens", "5", "--workspace"); code != 0 {
		t.Fatalf("workspace set: exit %d: %s", code, h.errb)
	}
	if code := h.run(h.home, "usage", "budget", "clear", "--workspace"); code != 0 {
		t.Fatalf("workspace clear: exit %d: %s", code, h.errb)
	}
}
