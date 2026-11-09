package cli

import (
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/persongate"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testenv"
)

type testProcs string

func (p testProcs) Snapshot() (map[int]persongate.Process, error) {
	pid := os.Getpid()
	return map[int]persongate.Process{pid: {PID: pid, PPID: pid + 1, Exe: "grid"}, pid + 1: {PID: pid + 1, PPID: pid + 2, Exe: "bash"}, pid + 2: {PID: pid + 2, PPID: 1, Exe: string(p)}, 1: {PID: 1, Exe: "init"}}, nil
}
func personTestEnv(t *testing.T, e *Env) {
	t.Helper()
	testenv.Person(t)
	home := t.TempDir()
	e.Procs = testProcs("Terminal")
	e.PersonDir = func() (string, error) { return filepath.Join(home, ".grid-person"), nil }
}
func fileBytes(t *testing.T, root string) map[string][32]byte {
	t.Helper()
	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.Type().IsRegular() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out[rel] = sha256.Sum256(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestPersonVerbRefusals(t *testing.T) {
	for _, verb := range persongate.Floor() {
		t.Run(strings.ReplaceAll(verb, " ", "_"), func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			h.env.Procs = testProcs("codex")
			h.env.Getenv = func(string) string { return "" }
			h.write(filepath.Join(h.gridHome, "settings.json"), "{}\n")
			before := fileBytes(t, h.gridHome)
			args := append(strings.Fields(verb), "T-01", "--json")
			if code := h.run(fixture.Path, args...); code != 3 {
				t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
			}
			var refusal map[string]string
			if err := json.Unmarshal(h.out.Bytes(), &refusal); err != nil {
				t.Fatal(err)
			}
			if refusal["error"] != "person_required" || refusal["reason"] != "agent_ancestor" || refusal["hint"] != personHint {
				t.Fatal(refusal)
			}
			if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
				t.Fatal("refused command changed task/settings files")
			}
		})
	}
}

func TestPersonEnvironmentRefusals(t *testing.T) {
	keys := persongate.EnvironmentKeys([]string{"GRID_WORKER_FUTURE=agent", "GRID_LEASE_FUTURE=agent"})
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			// Simulate an inherited agent session before building the harness.
			t.Setenv(key, "agent")
			h := newHarness(t)
			fixture := taskFixture(t, h)
			if code := h.run(fixture.Path, "person", "check", "--json"); code != 0 {
				t.Fatalf("inherited marker was not isolated: %d %s", code, h.errb)
			}
			// Setting the same marker after setup must still reach the real gate.
			t.Setenv(key, "agent")
			before := fileBytes(t, h.gridHome)
			for _, args := range [][]string{{"task", "close", "T-01", "--json"}, {"person", "check", "--json"}} {
				if code := h.run(fixture.Path, args...); code != 3 || !strings.Contains(h.out.String(), `"reason":"agent_env"`) {
					t.Fatalf("%v exit %d: %s %s", args, code, h.out, h.errb)
				}
			}
			if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
				t.Fatal("refused command changed files")
			}
		})
	}
}

