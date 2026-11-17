package job_test

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// TestPbRdReaders walks the job rows of the reader table (brain-obsidian.md 2.2,
// internal/job/source.go:75): the id a Job holds for a note of the brain. The id
// is a path of the level root, so a note of a vault whose name holds a space or
// Unicode is a brain pointer. The old layout's ids (one plain name) stay valid.
// A pointer is one segment: a/b stays refused until the board decision.
func TestPbRdReaders(t *testing.T) {
	for _, c := range []struct {
		id, level, name string
		ok              bool
	}{
		{"brain:project/conventions", "project", "conventions", true},
		{"brain:home/api-style", "home", "api-style", true},
		{"brain:project/Café plan", "project", "Café plan", true},
		{"brain:home/Release notes 2026", "home", "Release notes 2026", true},
		{"brain:project/a/b", "project", "a/b", false},
		{"brain:project/..", "project", "..", false},
		{"brain:project/", "project", "", false},
		{"brain:other/x", "other", "x", false},
		{"brain:project/a\\b", "project", "a\\b", false},
	} {
		level, name, ok := job.ParseBrain(c.id)
		if ok != c.ok || (ok && (level != c.level || name != c.name)) {
			t.Errorf("ParseBrain(%q) = %q, %q, %t; want %q, %q, %t", c.id, level, name, ok, c.level, c.name, c.ok)
		}
	}
}
