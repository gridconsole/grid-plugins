package guard

import (
	"path/filepath"
	"testing"
)

func TestHostAgentWorkflowFileTools(t *testing.T) {
	f := hostSetup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
		for _, path := range []string{
			"workflow.md", "./WORKFLOW.MD",
			filepath.Join(f.task, "workflow.md"),
			filepath.Join(f.grid, "personal", "tasks", "T-02-example", "workflow.md"),
		} {
			in := f.input(tool, path)
			in.AgentType = "grid-builder"
			expect(t, EvaluateHost(in, f.root, f.env), "deny")
		}
		for _, path := range []string{"workflow.md.bak", "my-workflow.md", "workflow.md-notes", filepath.Join(f.repo, "workflow.md")} {
			expect(t, EvaluateHost(f.input(tool, path), f.root, f.env), "")
		}
	}
	for _, tool := range []string{"Read", "Grep", "Glob"} {
		expect(t, EvaluateHost(f.input(tool, "workflow.md"), f.root, f.env), "")
	}
	alias := filepath.Join(f.repo, "policy-link")
	f.write(filepath.Join(f.task, "workflow.md"), "---\nperson_verbs: 5\n---\n")
	f.link(alias, filepath.Join(f.task, "workflow.md"))
	expect(t, EvaluateHost(f.input("Write", alias), f.root, f.env), "deny")
}