func TestPersonVerbTerminal(t *testing.T) {
	for _, verb := range persongate.Floor() {
		t.Run(strings.ReplaceAll(verb, " ", "_"), func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			fixture := taskFixture(t, h)
			args := append(strings.Fields(verb), "T-01")
			cwd := fixture.Path
			switch verb {
			case "approve":
				h.write(filepath.Join(cwd, "plan.md"), approvedPlan)
				if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: "T-01", Action: "status", State: "waiting"}); err != nil {
					t.Fatal(err)
				}
			case "decide", "refusal decide":
				h.write(filepath.Join(cwd, "refusals", "R-1.json"), `{"id":"R-1","command":"echo example"}`)
				args = append(args, "R-1", "allow")
			case "task status":
				args = append(args, "waiting")
			case "task reopen":
				if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: "T-01", Action: "close"}); err != nil {
					t.Fatal(err)
				}
			case "task restore":
				if code := h.run(cwd, "task", "delete", "T-01"); code != 0 {
					t.Fatal(h.errb)
				}
				args = append(args, "--project", "personal")
				cwd = h.home
			case "task rename":
				args = append(args, "--title", "A new title")
			case "task decline":
				args = append(args, "--reason", "Needs another owner")
			case "hook level":
				args = []string{"hook", "level", "home-allow", "off"}
			case "agent set":
				args = []string{"agent", "set", "grid-builder", "--model", "opus"}
			case "skill enable", "skill disable":
				args = append(strings.Fields(verb), "research")
			case "setup":
				args = []string{"setup", "--yes"}
			case "answer", "question answer", "question withdraw":
				h, cwd = feedHarness(t)
				q := questionAdd(t, h, cwd, "member")
				if verb == "question withdraw" {
					args = []string{"question", "withdraw", q.ID, "--task", cwd}
				} else {
					args = append(strings.Fields(verb), cwd, q.ID, "Yes")
				}
			}
			args = append(args, "--json")
			if code := h.run(cwd, args...); code != 0 {
				t.Fatalf("terminal command %v exit %d: %s %s", args, code, h.out, h.errb)
			}
			dir, _ := h.env.PersonDir()
			b, err := os.ReadFile(filepath.Join(dir, "gate.log"))
			if err != nil || !strings.Contains(string(b), `"verb":"`+verb+`"`) {
				t.Fatalf("receipt %s %v", b, err)
			}
		})
	}
}

func TestPersonCheck(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	if code := h.run(h.home, "person", "check", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var d persongate.Decision
	if err := json.Unmarshal(h.out.Bytes(), &d); err != nil || !d.Allow || d.Via != "terminal" {
		t.Fatalf("%+v %v", d, err)
	}
	h.env.Procs = testProcs("codex")
	if code := h.run(h.home, "person", "check", "--json"); code != 3 {
		t.Fatal(code)
	}
	if err := json.Unmarshal(h.out.Bytes(), &d); err != nil || d.Allow || d.Reason != "agent_ancestor" {
		t.Fatalf("%+v %v", d, err)
	}
}
func TestPersonWorkflowAndHelp(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	fixture := taskFixture(t, h)
	h.env.Procs = testProcs("codex")
	h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs:\n  - task show\n---\n")
	if code := h.run(fixture.Path, "task", "show", "T-01", "--json"); code != 3 || !strings.Contains(h.out.String(), "agent_ancestor") {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
	if code := h.run(fixture.Path, "task", "close", "T-01", "--reason", "--help"); code != 3 {
		t.Fatalf("help operand bypass: %d", code)
	}
	for _, args := range [][]string{{"approve", "--help"}, {"task", "close", "--help"}, {"setup", "--help"}, {"skill", "list"}, {"hook", "levels"}} {
		if code := h.run(h.home, args...); code == 3 {
			t.Fatalf("read-only %v refused: %s", args, h.errb)
		}
	}
}
func TestPersonViaHistory(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	fixture := taskFixture(t, h)
	h.env.Procs = testProcs("grid-desktop")
	if code := h.run(fixture.Path, "task", "close", "T-01", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var e task.Event
	if err := json.Unmarshal(h.out.Bytes(), &e); err != nil || e.Via != "app" {
		t.Fatalf("%+v %v", e, err)
	}
	b, err := os.ReadFile(filepath.Join(fixture.Path, "history.md"))
	if err != nil || !strings.Contains(string(b), `"via":"app"`) {
		t.Fatalf("%s %v", b, err)
	}
}

func TestPersonHelpGolden(t *testing.T) {
	h := newHarness(t)
	for _, tc := range []struct {
		name string
		args []string
	}{{"help", []string{"help"}}, {"person-help", []string{"person", "--help"}}, {"person-check-help", []string{"person", "check", "--help"}}} {
		if code := h.run(h.home, tc.args...); code != 0 {
			t.Fatal(code)
		}
		golden.Assert(t, tc.name, strings.ReplaceAll(h.out.String()+h.errb.String(), "    \t", "\t"))
	}
}
func TestSetupAgentPreviewAndWrite(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.env.Procs = testProcs("codex")
	before := fileBytes(t, h.gridHome)
	for _, args := range [][]string{{"setup"}, {"setup", "--json"}, {"setup", "--remove", "--json"}} {
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("preview %v: %d %s", args, code, h.errb)
		}
	}
	if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
		t.Fatal("preview wrote files")
	}
	if code := h.run(h.home, "setup", "--yes", "--json"); code != 3 || !strings.Contains(h.out.String(), `"reason":"agent_ancestor"`) {
		t.Fatalf("write %d %s %s", code, h.out, h.errb)
	}
	if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
		t.Fatal("refused setup wrote files")
	}
}

