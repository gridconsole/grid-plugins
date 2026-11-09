package guard

import (
	"fmt"
	"path/filepath"
	"testing"
)

// The preview receipt is a gate file like the plan and ship receipts: an agent
// cannot write it, and reading it stays allowed.
func TestHostPreviewApprovalReceipt(t *testing.T) {
	f := setup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	for _, name := range []string{"approval-preview.json", "APPROVAL-PREVIEW.JSON"} {
		t.Run(name, func(t *testing.T) {
			for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
				t.Run(tool, func(t *testing.T) {
					i := f.input(tool, filepath.Join(f.task, name))
					i.AgentType = "grid-builder"
					expect(t, EvaluateHost(i, f.root, f.env), "deny")
				})
			}
			for _, path := range []string{name, filepath.ToSlash(filepath.Join(f.task, name))} {
				for _, form := range []string{`echo '{}' > "%s"`, `cp /tmp/receipt "%s"`, `sed -i s/no/yes/ "%s"`} {
					command := fmt.Sprintf(form, path)
					t.Run(command, func(t *testing.T) {
						i := f.input("Bash", command)
						i.AgentType = "grid-builder"
						expect(t, EvaluateHost(i, f.root, f.env), "deny")
					})
				}
				for _, command := range []string{"cat " + path, "jq . " + path} {
					expect(t, EvaluateHost(f.input("Bash", command), f.root, f.env), "")
				}
			}
			expect(t, EvaluateHost(f.input("Write", filepath.Join(f.task, "artifacts", name)), f.root, f.env), "")
			expect(t, EvaluateHost(f.input("Write", filepath.Join(f.task, "my-"+name)), f.root, f.env), "")
		})
	}
}
