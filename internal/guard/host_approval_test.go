package guard

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostApprovalReceipts(t *testing.T) {
	f := setup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, name := range []string{"approval.json", "approval-ship.json", "APPROVAL-SHIP.JSON"} {
		t.Run(name, func(t *testing.T) {
			for _, dir := range []string{f.task, filepath.Join(f.grid, "personal", "tasks", "T-02-mine")} {
				for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
					t.Run(tool+"/"+filepath.Base(dir), func(t *testing.T) {
						i := f.input(tool, filepath.Join(dir, name))
						i.AgentType = "grid-builder"
						expect(t, EvaluateHost(i, f.root, f.env), "deny")
					})
				}
			}
			for _, path := range []string{name, filepath.ToSlash(filepath.Join(f.task, name))} {
				for _, form := range []string{
					`echo '{}' > "%s"`, `echo '{}' >> "%s"`, `echo '{}' | tee "%s"`,
					`cp /tmp/receipt "%s"`, `mv /tmp/receipt "%s"`, `sed -i s/no/yes/ "%s"`,
					`python -c "open('%s', 'w').write('{}')"`,
				} {
					command := fmt.Sprintf(form, path)
					t.Run(command, func(t *testing.T) {
						i := f.input("Bash", command)
						i.AgentType = "grid-builder"
						expect(t, EvaluateHost(i, f.root, f.env), "deny")
					})
				}
				for _, command := range []string{"cat " + path, "jq . " + path, "ls -l " + path} {
					expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "")
				}
			}
			for _, lookalike := range []string{name + ".bak", "my-" + name, strings.TrimSuffix(name, ".json") + ".jsonl"} {
				expect(t, EvaluateHost(f.input("Write", filepath.Join(f.task, lookalike)), f.root, f.env), "")
				expect(t, EvaluateHost(f.input("Bash", "echo x > "+lookalike), f.root, f.env), "")
			}
			expect(t, EvaluateHost(f.input("Write", filepath.Join(f.task, "artifacts", name)), f.root, f.env), "")
		})
	}
}

func TestHostProtectsTaskWorkflow(t *testing.T) {
	f := setup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
		t.Run(tool, func(t *testing.T) {
			i := f.input(tool, filepath.Join(f.task, "task.md"))
			i.AgentType = "grid-builder"
			i.ToolInput["content"] = "---\nworkflow: [{step: build, kind: run}]\n---\n"
			i.ToolInput["old_string"] = "kind: approve"
			i.ToolInput["new_string"] = "kind: run"
			i.ToolInput["edits"] = []any{map[string]any{"old_string": "kind: approve", "new_string": "kind: run"}}
			expect(t, EvaluateHost(i, f.root, f.env), "deny")
		})
	}
	for _, command := range []string{"sed -i s/approve/run/ task.md", "echo changed > task.md", "rm task.md"} {
		t.Run(command, func(t *testing.T) {
			expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "deny")
		})
	}
	expect(t, EvaluateHost(f.input("Bash", "cat task.md"), f.root, f.env), "")
	expect(t, EvaluateHost(f.input("Write", filepath.Join(f.repo, "task.md")), f.root, f.env), "")
	expect(t, EvaluateHost(f.input("Write", filepath.Join(f.grid, "personal", "tasks", "T-02-mine", "TASK.MD")), f.root, f.env), "deny")
}