func TestPersonWorkflowSingleWordAndRoles(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	fixture := taskFixture(t, h)
	h.env.Procs = testProcs("codex")
	h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: [deliver]\n---\n")
	if code := h.run(fixture.Path, "deliver", "Ready", "--json"); code != 3 || !strings.Contains(h.out.String(), "agent_ancestor") {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
	h.env.Procs = testProcs("Terminal")
	h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: {approve: [member]}\n---\n")
	h.env.Getenv = func(k string) string {
		if k == "GRID_ROLE" {
			return "owner"
		}
		return ""
	}
	if code := h.run(fixture.Path, "approve", "T-01", "--json"); code != 3 || !strings.Contains(h.out.String(), "agent_env") {
		t.Fatalf("%d %s %s", code, h.out, h.errb)
	}
}

func TestPersonFloorInventory(t *testing.T) {
	want := []string{"approve", "refusal decide", "decide", "answer", "question answer", "question withdraw", "task status", "task stop", "task close", "task reopen", "task delete", "task restore", "task rename", "task decline", "hook level", "agent set", "skill enable", "skill disable", "setup"}
	if !reflect.DeepEqual(want, persongate.Floor()) {
		t.Fatalf("person floor changed: %v", persongate.Floor())
	}
}
func TestPersonLeaseWithdrawalNeedsFence(t *testing.T) {
	h, dir, _ := orchestratorHarness(t)
	q := questionAdd(t, h, dir, "member")
	h.env.Procs = testProcs("codex")
	t.Setenv("GRID_AGENT", "builder")
	t.Setenv("GRID_LEASE", "task")
	t.Setenv("GRID_LEASE_EPOCH", "0.999999")
	before := fileBytes(t, dir)
	if code := h.run(dir, "question", "withdraw", q.ID); code != 1 {
		t.Fatalf("bad fence exit %d: %s", code, h.errb)
	}
	if !reflect.DeepEqual(before, fileBytes(t, dir)) {
		t.Fatal("bad fence changed task")
	}
}
func TestPersonAnswerAuthorship(t *testing.T) {
	h, dir := feedHarness(t)
	q := questionAdd(t, h, dir, "member")
	t.Setenv("GRID_ACTOR", "reviewer")
	t.Setenv("GRID_ROLE", "owner")
	if code := h.run(dir, "answer", dir, q.ID, "Yes", "--json"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), `"who":"reviewer"`) || !strings.Contains(h.out.String(), `"role":"owner"`) {
		t.Fatal(h.out)
	}
}

func TestPersonAnswerRoleParity(t *testing.T) {
	for _, role := range []string{"guest", "lead"} {
		for _, verb := range []string{"answer", "question answer"} {
			t.Run(role+"/"+verb, func(t *testing.T) {
				h, dir := feedHarness(t)
				q := questionAdd(t, h, dir, "visitor")
				t.Setenv("GRID_ROLE", role)
				t.Setenv("GRID_ACTOR", "visitor")
				t.Setenv("GRID_MEMBER", "visitor")
				args := append(strings.Fields(verb), dir, q.ID, "Yes", "--json")
				if code := h.run(dir, args...); code != 0 {
					t.Fatalf("answer exit %d: %s %s", code, h.out, h.errb)
				}
				if code := h.run(dir, "approve", dir, "--json"); code != 3 || !strings.Contains(h.out.String(), "agent_env") {
					t.Fatalf("approve exit %d: %s %s", code, h.out, h.errb)
				}
			})
		}
	}
}

