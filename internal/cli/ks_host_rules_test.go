package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// ksHrMatching returns the written Edit rules that match target.
func ksHrMatching(t *testing.T, h *harness, rules []string, target string) []string {
	t.Helper()
	var matching []string
	for _, rule := range rules {
		if strings.HasPrefix(rule, "Edit(") && claudeRuleMatches(t, h.home, rule, target) {
			matching = append(matching, rule)
		}
	}
	return matching
}

// Setup writes no deny rule for the gate files of a task-like folder below the Grid
// home, the settings, lock and workflows files or the host folder: agents write them,
// and a refusal there stalled tasks on edge cases.
func TestKsHostRulesGateFilesBelowHome(t *testing.T) {
	for _, location := range []string{"default", "outside home"} {
		t.Run(location, func(t *testing.T) {
			h := claudeHarness(t)
			projects := filepath.Join(h.gridHome, "projects")
			if location == "outside home" {
				projects = filepath.Join(h.root, "work")
			}
			t.Setenv("GRID_PROJECTS", projects)
			if code := h.run(h.home, "setup", "--yes"); code != 0 {
				t.Fatalf("code=%d: %s", code, h.errb)
			}
			rules := claudeDenyRules(t)

			task := filepath.Join(h.gridHome, "notes", "tasks", "T-01-x")
			parent := filepath.Dir(projects)
			for label, target := range map[string]string{
				"approval.json below home":   filepath.Join(task, "approval.json"),
				"question below home":        filepath.Join(task, "questions", "q1.md"),
				"settings.json in home":      filepath.Join(h.gridHome, "settings.json"),
				"workflows.md below home":    filepath.Join(h.gridHome, "x", "workflows.md"),
				".host in home":              filepath.Join(h.gridHome, ".host", "hold.json"),
				"gate file below the parent": filepath.Join(parent, "notes", "tasks", "T-01-x", "approval.json"),
				"settings.json in parent":    filepath.Join(parent, "settings.json"),
			} {
				if got := ksHrMatching(t, h, rules, target); len(got) != 0 {
					t.Errorf("%s: rules match %s: %v", label, target, got)
				}
			}
		})
	}
}
