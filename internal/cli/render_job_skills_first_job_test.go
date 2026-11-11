package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The first Job grid task new posts carries skills: ["plan-jobs"]. The brief the
// worker renders for it must name that skill as grid:plan-jobs, because with the
// plugin loaded a skill only runs when the agent is told its exact name.
func TestTaskNewFirstJobBriefNamesItsSkill(t *testing.T) {
	h := newTaskHarness(t)
	path := newTask(t, h)
	if code := h.run(path, "job", "render", path, "J-1", "--json"); code != 0 {
		t.Fatalf("job render exit %d: %s %s", code, h.out, h.errb)
	}
	saved, err := os.ReadFile(filepath.Join(path, "jobs", "J-1", "brief.md"))
	if err != nil {
		t.Fatalf("job render saved no brief: %v (render printed %s)", err, h.out)
	}
	brief := string(saved)
	if n := strings.Count(brief, "### Skills\n"); n != 1 {
		t.Fatalf("%d Skills sections in the first Job's brief:\n%s", n, brief)
	}
	if !strings.Contains(brief, "- grid:plan-jobs: load it with the Skill tool before you start") {
		t.Fatalf("the first Job's brief does not name grid:plan-jobs:\n%s", brief)
	}
	if strings.Contains(brief, "not in this plugin") {
		t.Fatalf("plan-jobs is marked as missing from the plugin:\n%s", brief)
	}
}
