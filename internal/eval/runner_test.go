package eval

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testRunner(t *testing.T) Runner {
	t.Helper()
	return Runner{ResultsDir: t.TempDir(), TempDir: t.TempDir(), Timeout: 3 * time.Second}
}

func runTask(t *testing.T, runner Runner, task Task) Result {
	t.Helper()
	result, err := runner.Run(context.Background(), task)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := LoadResult(runner.ResultsDir, result.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, result) {
		t.Fatalf("saved result differs: %+v != %+v", saved, result)
	}
	entries, err := os.ReadDir(runner.TempDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary workspaces were not cleaned up: %v, %v", entries, err)
	}
	return result
}

func TestFakeAgentSamples(t *testing.T) {
	tasks, err := LoadSuite("../../evals/tasks")
	if err != nil {
		t.Fatal(err)
	}
	runner := testRunner(t)
	var results []Result
	for _, task := range tasks {
		t.Run(task.ID, func(t *testing.T) {
			r := runTask(t, runner, task)
			if r.Status != "passed" || !r.Fake || r.Summary.Passed != len(task.Checks) || r.Summary.CheckScore == nil || *r.Summary.CheckScore != 5 {
				t.Fatalf("fake run failed: %+v", r)
			}
			if r.Summary.GatesPassed != 1 || r.Summary.GatesTotal != 1 || len(r.ConfigHash) != 64 {
				t.Fatalf("missing metadata: %+v", r)
			}
			results = append(results, r)
		})
	}
	var table bytes.Buffer
	if err := WriteSummary(&table, results); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"CHECK SCORE", "5.00/5", "fake", "clean-csv", "fix-total", "release-notes"} {
		if !strings.Contains(table.String(), text) {
			t.Errorf("summary missing %q: %s", text, &table)
		}
	}
}

func TestFreshWorkspaceAndPrompt(t *testing.T) {
	task := sampleTask(t)
	task.Prompt = "A prompt with spaces, 'quotes', $(exit 99) and\nnewlines."
	task.Setup = `test ! -e previous && test ! -e "$HOME/previous" && test -d "$GRID_HOME" && printf 'prepared\n' > seed`
	task.Checks = []Check{
		{ID: "stdin", Type: "text_contains", Path: "stdin.txt", Text: task.Prompt},
		{ID: "argv", Type: "text_contains", Path: "argv.txt", Text: task.Prompt},
		{ID: "seed", Type: "text_contains", Path: "seed", Text: "prepared"},
	}
	runner := testRunner(t)
	runner.Agent = []string{"sh", "-c", `cat > stdin.txt; printf '%s' "$1" > argv.txt; pwd; printf '%s\n' "$HOME"; touch previous "$HOME/previous"`, "agent", "{prompt}"}
	first := runTask(t, runner, task)
	second := runTask(t, runner, task)
	if first.Status != "passed" || second.Status != "passed" || first.Fake || first.ID == second.ID || first.ConfigHash != second.ConfigHash {
		t.Fatalf("fresh runs failed: %+v, %+v", first, second)
	}
	if first.AgentResult.Stdout == second.AgentResult.Stdout {
		t.Fatal("workspace and home were reused")
	}
	for _, path := range strings.Fields(first.AgentResult.Stdout + second.AgentResult.Stdout) {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("temporary path survived: %s (%v)", path, err)
		}
	}
}

