package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func planFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "plan", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func planTask(h *harness) string {
	path := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-07-tag-filter")
	h.write(filepath.Join(path, "plan.md"), planFixture(h.t, "plan.md"))
	return path
}

func TestPlanReadGolden(t *testing.T) {
	for _, action := range []string{"hash", "items"} {
		for _, format := range []string{"plain", "json"} {
			t.Run(action+"/"+format, func(t *testing.T) {
				h := newHarness(t)
				task := planTask(h)
				h.write(filepath.Join(task, "items.md"), planFixture(t, "items.md"))
				h.write(filepath.Join(task, "subagents", "toolbar.md"), "---\nstatus: running\nitem: 2\n---\nBuild the toolbar.\n")
				args := []string{"plan", action}
				if format == "json" {
					args = append(args, "--json")
				}
				if code := h.run(task, args...); code != 0 {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				golden.Assert(t, "plan-cli-"+action+"-"+format, h.out.String())
			})
		}
	}
}

func TestPlanWriteAndDiffGolden(t *testing.T) {
	for _, format := range []string{"plain", "json"} {
		t.Run(format, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			source := filepath.Join(h.tmp, "draft.md")
			draft := planFixture(t, "revised.md")
			h.write(source, draft)
			args := []string{"plan", "write", "draft.md", "--task", task}
			if format == "json" {
				args = append(args, "--json")
			}
			if code := h.run(h.tmp, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, "plan-cli-write-"+format, h.out.String())
			data, err := os.ReadFile(source)
			if err != nil || string(data) != draft {
				t.Fatal("source file changed")
			}
			data, err = os.ReadFile(filepath.Join(task, "plan-revisions", "3.md"))
			if err != nil || string(data) != planFixture(t, "plan.md") {
				t.Fatal("previous revision not preserved")
			}
			args = []string{"plan", "diff", "3", "--task", task, "4"}
			if format == "json" {
				args = append(args, "--json")
			}
			if code := h.run(h.tmp, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, "plan-cli-diff-"+format, h.out.String())
		})
	}
}

func TestPlanTaskResolution(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	nested := filepath.Join(task, "checks", "R-1")
	h.mkdir(nested)
	for _, tc := range []struct {
		cwd  string
		args []string
	}{
		{nested, []string{"plan", "hash", "--json"}},
		{h.tmp, []string{"plan", "hash", "--task", task, "--json"}},
		{task, []string{"plan", "hash", "--task=T-07", "--json"}},
	} {
		if code := h.run(tc.cwd, tc.args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", tc.args, code, h.errb)
		}
		var got struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Hash != plan.Hash(planFixture(t, "plan.md")) {
			t.Fatalf("%s %v", h.out, err)
		}
	}
	if code := h.run(h.tmp, "plan", "hash"); code != 1 || !strings.Contains(h.errb.String(), "--task") {
		t.Fatalf("no task: %d %s", code, h.errb)
	}
	if code := h.run(h.tmp, "plan", "hash", "--task", h.tmp); code != 1 {
		t.Fatalf("outside Grid home: %d", code)
	}
}

func TestPlanUsageAndRefusals(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	for _, args := range [][]string{
		{"plan"}, {"plan", "bogus"}, {"plan", "hash", "extra"}, {"plan", "write"}, {"plan", "diff", "1"}, {"plan", "diff", "1", "2", "3"}, {"plan", "items", "--unknown"}, {"plan", "hash", "--task"},
	} {
		if code := h.run(task, args...); code != 2 {
			t.Errorf("%v: %d %s", args, code, h.errb)
		}
	}
	for _, args := range [][]string{{"plan", "--help"}, {"plan", "diff", "--help"}} {
		if code := h.run(task, args...); code != 0 {
			t.Errorf("help: %d", code)
		}
	}
	h.write(filepath.Join(task, "bad.md"), "# Plan\n- [ ] 1 No recall citation\n")
	if code := h.run(task, "plan", "write", "bad.md"); code != 1 || !strings.Contains(h.errb.String(), "Used:") {
		t.Fatalf("missing recall: %d %s", code, h.errb)
	}
	if code := h.run(task, "plan", "write", "missing.md"); code != 1 {
		t.Fatalf("missing input: %d", code)
	}
	h.write(filepath.Join(task, "-draft.md"), "# Plan\nRecall: nothing found\n- [ ] 1 First\n")
	if code := h.run(task, "plan", "write", "--", "-draft.md"); code != 0 {
		t.Fatalf("literal filename: %d %s", code, h.errb)
	}
	if code := h.run(task, "plan", "diff", "3", "99"); code != 1 {
		t.Fatalf("missing revision: %d", code)
	}
}
