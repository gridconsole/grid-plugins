package guard

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestHostAgentProjectWorkflows(t *testing.T) {
	for _, scope := range []string{"workspace", "project"} {
		t.Run(scope, func(t *testing.T) {
			f := hostSetup(t)
			f.env["GRID_PLAN_GUARD"] = "off"
			dir := f.grid
			if scope == "project" {
				dir = filepath.Join(f.projects, "example")
			}
			path := filepath.Join(dir, "workflows.md")
			f.write(path, "---\nperson_verbs: [signal act]\n---\n")
			alias := filepath.Join(f.repo, "policy alias")
			f.link(alias, path)
			for _, target := range []struct{ name, path string }{{"direct", path}, {"symlink", alias}} {
				t.Run(target.name, func(t *testing.T) {
					for _, tool := range []string{"Write", "Edit", "MultiEdit"} {
						t.Run(tool, func(t *testing.T) {
							in := f.input(tool, target.path)
							in.AgentType = "grid-builder"
							got := EvaluateHost(in, f.root, f.env)
							if got.Behavior != "deny" || got.Reason != fmt.Sprintf(gateReason, target.path) {
								t.Fatalf("want gate denial; behavior=%q", got.Behavior)
							}
						})
					}
				})
			}
		})
	}
}
