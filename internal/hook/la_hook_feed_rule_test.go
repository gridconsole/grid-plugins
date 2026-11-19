package hook

import (
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// laHookDenyPatterns lists the pattern of every permissions deny Edit(...) rule.
func laHookDenyPatterns(rules []settings.ClaudeRule) []string {
	var patterns []string
	for _, r := range rules {
		if r.Section != "permissions" || r.Key != "deny" {
			continue
		}
		if p, ok := strings.CutPrefix(r.Value, "Edit("); ok {
			patterns = append(patterns, strings.TrimSuffix(p, ")"))
		}
	}
	return patterns
}

// laHookTaskFolder is the path up to and including the folder that follows a
// "tasks" folder, in the ~/ form the rules use; a path with no such folder is
// returned whole.
func laHookTaskFolder(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		if strings.EqualFold(part, "tasks") && i+1 < len(parts) {
			return strings.Join(parts[:i+2], "/")
		}
	}
	return p
}

// laHookMatching returns the deny patterns that carry p: the pattern, with its
// trailing /** cut, matches the task folder of p.
func laHookMatching(t *testing.T, patterns []string, p string) []string {
	t.Helper()
	folder := laHookTaskFolder(p)
	var matched []string
	for _, pattern := range patterns {
		ok, err := path.Match(strings.TrimSuffix(pattern, "/**"), folder)
		if err != nil {
			t.Fatalf("pattern %q: %v", pattern, err)
		}
		if ok {
			matched = append(matched, pattern)
		}
	}
	return matched
}

// A task Feed holds the answer and approval records, and agents write their own task
// state: no Grid hook guard refuses a write to it, and no deny rule `grid setup`
// writes into Claude Code's permissions carries it, under the projects folder, under
// Personal or under any other folder one level below the Grid home.
func TestLaHookFeedIsNotDeniedBySetup(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	grid := filepath.Join(home, ".grid")
	rules, err := settings.ClaudeBlock(home, grid)
	if err != nil {
		t.Fatal(err)
	}
	patterns := laHookDenyPatterns(rules)
	const base = "~/.grid/"
	feeds := []struct{ name, path string }{
		{"project task", base + "projects/example/tasks/T-01/feed/record.jsonl"},
		{"personal task", base + "personal/tasks/T-01/feed/record.jsonl"},
		{"folder one level below the home", base + "other/tasks/T-1/feed/record.jsonl"},
	}
	for _, tt := range feeds {
		t.Run(tt.name, func(t *testing.T) {
			if got := laHookMatching(t, patterns, tt.path); len(got) != 0 {
				t.Fatalf("deny Edit rules carry %s: %q", tt.path, got)
			}
		})
	}
	// Not vacuous: the matcher does carry each Feed path under the task folder rules
	// earlier setups wrote.
	t.Run("the old task folder rules carried them", func(t *testing.T) {
		old := []string{base + "projects/*/tasks/[Tt]-[0-9]*/**", base + "personal/tasks/[Tt]-[0-9]*/**", base + "*/tasks/[Tt]-[0-9]*/**"}
		for _, tt := range feeds {
			if got := laHookMatching(t, old, tt.path); len(got) == 0 {
				t.Fatalf("the matcher carries %s by none of %q", tt.path, old)
			}
		}
	})
}
