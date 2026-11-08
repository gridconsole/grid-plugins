package job_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func TestLoadChecklistPlan(t *testing.T) {
	data, err := os.ReadFile("../../testdata/job/task/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	h := jobtest.New(t, testdata())
	dir := h.Task
	path := filepath.Join(dir, "plan.md")
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	base := string(data)
	write(base)
	for _, selector := range []string{"1", "J-1"} {
		j, err := job.LoadPlan(dir, selector)
		if err != nil {
			t.Fatal(err)
		}
		if j.Item != 1 || j.Title != "Document the export format" || j.Intent.Item != "plan.md#step-1" || j.Intent.Goal != "plan.md#verify" || j.Kind != "non-code" {
			t.Fatalf("%+v", j)
		}
	}
	fs := job.Files{Task: dir}
	source, err := fs.Resolve("plan.md#step-1")
	if err != nil || source.Text != "- [ ] 1 Document the export format\n  Document the export format and column order." {
		t.Fatalf("%+v %v", source, err)
	}
	source, err = fs.Resolve("plan.md#verify")
	if err != nil || source.Text != "The export documentation lists the column order." {
		t.Fatalf("%+v %v", source, err)
	}
	start := strings.Index(base, "    - id:")
	end := strings.Index(base, "\n\n## Verify")
	second := strings.Replace(base[start:end], "id: J-1", "id: J-2", 1)
	write(base[:end] + "\n" + second + base[end:])
	if _, err := job.LoadPlan(dir, "1"); err == nil || !strings.Contains(err.Error(), "found 2") {
		t.Fatalf("ambiguous item: %v", err)
	}
	if j, err := job.LoadPlan(dir, "J-2"); err != nil || j.Item != 1 {
		t.Fatalf("%+v %v", j, err)
	}
	write(strings.Replace(base, "      skills: []\n", "", 1))
	if _, err := job.LoadPlan(dir, "J-1"); err == nil || !strings.Contains(err.Error(), "missing required field skills") {
		t.Fatalf("missing field: %v", err)
	}
	write("---\nitems: [{id: J-1, item: 1}]\n---\n# Legacy plan\n")
	if _, err := job.LoadPlan(dir, "J-1"); err == nil {
		t.Fatal("read the old front-matter form")
	}
}
