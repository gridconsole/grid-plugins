package guard

import (
	"path/filepath"
	"testing"
)

func TestTaskFileGateProtection(t *testing.T) {
	f := hostSetup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, dir := range []string{filepath.Join(f.projects, "example", "tasks", "T-01-example"), filepath.Join(f.grid, "personal", "tasks", "T-01-example")} {
		for _, file := range []string{"approval.json", "refusal-R-1.json", ".task-write.json", ".task.lock", "APPROVAL.JSON"} {
			for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
				expect(t, EvaluateHost(f.input(tool, filepath.Join(dir, file)), f.root, f.env), "deny")
			}
		}
		expect(t, EvaluateHost(f.input("Write", filepath.Join(dir, "artifacts", "approval.json")), f.root, f.env), "")
	}
}
