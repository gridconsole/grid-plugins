package settings

import (
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// brainRootOf is the brain folder the Grid config names for the data folder
// gridHome of the user whose home is userHome: "" when the key is absent or
// names the data folder itself, so the brain stays where it is today under
// gridHome/knowledge. getenv may be nil.
func brainRootOf(gridHome, userHome string, getenv func(string) string) string {
	if gridHome == "" || userHome == "" {
		return ""
	}
	h := gridhome.Resolve(func(key string) string {
		if key == "GRID_HOME" {
			return gridHome
		}
		if getenv != nil {
			return getenv(key)
		}
		return ""
	}, userHome)
	if h.Brain == "" || filepath.Clean(h.Brain) == filepath.Clean(gridHome) {
		return ""
	}
	return h.Brain
}

// claudeBrainRules lets Claude Code read and edit the brain folder, which is a
// folder of its own outside the Grid home's knowledge/ (the Grid home's own allow
// rule covers a brain folder inside it): the same Read and Edit rules as the Grid
// home's, on <brain folder>/**. With the key absent it adds nothing and the rules
// stay as they were. It adds no deny: the person and the agents both edit the
// notes of the vault.
func claudeBrainRules(home, gridHome string, getenv func(string) string) ([]ClaudeRule, error) {
	root := brainRootOf(gridHome, home, getenv)
	if root == "" {
		return nil, nil
	}
	anchor, err := claudeProjectAnchor(gridhome.Real(home), gridhome.Real(root))
	if err != nil {
		return nil, err
	}
	return []ClaudeRule{
		{"permissions", "allow", "Read(" + anchor + "/**)"},
		{"permissions", "allow", "Edit(" + anchor + "/**)"},
	}, nil
}
