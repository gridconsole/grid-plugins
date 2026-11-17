package braincmd

import (
	"strings"
	"testing"
)

// TestPbRdWriteVerbTargets walks the braincmd rows of the reader table
// (brain-obsidian.md 2.2): the target of write, drop, rename and revert is read
// with brain.ValidID, so a note of the vault whose name has a space or Unicode
// is a target, and one name with a path separator is not. Each case gives the
// verb a second fault that is checked after the target, so the answer shows
// whether the target was accepted.
func TestPbRdWriteVerbTargets(t *testing.T) {
	env, _ := decideFixture(t)
	const target = "expected <topic>/<name>"
	for _, tc := range []struct {
		verb  string
		args  []string // after the target
		later string   // what the verb says once the target is accepted
	}{
		{"write", []string{"--expect", "none"}, "--file is required"},
		{"drop", []string{"--expect", "zz"}, "--expect must be"},
		{"rename", []string{"a b"}, "the new name must be a plain name"},
		{"revert", []string{"--to", "0"}, "--to must be a version number"},
	} {
		for _, good := range []string{"ideas/Café plan", "notes/Release notes 2026", "decisions/api-style"} {
			t.Run(tc.verb+" accepts "+good, func(t *testing.T) {
				_, _, errs := qfRun(env, append([]string{tc.verb, good}, tc.args...)...)
				if strings.Contains(errs, target) || !strings.Contains(errs, tc.later) {
					t.Errorf("%s %q: stderr %q, want the answer %q", tc.verb, good, errs, tc.later)
				}
			})
		}
		for _, bad := range []string{"ideas/a/b", "ideas/..", "ideas", "../x/y"} {
			t.Run(tc.verb+" refuses "+bad, func(t *testing.T) {
				code, _, errs := qfRun(env, append([]string{tc.verb, bad}, tc.args...)...)
				if code != 2 || !strings.Contains(errs, target) {
					t.Errorf("%s %q: exit %d, stderr %q, want a refused target", tc.verb, bad, code, errs)
				}
			})
		}
	}
}
