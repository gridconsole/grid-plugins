package guard

import (
	"path/filepath"
	"testing"
)

func TestBrainWritePluginAgents(t *testing.T) {
	for _, agent := range []struct {
		name  string
		allow bool
	}{
		{"grid:grid-curator", true}, {"grid-curator", true},
		{"grid:grid-planner", true}, {"grid-planner", true},
		{"curator", false}, {"planner", false},
		{"other:curator", false}, {"other:planner", false},
		{"other:grid-curator", false}, {"other:grid-planner", false},
		{"grid:curator", false}, {"grid:planner", false},
		{"grid-grid-curator", false}, {"grid-grid-planner", false},
	} {
		t.Run(agent.name, func(t *testing.T) {
			f := setup(t)
			for _, dir := range []string{f.grid, filepath.Join(f.projects, "example")} {
				in := f.input("Write", filepath.Join(dir, "knowledge", "note.md"))
				in.AgentType = agent.name
				got := EvaluatePlan(in, f.root, f.env)
				if (got.Behavior != "deny") != agent.allow {
					t.Fatalf("brain write allowed=%t, want %t", got.Behavior != "deny", agent.allow)
				}
			}
		})
	}
}
