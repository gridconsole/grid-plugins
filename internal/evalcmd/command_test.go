package evalcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

func command(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	var out, errout bytes.Buffer
	code := Run(args, Env{Cwd: root, Stdout: &out, Stderr: &errout})
	return code, out.String(), errout.String()
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{
		{"unknown"}, {"list", "extra"}, {"list", "--tier", "a"}, {"list", "--size", "huge"},
		{"run", "--agent", "sh -c true"}, {"run", "--agent", "[]"}, {"run", "--agent", "null"},
		{"run", "--agent", `[""]`}, {"run", "--env", "GRID_HOME"}, {"run", "--env", "CLAUDE_CODE_MESSAGING_TOKEN"}, {"run", "--timeout", "0s"}, {"run", "one", "two"}, {"show"},
	} {
		if code, _, stderr := command(t, args...); code != 2 || stderr == "" {
			t.Errorf("%v: code=%d stderr=%s", args, code, stderr)
		}
	}
	for _, args := range [][]string{nil, {"--help"}, {"run", "--help"}, {"list", "--help"}, {"show", "--help"}} {
		if code, _, stderr := command(t, args...); code != 0 {
			t.Errorf("%v: code=%d stderr=%s", args, code, stderr)
		}
	}
}

func TestSelectionAndRealCommand(t *testing.T) {
	code, out, stderr := command(t, "list", "--tier", "b", "--size", "tiny", "--json")
	var tasks []eval.Task
	if code != 0 || json.Unmarshal([]byte(out), &tasks) != nil || len(tasks) != 1 || tasks[0].ID != "release-notes" {
		t.Fatalf("filter: %d %s %s", code, out, stderr)
	}
	results := t.TempDir()
	code, out, stderr = command(t, "run", "release-notes", "--agent", `["sh","-c","cat >/dev/null; printf '# Release notes\\n- Fix empty search results\\n- Add CSV export\\n' > RELEASE.md"]`, "--results", results, "--json")
	var runs []eval.Result
	if code != 0 || json.Unmarshal([]byte(out), &runs) != nil || len(runs) != 1 || runs[0].Fake {
		t.Fatalf("configured command: %d %s %s", code, out, stderr)
	}
	code, out, stderr = command(t, "show", "--results", results, "--json", runs[0].ID)
	var shown eval.Result
	if code != 0 || json.Unmarshal([]byte(out), &shown) != nil || shown.ID != runs[0].ID {
		t.Fatalf("show JSON: %d %s %s", code, out, stderr)
	}
	if code, _, _ := command(t, "run", "unknown", "--results", results); code != 1 {
		t.Fatalf("missing task: exit %d", code)
	}
}

func TestEnvFlagPassesNamedVariablesOnly(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "key-1")
	t.Setenv("OPENAI_API_KEY", "key-2")
	t.Setenv("GRID_PROJECTS", "/leak")
	var runs []eval.Result
	code, out, stderr := command(t, "run", "release-notes", "--results", t.TempDir(), "--json", "--env", "ANTHROPIC_API_KEY", "--agent", `["sh","-c","cat >/dev/null; env"]`)
	if code != 1 || json.Unmarshal([]byte(out), &runs) != nil || len(runs) != 1 {
		t.Fatalf("run: %d %s %s", code, out, stderr)
	}
	agent := runs[0].AgentResult.Stdout
	if !strings.Contains(agent, "ANTHROPIC_API_KEY=key-1\n") || strings.Contains(agent, "OPENAI_API_KEY") || strings.Contains(agent, "/leak") {
		t.Fatalf("agent environment: %s", agent)
	}
}

func TestSuiteContinuesAfterFailure(t *testing.T) {
	results := t.TempDir()
	code, out, stderr := command(t, "run", "--results", results, "--agent", `["sh","-c","exit 7"]`, "--json")
	var runs []eval.Result
	if code != 1 || json.Unmarshal([]byte(out), &runs) != nil || len(runs) != 3 {
		t.Fatalf("failure: %d %s %s", code, out, stderr)
	}
	for _, run := range runs {
		if run.Status != "failed" {
			t.Fatalf("unexpected run: %+v", run)
		}
		if _, err := eval.LoadResult(results, run.ID); err != nil {
			t.Fatal(err)
		}
	}
	if code, out, stderr := command(t, "show", runs[0].ID, "--results", results); code != 0 || !strings.Contains(out, "exit 7") {
		t.Fatalf("show failed run: %d %s %s", code, out, stderr)
	}
}

func TestDefaultResultsLiveUnderGridHome(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cwd, gridHome := t.TempDir(), t.TempDir()
	runCmd := func(args ...string) (int, string) {
		var out, errout bytes.Buffer
		code := Run(args, Env{Cwd: cwd, GridHome: gridHome, Stdout: &out, Stderr: &errout})
		return code, out.String() + errout.String()
	}
	suite := filepath.Join(root, "evals", "tasks")
	code, out := runCmd("run", "release-notes", "--suite", suite, "--json")
	var runs []eval.Result
	if code != 0 || json.Unmarshal([]byte(out), &runs) != nil || len(runs) != 1 {
		t.Fatalf("run: %d %s", code, out)
	}
	if _, err := eval.LoadResult(filepath.Join(gridHome, "evals", "runs"), runs[0].ID); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(cwd); err != nil || len(entries) != 0 {
		t.Fatalf("results leaked into the working directory: %v %v", entries, err)
	}
	if code, out := runCmd("show", runs[0].ID); code != 0 || !strings.Contains(out, runs[0].ID) {
		t.Fatalf("show: %d %s", code, out)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestOutputError(t *testing.T) {
	var stderr bytes.Buffer
	if code := Run([]string{"list", "--suite", "../../evals/tasks", "--json"}, Env{Stdout: brokenWriter{}, Stderr: &stderr}); code != 1 {
		t.Fatalf("output error ignored: %d", code)
	}
}
