package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/usage"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// holdFixture is a Grid home with one project task (T-01-demo) and the project's
// code folder, which is outside the task folder, as a worker's worktree is.
type holdFixture struct {
	home, task, code, data string
	resolver               where.Resolver
}

func holdSetup(t *testing.T) *holdFixture {
	t.Helper()
	root := gridhome.Real(t.TempDir())
	f := &holdFixture{home: filepath.Join(root, "grid"), code: filepath.Join(root, "code"), data: filepath.Join(root, "data")}
	f.task = filepath.Join(f.home, "projects", "example", "tasks", "T-01-demo")
	for _, dir := range []string{filepath.Join(f.task, "artifacts"), f.code, f.data} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(f.task, "task.md"), []byte("---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n---\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.resolver = where.Resolver{GridHome: f.home, Home: root, Ceiling: root, Machine: "machine-demo"}
	rec := project.Record{Name: "example", Folders: []project.Folder{{Machine: "machine-demo", Path: f.code}}}
	if err := project.Write(f.home, rec); err != nil {
		t.Fatal(err)
	}
	return f
}

// spend posts usage and then sets the budget, so no notice is posted.
func (f *holdFixture) spend(t *testing.T, spent usage.Input, budget usage.Change) {
	t.Helper()
	s := usage.Store{TaskPath: f.task, GridHome: f.home}
	spent.Job = "lead"
	if _, err := s.Post(spent); err != nil {
		t.Fatal(err)
	}
	budget.Budget = true
	if _, err := s.Configure(budget); err != nil {
		t.Fatal(err)
	}
}

type holdResult struct {
	out, stderr, behavior, reason string
}

func (f *holdFixture) run(t *testing.T, event, tool, cwd string, env map[string]string, input map[string]any) holdResult {
	t.Helper()
	b, err := json.Marshal(hookio.Input{ToolName: tool, Cwd: cwd, SessionID: "session-demo", ToolInput: input})
	if err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	code := Run(Options{
		Event: event, Stdin: bytes.NewReader(b), Stdout: &out, Stderr: &stderr, Resolver: f.resolver, Cwd: cwd,
		Getenv: func(k string) string {
			if v, ok := env[k]; ok {
				return v
			}
			return map[string]string{"GRID_PLAN_GUARD": "off", "CLAUDE_PLUGIN_DATA": f.data, "GRID_PROJECTS": filepath.Join(f.home, "projects")}[k]
		},
	})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got := holdResult{out: out.String(), stderr: stderr.String()}
	if out.Len() > 0 {
		var decoded struct {
			Output struct {
				Behavior string `json:"permissionDecision"`
				Reason   string `json:"permissionDecisionReason"`
				Decision struct {
					Behavior string `json:"behavior"`
					Message  string `json:"message"`
				} `json:"decision"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
			t.Fatalf("%s: %v", out.String(), err)
		}
		got.behavior, got.reason = decoded.Output.Behavior, decoded.Output.Reason
		if got.behavior == "" {
			got.behavior, got.reason = decoded.Output.Decision.Behavior, decoded.Output.Decision.Message
		}
	}
	return got
}

var agentInput = map[string]any{"prompt": "Map the settings code.", "description": "Map settings", "subagent_type": "grid:grid-scout"}

func (f *holdFixture) agent(t *testing.T) holdResult {
	t.Helper()
	return f.run(t, "pretooluse", "Agent", f.task, nil, agentInput)
}

func TestHoldDeniesAnAgentStartAtTheBudget(t *testing.T) {
	for _, tc := range []struct {
		name   string
		spent  usage.Input
		budget usage.Change
		deny   bool
		reason []string
	}{
		{"dollars 10 of 10", usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}}, true, []string{"dollars: 10 spent of a budget of 10"}},
		{"dollars 9.99 of 10", usage.Input{Dollars: ptr(9.99)}, usage.Change{Dollars: []float64{10}}, false, nil},
		{"minutes 60 of 60", usage.Input{Minutes: ptr(60.0)}, usage.Change{Minutes: []float64{60}}, true, []string{"minutes: 60 spent of a budget of 60"}},
		{"minutes 59.99 of 60", usage.Input{Minutes: ptr(59.99)}, usage.Change{Minutes: []float64{60}}, false, nil},
		{"tokens 5000 of 5000", usage.Input{Tokens: ptr(uint64(5000))}, usage.Change{BudgetTokens: []uint64{5000}}, true, []string{"tokens: 5000 spent of a budget of 5000"}},
		{"tokens 4999 of 5000", usage.Input{Tokens: ptr(uint64(4999))}, usage.Change{BudgetTokens: []uint64{5000}}, false, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := holdSetup(t)
			f.spend(t, tc.spent, tc.budget)
			got := f.agent(t)
			if tc.deny {
				if got.behavior != "deny" || got.stderr != "" {
					t.Fatalf("want deny, got %+v", got)
				}
				// The deny names the unit, the spent amount and the budget, and addresses the lift to the person.
				for _, want := range append(tc.reason, "Stop and ask the person", "grid usage budget set --task T-01-demo", "grid usage budget clear --task T-01-demo", "Do not run these yourself") {
					if !strings.Contains(got.reason, want) {
						t.Fatalf("reason omits %q: %s", want, got.reason)
					}
				}
				// The session is never told to lift the hold and retry.
				if strings.Contains(got.reason, "start the agent again") {
					t.Fatalf("reason tells the session to start again: %s", got.reason)
				}
				return
			}
			if got.behavior == "deny" || got.stderr != "" {
				t.Fatalf("want allowed, got %+v", got)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestHoldAppliesToTheTaskToolToo(t *testing.T) {
	f := holdSetup(t)
	f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
	if got := f.run(t, "pretooluse", "Task", f.task, nil, agentInput); got.behavior != "deny" {
		t.Fatalf("got %+v", got)
	}
}

// Everything but a PreToolUse start of an agent is what it was without the hold.
func TestHoldLeavesOtherCallsAlone(t *testing.T) {
	atBudget, underBudget := holdSetup(t), holdSetup(t)
	atBudget.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
	underBudget.spend(t, usage.Input{Dollars: ptr(1.0)}, usage.Change{Dollars: []float64{10}})
	for _, tc := range []struct {
		name, event, tool string
		input             map[string]any
	}{
		{"Read", "pretooluse", "Read", map[string]any{"file_path": "main.go"}},
		{"Bash", "pretooluse", "Bash", map[string]any{"command": "ls"}},
		{"Grep", "pretooluse", "Grep", map[string]any{"pattern": "TODO"}},
		{"permission request for Agent", "permissionrequest", "Agent", agentInput},
		{"permission request for Task", "permissionrequest", "Task", agentInput},
		{"permission request for Read", "permissionrequest", "Read", map[string]any{"file_path": "main.go"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Same relative cwd in both homes, so the two outputs are comparable.
			at := atBudget.run(t, tc.event, tc.tool, atBudget.task, nil, tc.input)
			under := underBudget.run(t, tc.event, tc.tool, underBudget.task, nil, tc.input)
			if at.out != under.out || at.stderr != under.stderr {
				t.Fatalf("at the budget %+v differs from under it %+v", at, under)
			}
			if strings.Contains(at.reason, "usage budget") {
				t.Fatalf("hold applied: %+v", at)
			}
		})
	}
}

func TestHoldNeedsABudgetAndATask(t *testing.T) {
	t.Run("no budget", func(t *testing.T) {
		f := holdSetup(t)
		s := usage.Store{TaskPath: f.task, GridHome: f.home}
		if _, err := s.Post(usage.Input{Job: "lead", Dollars: ptr(500.0), Minutes: ptr(500.0), Tokens: ptr(uint64(500))}); err != nil {
			t.Fatal(err)
		}
		if got := f.agent(t); got.behavior == "deny" || got.stderr != "" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("no usage data at all", func(t *testing.T) {
		f := holdSetup(t)
		if got := f.agent(t); got.behavior == "deny" || got.stderr != "" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("a session in no task", func(t *testing.T) {
		f := holdSetup(t)
		f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
		elsewhere := t.TempDir()
		if got := f.run(t, "pretooluse", "Agent", elsewhere, nil, agentInput); got.behavior == "deny" || got.stderr != "" {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("no Grid home", func(t *testing.T) {
		f := holdSetup(t)
		f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
		f.resolver = where.Resolver{}
		if got := f.run(t, "pretooluse", "Agent", f.task, nil, agentInput); got.behavior == "deny" || got.stderr != "" {
			t.Fatalf("got %+v", got)
		}
	})
}

// The limit of this lane: the hold finds a task the way taskDir does. A session
// in a code folder with no GRID_TASK or GRID_FEED_TASK in the hook's environment
// names no task, so it is not held. Widening taskDir is another lane.
func TestHoldDoesNotReachACodeFolderWithoutATaskInTheEnvironment(t *testing.T) {
	f := holdSetup(t)
	f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
	if got := f.run(t, "pretooluse", "Agent", f.code, nil, agentInput); got.behavior == "deny" || got.stderr != "" {
		t.Fatalf("got %+v", got)
	}
}

// One row per way taskDir names a task, each at the budget.
func TestHoldFindsTheTaskTheWayRealSessionsNameIt(t *testing.T) {
	for _, tc := range []struct {
		name string
		cwd  func(*holdFixture) string
		env  func(*holdFixture) map[string]string
	}{
		{"cwd is the task folder", func(f *holdFixture) string { return f.task }, nil},
		{"cwd is inside the task folder", func(f *holdFixture) string { return filepath.Join(f.task, "artifacts") }, nil},
		{"GRID_TASK is the task folder path, cwd in the code folder", func(f *holdFixture) string { return f.code },
			func(f *holdFixture) map[string]string { return map[string]string{"GRID_TASK": f.task} }},
		{"GRID_TASK is the task folder name, cwd in the code folder", func(f *holdFixture) string { return f.code },
			func(*holdFixture) map[string]string { return map[string]string{"GRID_TASK": "T-01-demo"} }},
		{"GRID_TASK is the task id, cwd in the code folder", func(f *holdFixture) string { return f.code },
			func(*holdFixture) map[string]string { return map[string]string{"GRID_TASK": "T-01"} }},
		{"GRID_FEED_TASK is the task folder path, cwd in the code folder", func(f *holdFixture) string { return f.code },
			func(f *holdFixture) map[string]string { return map[string]string{"GRID_FEED_TASK": f.task} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := holdSetup(t)
			f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
			var env map[string]string
			if tc.env != nil {
				env = tc.env(f)
			}
			got := f.run(t, "pretooluse", "Agent", tc.cwd(f), env, agentInput)
			if got.behavior != "deny" || !strings.Contains(got.reason, "dollars: 10 spent of a budget of 10") || got.stderr != "" {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestHoldLogsUnreadableUsageAndDoesNotHold(t *testing.T) {
	f := holdSetup(t)
	f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
	log := filepath.Join(f.task, "usage", "usage.jsonl")
	if err := os.Remove(log); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(log, 0o700); err != nil {
		t.Fatal(err)
	}
	got := f.agent(t)
	if got.behavior == "deny" {
		t.Fatalf("held on unreadable data: %+v", got)
	}
	if !strings.HasPrefix(got.stderr, "grid hook: usage hold: ") || strings.Count(got.stderr, "\n") != 1 {
		t.Fatalf("stderr %q", got.stderr)
	}
}

func TestHoldLogsAStaleTaskNameAndDoesNotHold(t *testing.T) {
	f := holdSetup(t)
	f.spend(t, usage.Input{Dollars: ptr(10.0)}, usage.Change{Dollars: []float64{10}})
	got := f.run(t, "pretooluse", "Agent", f.code, map[string]string{"GRID_TASK": "T-99"}, agentInput)
	if got.behavior == "deny" || !strings.HasPrefix(got.stderr, "grid hook: usage hold: ") {
		t.Fatalf("got %+v", got)
	}
}

