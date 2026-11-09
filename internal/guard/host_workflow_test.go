package guard

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestHostAgentWorkflowShell(t *testing.T) {
	f := setup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, path := range []string{
		"workflow.md", "./workflow.md", "WORKFLOW.MD",
		filepath.ToSlash(filepath.Join(f.task, "workflow.md")),
		filepath.ToSlash(filepath.Join(f.grid, "personal", "tasks", "T-02-mine", "workflow.md")),
	} {
		for _, form := range []string{
			"echo x > %s", "echo x >> %s", "echo x | tee %s",
			"cp /tmp/policy %s", "mv /tmp/policy %s", "sed -i s/owner/agent/ %s",
			`python -c "open('%s','w').write('x')"`,
		} {
			command := fmt.Sprintf(form, path)
			t.Run(command, func(t *testing.T) {
				in := f.input("Bash", command)
				in.AgentType = "grid-builder"
				expect(t, EvaluateHost(in, f.root, f.env), "deny")
			})
		}
		for _, form := range []string{"cat %s", "head %s", "grep gate_post_roles %s", "ls -l %s"} {
			command := fmt.Sprintf(form, path)
			t.Run(command, func(t *testing.T) {
				in := f.input("Bash", command)
				in.AgentType = "grid-builder"
				expect(t, EvaluateHost(in, f.root, f.env), "")
			})
		}
	}
	t.Run("quoted path", func(t *testing.T) {
		in := f.input("Bash", "echo x > 'work''flow.md'")
		in.AgentType = "grid-builder"
		expect(t, EvaluateHost(in, f.root, f.env), "deny")
	})
	for _, command := range []string{"echo x > workflow.md.bak", "echo x > my-workflow.md", "echo x > workflow.md-notes"} {
		t.Run(command, func(t *testing.T) {
			expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "")
		})
	}
}

func TestHostAgentWorkflowFileTools(t *testing.T) {
	f := setup(t)
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
