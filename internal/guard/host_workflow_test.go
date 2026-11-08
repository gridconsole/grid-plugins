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