func TestFailuresAreSaved(t *testing.T) {
	for _, tc := range []struct {
		name, setup, agent, status, phase string
	}{
		{"setup", "echo setup-failed >&2; exit 7", "exit 0", "failed", "setup"},
		{"agent", "true", "echo agent-failed >&2; exit 9", "failed", "agent"},
		{"timeout", "true", "sleep 20", "error", "agent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task, runner := sampleTask(t), testRunner(t)
			task.Setup = tc.setup
			runner.Agent = []string{"sh", "-c", tc.agent}
			if tc.name == "timeout" {
				runner.Timeout = 100 * time.Millisecond
			}
			start := time.Now()
			r := runTask(t, runner, task)
			if r.Status != tc.status || r.Summary.Skipped != len(task.Checks) || r.Summary.CheckScore != nil {
				t.Fatalf("bad failure record: %+v", r)
			}
			if tc.phase == "setup" && (r.Setup.ExitCode != 7 || r.AgentResult.Status != "skipped") {
				t.Fatalf("agent ran after setup failed: %+v", r)
			}
			if tc.name == "timeout" && (time.Since(start) > 2*time.Second || !strings.Contains(r.AgentResult.Error, "deadline")) {
				t.Fatalf("timeout did not stop the command: %+v", r)
			}
		})
	}
}

