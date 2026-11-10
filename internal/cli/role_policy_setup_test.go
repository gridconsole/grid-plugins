package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// The setup preview is not exempt from a workflow's role list: with
// person_verbs: {setup: [owner]} a role that is not listed is refused before
// the preview is printed, and a listed role sees the preview. The policy is
// read from the project's workflows.md or from the task's workflow.md, as for
// every other verb. The preview writes nothing, so the refusal is the only
// visible difference.
func TestSetupPreviewHonoursWorkflowRoleList(t *testing.T) {
	for _, scope := range []string{"project", "task"} {
		for _, tc := range []struct {
			name, role, policy string
			json               bool
			code               int
		}{
			{"member_not_listed", "member", "owner", false, 3},
			{"default_role_is_member", "", "owner", false, 3},
			{"lead_not_listed", "lead", "owner", false, 3},
			{"not_listed_json", "member", "owner", true, 3},
			{"owner_listed", "owner", "owner", false, 0},
			{"member_listed", "member", "owner, member", false, 0},
			{"no_policy", "member", "", false, 0},
		} {
			t.Run(scope+"/"+tc.name, func(t *testing.T) {
				h, task := feedHarness(t)
				projectDir := filepath.Join(h.gridHome, "projects", "example")
				dir, file := projectDir, filepath.Join(projectDir, "workflows.md")
				if scope == "task" {
					dir, file = task, filepath.Join(task, "workflow.md")
				} else {
					// The project's folder is the caller's folder, so the project resolves.
					h.record(project.Record{Name: "example", ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Folders: []project.Folder{{Machine: feedMachine, Path: h.root}}})
					dir = h.root
				}
				if tc.policy != "" {
					h.write(file, "---\nperson_verbs: {setup: ["+tc.policy+"]}\n---\n")
				}
				config := filepath.Join(h.home, "custom-claude")
				t.Setenv("HOME", h.home)
				t.Setenv("CLAUDE_CONFIG_DIR", config)
				t.Setenv("GRID_PROJECTS", filepath.Join(h.gridHome, "projects"))
				t.Setenv("GRID_ROLE", tc.role)
				args := []string{"setup"}
				if tc.json {
					args = append(args, "--json")
				}
				code := h.run(dir, args...)
				if code != tc.code {
					t.Fatalf("setup exit %d, want %d: %s %s", code, tc.code, h.out, h.errb)
				}
				if _, err := os.Stat(filepath.Join(config, "settings.json")); !os.IsNotExist(err) {
					t.Fatal("the preview wrote settings", err)
				}
				if tc.code == 0 {
					if !strings.Contains(h.out.String(), "+ Bash(grid *)") || !strings.Contains(h.out.String(), "Nothing written.") {
						t.Fatalf("no preview for an allowed role: %s %s", h.out, h.errb)
					}
					return
				}
				if !strings.Contains(h.errb.String(), "grid setup:") || !strings.Contains(h.errb.String(), "role_not_allowed") || strings.Contains(h.out.String(), "Bash(grid *)") {
					t.Fatalf("refusal text or preview leak: out=%q err=%q", h.out, h.errb)
				}
				if tc.json && !strings.Contains(h.out.String(), `"error":"person_required"`) {
					t.Fatalf("json refusal without the person_required code: %s", h.out)
				}
			})
		}
	}
}
