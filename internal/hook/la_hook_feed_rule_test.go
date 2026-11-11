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

// A task Feed holds the answer and approval records. No Grid hook guard refuses
// a write to it any more; the deny rules `grid setup` writes into Claude Code's
// permissions carry it, under the projects folder, under Personal and under any
// other folder one level below the Grid home.
func TestLaHookFeedCarriedBySetupRule(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	grid := filepath.Join(home, ".grid")
	rules, err := settings.ClaudeBlock(home, grid)
	if err != nil {
		t.Fatal(err)
	}
	patterns := laHookDenyPatterns(rules)
	if len(patterns) == 0 {
		t.Fatal("setup wrote no deny Edit rule")
	}
	const base = "~/.grid/"
	feeds := []struct{ name, path string }{
		{"project task", base + "projects/example/tasks/T-01/feed/record.jsonl"},
		{"personal task", base + "personal/tasks/T-01/feed/record.jsonl"},
		{"folder one level below the home", base + "other/tasks/T-1/feed/record.jsonl"},
	}
	for _, tt := range feeds {
		t.Run(tt.name, func(t *testing.T) {
			if got := laHookMatching(t, patterns, tt.path); len(got) == 0 {
				t.Fatalf("no deny Edit rule carries %s; rules: %q", tt.path, patterns)
			}
		})
	}
	// The third path sits under no projects or personal rule: one rule carries it,
	// and it carries nothing outside a task folder.
	t.Run("one level below the home has one carrier", func(t *testing.T) {
		got := laHookMatching(t, patterns, feeds[2].path)
		if len(got) != 1 {
			t.Fatalf("want one rule for %s, got %q", feeds[2].path, got)
		}
		if outside := laHookMatching(t, got, base+"projects/example/notes.md"); len(outside) != 0 {
			t.Fatalf("the rule %q also carries a path outside a task folder", got[0])
		}
	})
	// Not vacuous: a path outside any task folder is carried by no rule.
	t.Run("outside a task folder is carried by none", func(t *testing.T) {
		outside := base + "projects/example/notes.md"
		if got := laHookMatching(t, patterns, outside); len(got) != 0 {
			t.Fatalf("%s is carried by %q", outside, got)
		}
	})
}
