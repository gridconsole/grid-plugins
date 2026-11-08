package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestHookPersonalScope(t *testing.T) {
	for _, event := range []string{"PreToolUse", "PermissionRequest", "permission-forward"} {
		for _, role := range []string{"guest", "grid-guest", "unknown", "grid-unknown", "external", "member", "owner", "worker", "grid-worker"} {
			for _, target := range []string{"personal/tasks/T-01-private/task.md", "personal/new.md", "projects/alpha/notes.md", "projects/beta/notes.md", "notes.md", "."} {
				t.Run(event+"/"+role+"/"+target, func(t *testing.T) {
					h, cwd := settingsAccessFixture(t, role)
					h.record(project.Record{Name: "beta", ID: "01BETA"})
					h.write(filepath.Join(h.gridHome, "personal", "tasks", "T-01-private", "task.md"), "private")
					path := filepath.Join(h.gridHome, filepath.FromSlash(target))
					b, err := json.Marshal(hookio.Input{Cwd: cwd, ToolName: "Read", ToolInput: map[string]any{"file_path": path}})
					if err != nil {
						t.Fatal(err)
					}
					h.env.Stdin = strings.NewReader(string(b))
					if code := h.run(cwd, "hook", event); code != 0 {
						t.Fatal(code)
					}
					var reply struct {
						Output struct {
							Behavior string `json:"permissionDecision"`
							Decision struct {
								Behavior    string `json:"behavior"`
								Permissions []struct {
									Directories []string `json:"directories"`
								} `json:"updatedPermissions"`
							} `json:"decision"`
						} `json:"hookSpecificOutput"`
					}
					if h.out.Len() > 0 {
						if err := json.Unmarshal(h.out.Bytes(), &reply); err != nil {
							t.Fatal(err)
						}
					}
					behavior := reply.Output.Behavior
					if event != "PreToolUse" {
						behavior = reply.Output.Decision.Behavior
					}
					full := role == "owner" || role == "worker" || role == "grid-worker"
					want := full || (role == "member" && target != "." && !strings.HasPrefix(target, "personal/")) ||
						((role == "guest" || role == "grid-guest" || role == "external") && target == "projects/alpha/notes.md")
					if (behavior == "allow") != want {
						t.Fatalf("allow=%t, want %t: %s", behavior == "allow", want, h.out)
					}
					for _, perm := range reply.Output.Decision.Permissions {
						for _, dir := range perm.Directories {
							expected := h.gridHome
							if !full {
								expected = filepath.Join(h.gridHome, "projects", "alpha")
							}
							if dir != expected {
								t.Fatalf("directory %q, want %q", dir, expected)
							}
						}
					}
					if role == "member" && len(reply.Output.Decision.Permissions) != 0 {
						t.Fatalf("member received directory permissions: %s", h.out)
					}
					if want && role != "member" && event != "PreToolUse" && len(reply.Output.Decision.Permissions) != 1 {
						t.Fatalf("missing scoped permission: %s", h.out)
					}
				})
			}
		}
	}
}
