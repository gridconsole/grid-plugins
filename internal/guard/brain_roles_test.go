package guard

import (
	"path/filepath"
	"testing"
)

// Any agent writes the brain: a rule that left it to the curator and the planner
// stalled tasks whose builder had a note to keep, so no role is fenced from it.
func TestBrainWriteIsOpenToEveryAgent(t *testing.T) {
	for _, agent := range []string{
		"", "grid:grid-curator", "grid-curator", "grid:grid-planner", "grid-planner",
		"grid:grid-builder", "grid-researcher", "curator", "planner", "other:curator", "other:grid-planner",
	} {
		t.Run(agent, func(t *testing.T) {
			f := setup(t)
			planBlock(f)
			for _, dir := range []string{f.grid, filepath.Join(f.projects, "example")} {
				in := f.input("Write", filepath.Join(dir, "knowledge", "note.md"))
				in.AgentType = agent
				if got := EvaluatePlan(in, f.root, f.env); got.Behavior != "" || len(got.Warnings) != 0 {
					t.Fatalf("brain write answered %+v", got)
				}
			}
		})
	}
}
