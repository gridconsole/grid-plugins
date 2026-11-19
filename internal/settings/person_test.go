package settings

import (
	"path/filepath"
	"testing"
)

// The person's approval identity is an agent's to write: setup denies neither tool on
// ~/.grid-person, and the rules earlier setups wrote there are retired.
func TestPersonDirectoryHasNoDenyRules(t *testing.T) {
	home := t.TempDir()
	grid := filepath.Join(home, ".grid")
	rules, err := ClaudeBlock(home, grid)
	if err != nil {
		t.Fatal(err)
	}
	retired, err := claudeRetired(home, grid, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"Edit", "Write"} {
		rule := ClaudeRule{"permissions", "deny", tool + "(~/.grid-person/**)"}
		for _, r := range rules {
			if r == rule {
				t.Errorf("setup still writes %s", r.Value)
			}
		}
		found := false
		for _, r := range retired {
			found = found || r == rule
		}
		if !found {
			t.Errorf("%s is not retired", rule.Value)
		}
	}
}