func TestCommandNotFoundAndCancellation(t *testing.T) {
	runner, task := testRunner(t), sampleTask(t)
	runner.Agent = []string{"grid-eval-nonexistent-executable"}
	r := runTask(t, runner, task)
	if r.Status != "error" || r.AgentResult.ExitCode != -1 {
		t.Fatalf("missing agent was not an error: %+v", r)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err := runner.Run(ctx, task)
	if err != nil || r.Status != "error" || r.Setup.Error != "context canceled" {
		t.Fatalf("cancellation: %+v, %v", r, err)
	}
	if _, err := LoadResult(runner.ResultsDir, r.ID); err != nil {
		t.Fatal(err)
	}
}

func TestChecksAndWeightedScore(t *testing.T) {
	task, runner := sampleTask(t), testRunner(t)
	task.Setup = "printf 'first\\nneedle\\n' > output.txt; mkdir directory"
	runner.Agent = []string{"sh", "-c", "true"}
	task.Checks = []Check{
		{ID: "gate", Type: "file_exists", Path: "missing", Gate: true},
		{ID: "found", Type: "text_contains", Path: "output.txt", Text: "needle", Weight: 3},
		{ID: "missing", Type: "text_contains", Path: "output.txt", Text: "absent"},
		{ID: "command", Type: "command_passes", Command: []string{"sh", "-c", "echo evidence >&2; exit 4"}},
		{ID: "directory", Type: "file_exists", Path: "directory", Gate: true},
	}
	r := runTask(t, runner, task)
	if r.Status != "failed" || r.Summary.Passed != 1 || r.Summary.Failed != 4 || r.Summary.GatesPassed != 0 || *r.Summary.CheckScore != 3 {
		t.Fatalf("incorrect scoring: %+v", r)
	}
	if r.Checks[1].Evidence != "output.txt:2" || r.Checks[3].Process.ExitCode != 4 || r.Checks[3].Process.Stderr != "evidence\n" {
		t.Fatalf("missing evidence: %+v", r.Checks)
	}
}

func TestBoundedOutputAndConfinedFiles(t *testing.T) {
	task, runner := sampleTask(t), testRunner(t)
	task.Setup = "ln -s ../home outside; printf secret > ../home/value; head -c 1048577 /dev/zero > huge"
	runner.Agent = []string{"sh", "-c", "head -c 100000 /dev/zero; head -c 100000 /dev/zero >&2"}
	task.Checks = []Check{
		{ID: "symlink", Type: "text_contains", Path: "outside/value", Text: "secret"},
		{ID: "huge", Type: "text_contains", Path: "huge", Text: "absent"},
	}
	r := runTask(t, runner, task)
	if r.Status != "error" || r.Summary.Errors != 2 || r.Summary.CheckScore != nil {
		t.Fatalf("unsafe checks accepted: %+v", r)
	}
	if !r.AgentResult.Truncated || len(r.AgentResult.Stdout) != outputLimit || len(r.AgentResult.Stderr) != outputLimit {
		t.Fatalf("output was not bounded: %d, %d", len(r.AgentResult.Stdout), len(r.AgentResult.Stderr))
	}
}

type judgeFunc func(context.Context, JudgeRequest) (Judgement, error)

func (f judgeFunc) Evaluate(ctx context.Context, req JudgeRequest) (Judgement, error) {
	return f(ctx, req)
}

func TestJudgeHook(t *testing.T) {
	for _, mode := range []string{"missing", "success", "failure", "error", "invalid", "nan"} {
		t.Run(mode, func(t *testing.T) {
			task, runner := sampleTask(t), testRunner(t)
			task.Checks = []Check{{ID: "quality", Type: "llm_judge", Rubric: "S3: accurate CSV"}}
			if mode != "missing" {
				runner.Judge = judgeFunc(func(ctx context.Context, req JudgeRequest) (Judgement, error) {
					if req.Task.ID != task.ID || req.Check.ID != "quality" || ctx.Err() != nil {
						t.Fatalf("bad judge request: %+v", req)
					}
					if _, err := os.Stat(filepath.Join(req.Workspace, "clean.csv")); err != nil {
						t.Fatal(err)
					}
					j := Judgement{Passed: true, Score: 4.5, Reason: "accurate", Evidence: "clean.csv:2"}
					switch mode {
					case "failure":
						j.Passed, j.Score = false, 2
					case "error":
						return j, errors.New("judge unavailable")
					case "invalid":
						j.Score = 6
					case "nan":
						j.Score = math.NaN()
					}
					return j, nil
				})
			}
			r := runTask(t, runner, task)
			want := "error"
			if mode == "success" {
				want = "passed"
			} else if mode == "failure" {
				want = "failed"
			}
			if r.Status != want || r.Checks[0].Scorer != "judge" {
				t.Fatalf("judge result: %+v", r)
			}
			if mode == "success" && (*r.Summary.CheckScore != 4.5 || r.Checks[0].Evidence != "clean.csv:2") {
				t.Fatalf("judge score lost: %+v", r)
			}
		})
	}
}

func TestLoadResultIgnoresUnknownFields(t *testing.T) {
	runner := testRunner(t)
	result := runTask(t, runner, sampleTask(t))
	path := filepath.Join(runner.ResultsDir, result.ID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	newer := strings.Replace(string(data), `"checks":`, `"added_later": {"x": 1},`+"\n  "+`"checks":`, 1)
	newer = strings.Replace(newer, `"exit_code":`, `"future_field": true, "exit_code":`, 1)
	if newer == string(data) || os.WriteFile(path, []byte(newer), 0o600) != nil {
		t.Fatal("could not write the newer result")
	}
	loaded, err := LoadResult(runner.ResultsDir, result.ID)
	if err != nil || !reflect.DeepEqual(loaded, result) {
		t.Fatalf("newer result not readable: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(newer, `"schema_version": 1`, `"schema_version": 2`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadResult(runner.ResultsDir, result.ID); err == nil {
		t.Fatal("a result with another schema version was accepted")
	}
}

func TestResultValidationAndWriteFailure(t *testing.T) {
	runner := testRunner(t)
	if _, err := LoadResult(runner.ResultsDir, "../escape"); err == nil {
		t.Fatal("unsafe id accepted")
	}
	file := filepath.Join(runner.ResultsDir, "not-a-directory")
	if err := os.WriteFile(file, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner.ResultsDir = file
	if _, err := runner.Run(context.Background(), sampleTask(t)); err == nil {
		t.Fatal("result write failure ignored")
	}
}

func envLines(out string) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			env[key] = value
		}
	}
	return env
}

func TestAgentDoesNotInheritSessionEnvironment(t *testing.T) {
	leaked := []string{
		"GRID_TASK", "GRID_PROJECTS", "GRID_HOME", "GRID_ANYTHING", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_MESSAGING_TOKEN",
		"CLAUDE_CODE_MESSAGING_SOCKET", "CLAUDE_CODE_CHILD_SESSION", "CLAUDE_ENV_FILE", "CLAUDE_PLUGIN_ROOT",
		"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "SSH_AUTH_SOCK", "GITHUB_TOKEN",
	}
	for _, name := range leaked {
		t.Setenv(name, "/leak/"+name)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	t.Setenv("LC_ALL", "en_US.UTF-8")
	t.Setenv("TERM", "xterm")

	task, runner := sampleTask(t), testRunner(t)
	task.Setup = `env > setup-env.txt`
	task.Checks = []Check{{ID: "check-env", Type: "command_passes", Command: []string{"sh", "-c", "env >&2"}}}
	runner.Agent = []string{"sh", "-c", "cat setup-env.txt >&2; env"}
	assertClean := func(phase, out string) {
		t.Helper()
		env := envLines(out)
		for _, name := range leaked {
			if value, ok := env[name]; ok && name != "GRID_HOME" && name != "GRID_PROJECTS" {
				t.Errorf("%s inherited %s=%s", phase, name, value)
			}
		}
		for _, name := range []string{"HOME", "TMPDIR", "GRID_HOME", "GRID_PROJECTS", "XDG_CONFIG_HOME"} {
			if !strings.HasPrefix(env[name], runner.TempDir) {
				t.Errorf("%s: %s=%q is not inside the attempt's temp folder %s", phase, name, env[name], runner.TempDir)
			}
		}
		if env["PATH"] != os.Getenv("PATH") || env["LANG"] != "en_US.UTF-8" || env["LC_ALL"] != "en_US.UTF-8" || env["TERM"] != "xterm" {
			t.Errorf("%s lost an allowed variable: %v", phase, env)
		}
		for name := range env {
			switch {
			case strings.HasPrefix(name, "GRID_"), strings.HasPrefix(name, "CLAUDE"):
				if name != "GRID_HOME" && name != "GRID_PROJECTS" && name != "GRID_EVAL_TASK" {
					t.Errorf("%s has unexpected %s", phase, name)
				}
			}
		}
	}
	r := runTask(t, runner, task)
	assertClean("setup", r.AgentResult.Stderr)
	assertClean("agent", r.AgentResult.Stdout)
	assertClean("check", r.Checks[0].Process.Stderr)
}

func TestAllowEnvPassesOnlyNamedVariables(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "key-1")
	t.Setenv("OPENAI_API_KEY", "key-2")
	task, runner := sampleTask(t), testRunner(t)
	runner.Agent = []string{"sh", "-c", "env"}
	if env := envLines(runTask(t, runner, task).AgentResult.Stdout); env["ANTHROPIC_API_KEY"] != "" || env["OPENAI_API_KEY"] != "" {
		t.Fatalf("login variables passed without being allowed: %v", env)
	}
	runner.AllowEnv = []string{"ANTHROPIC_API_KEY", "CODEX_HOME_NOT_SET"}
	env := envLines(runTask(t, runner, task).AgentResult.Stdout)
	if env["ANTHROPIC_API_KEY"] != "key-1" || env["OPENAI_API_KEY"] != "" {
		t.Fatalf("allow list not honoured: %v", env)
	}
	if _, ok := env["CODEX_HOME_NOT_SET"]; ok {
		t.Fatal("an unset variable was invented")
	}
}

func TestAllowEnvRejectsSessionVariables(t *testing.T) {
	for _, name := range []string{
		"GRID_HOME", "GRID_PROJECTS", "CLAUDE_CODE_SESSION_ID", "CLAUDE_CODE_MESSAGING_TOKEN", "CLAUDE_ENV_FILE",
		"CLAUDE_PLUGIN_ROOT", "SLACK_MESSAGING_TOKEN", "HOME", "TMPDIR", "", "A=B", "has space",
	} {
		if err := ValidateAllowEnv([]string{"ANTHROPIC_API_KEY", name}); err == nil {
			t.Errorf("%q accepted", name)
		}
		runner := testRunner(t)
		runner.AllowEnv = []string{name}
		if _, err := runner.Run(context.Background(), sampleTask(t)); err == nil {
			t.Errorf("Run accepted %q", name)
		}
	}
	if err := ValidateAllowEnv([]string{"ANTHROPIC_API_KEY", "CODEX_HOME", "OPENAI_API_KEY"}); err != nil {
		t.Fatal(err)
	}
}
