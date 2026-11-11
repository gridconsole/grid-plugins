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

// Setup writes one deny rule for the gate files of a task-like folder one level
// below the Grid home, which the host guard refuses. It writes none for the forms
// the host guard also refuses but Grid reads nothing at, by decision: they are
// the person's own folders outside the Grid home.
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
			for _, target := range []string{
				filepath.Join(task, "approval.json"),
				filepath.Join(task, "questions", "q1.md"),
			} {
				if len(ksHrMatching(t, h, rules, target)) == 0 {
					t.Errorf("no written rule matches %s", target)
				}
			}
			if location != "outside home" {
				return
			}

			// Not carried, by decision: nothing Grid reads sits there.
			parent := filepath.Dir(projects)
			for label, target := range map[string]string{
				"gate file below the parent": filepath.Join(parent, "notes", "tasks", "T-01-x", "approval.json"),
				"settings.json in parent":    filepath.Join(parent, "settings.json"),
				"workflows.md below parent":  filepath.Join(parent, "x", "workflows.md"),
				".host above the parent":     filepath.Join(filepath.Dir(parent), ".host", "hold.json"),
			} {
				if got := ksHrMatching(t, h, rules, target); len(got) != 0 {
					t.Errorf("%s: rules match %s: %v", label, target, got)
				}
			}
		})
	}
}