func TestPersonCheckAnswerRoles(t *testing.T) {
	for _, role := range []string{"guest", "lead"} {
		for _, host := range []string{"Terminal", "grid-server"} {
			t.Run(role+"/"+host, func(t *testing.T) {
				h := newHarness(t)
				h.env.Procs = testProcs(host)
				h.env.Getenv = func(k string) string {
					if k == "GRID_ROLE" {
						return role
					}
					return ""
				}
				if code := h.run(h.home, "person", "check", "--json"); code != 0 {
					t.Fatalf("check exit %d: %s %s", code, h.out, h.errb)
				}
				var d persongate.Decision
				if err := json.Unmarshal(h.out.Bytes(), &d); err != nil || !d.Allow {
					t.Fatalf("decision %+v error %v", d, err)
				}
			})
		}
	}
}

func TestPersonPolicyRemovalKeepsAgentChecks(t *testing.T) {
	for _, reason := range []string{"agent_ancestor", "agent_env"} {
		t.Run(reason, func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			dir, _ := h.env.PersonDir()
			h.write(filepath.Join(dir, "policy"), "remove approve\n")
			if err := os.Chmod(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filepath.Join(dir, "policy"), 0600); err != nil {
				t.Fatal(err)
			}
			h.env.Getenv = func(k string) string {
				if reason == "agent_env" && k == "GRID_AGENT" {
					return "builder"
				}
				return ""
			}
			if reason == "agent_ancestor" {
				h.env.Procs = testProcs("codex")
			}
			before := fileBytes(t, h.gridHome)
			if code := h.run(fixture.Path, "approve", "T-01", "--json"); code != 3 || !strings.Contains(h.out.String(), reason) {
				t.Fatalf("policy bypass exit %d: %s %s", code, h.out, h.errb)
			}
			if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
				t.Fatal("refused command changed files")
			}
		})
	}
}

func TestPersonAnswerScopeAndWorkflowRoles(t *testing.T) {
	for _, verb := range []string{"answer", "question answer"} {
		for _, tc := range []struct {
			name, role, who, policy, reason string
			code                            int
		}{
			{"guest_other_question", "guest", "other", "", "addressed to that guest", 1},
			{"guest_workflow_denied", "guest", "visitor", "member", "agent_env", 3},
			{"lead_workflow_denied", "lead", "visitor", "member", "agent_env", 3},
			{"guest_workflow_allowed", "guest", "visitor", "guest", "", 0},
			{"lead_workflow_allowed", "lead", "visitor", "lead", "", 0},
		} {
			t.Run(verb+"/"+tc.name, func(t *testing.T) {
				h, dir := feedHarness(t)
				q := questionAdd(t, h, dir, tc.who)
				if tc.policy != "" {
					h.write(filepath.Join(dir, "workflow.md"), "---\nperson_verbs: {"+verb+": ["+tc.policy+"]}\n---\n")
				}
				t.Setenv("GRID_ROLE", tc.role)
				t.Setenv("GRID_ACTOR", "visitor")
				t.Setenv("GRID_MEMBER", "visitor")
				before := fileBytes(t, dir)
				args := append(strings.Fields(verb), dir, q.ID, "Yes", "--json")
				if code := h.run(dir, args...); code != tc.code || !strings.Contains(h.out.String()+h.errb.String(), tc.reason) {
					t.Fatalf("answer exit %d: %s %s", code, h.out, h.errb)
				}
				if tc.code != 0 && !reflect.DeepEqual(before, fileBytes(t, dir)) {
					t.Fatal("refused answer changed task")
				}
			})
		}
	}
}
