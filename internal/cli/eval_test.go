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
	if err := json.Unmarshal(h.out.Bytes(), &tasks); err != nil || len(tasks) != 3 {
		t.Fatalf("list: %s %v", h.out, err)
	}
	results := t.TempDir()
	if code := h.run(root, "eval", "run", "--results", results, "--json"); code != 0 {
		t.Fatalf("run: %d %s %s", code, h.errb, h.out)
	}
	var runs []eval.Result
	if err := json.Unmarshal(h.out.Bytes(), &runs); err != nil || len(runs) != 3 {
		t.Fatalf("run: %s %v", h.out, err)
	}
	for _, run := range runs {
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
