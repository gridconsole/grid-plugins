package signal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const turnLinkedWorkflows = "# Workflows\n\n## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" +
	"| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n" +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

// signal act turn makes its task through the same taskFront as `grid task new`
// and marks the note done.
func TestTurnMakesATask(t *testing.T) {
	testgit.Isolate(t)
	s := fixture(t)
	root := gridhome.Real(t.TempDir())
	s.Resolver = where.Resolver{GridHome: s.GridHome, Home: root, Ceiling: root}
	for name, text := range map[string]string{"workflows.md": turnLinkedWorkflows, filepath.Join("projects", "example", "workflows.md"): "# Workflows\n"} {
		if err := os.WriteFile(filepath.Join(s.GridHome, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	added, err := s.Add(Input{Source: "mail", SourceID: "message-1", Title: "Turn me", Body: "body"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Apply(added.ID, ActOptions{Act: "turn", Cwd: root, Who: "alex", Role: "owner"})
	tasks, _ := filepath.Glob(filepath.Join(s.GridHome, "projects", s.Project, "tasks", "*"))
	if err != nil || got.Status != "done" || len(tasks) != 1 {
		t.Fatalf("turn = %+v, %v, tasks %v", got, err, tasks)
	}
}
