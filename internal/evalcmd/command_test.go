package evalcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

func command(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
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
		{"run", "--agent", `[""]`}, {"run", "--timeout", "0s"}, {"run", "one", "two"}, {"show"},
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

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("broken output") }

func TestOutputError(t *testing.T) {
	var stderr bytes.Buffer
	if code := Run([]string{"list", "--suite", "../../evals/tasks", "--json"}, Env{Stdout: brokenWriter{}, Stderr: &stderr}); code != 1 {
		t.Fatalf("output error ignored: %d", code)
	}
}
