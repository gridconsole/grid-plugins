package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

func TestEvalCommands(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	h := newHarness(t)
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(root, "eval", "list", "--json"); code != 0 {
		t.Fatalf("list: %d %s", code, h.errb)
	}
	var tasks []eval.Task
	if err := json.Unmarshal(h.out.Bytes(), &tasks); err != nil {
		t.Fatalf("list: %s %v", h.out, err)
	}
	// The suite also holds ported cases, so select the samples by id instead of counting.
	listed := map[string]bool{}
	for _, task := range tasks {
		listed[task.ID] = true
	}
	// The fake agent only supports the three samples, so run each by id.
	results := t.TempDir()
	for _, id := range []string{"clean-csv", "fix-total", "release-notes"} {
		if !listed[id] {
			t.Fatalf("list: missing sample %s in %s", id, h.out)
		}
		if code := h.run(root, "eval", "run", id, "--results", results, "--json"); code != 0 {
			t.Fatalf("run %s: %d %s %s", id, code, h.errb, h.out)
		}
		var runs []eval.Result
		if err := json.Unmarshal(h.out.Bytes(), &runs); err != nil || len(runs) != 1 {
			t.Fatalf("run %s: %s %v", id, h.out, err)
		}
		run := runs[0]
		if run.Status != "passed" {
			t.Fatalf("sample failed: %+v", run)
		}
		if code := h.run(root, "eval", "show", run.ID, "--results", results); code != 0 || !strings.Contains(h.out.String(), "CHECK SCORE") || !strings.Contains(h.out.String(), "SCORER") {
			t.Fatalf("show: %d %s %s", code, h.errb, h.out)
		}
	}
	if code := h.run(root, "help"); code != 0 || !strings.Contains(h.out.String(), "list, run and show task evaluations") {
		t.Fatalf("registration: %d %s", code, h.out)
	}
}
