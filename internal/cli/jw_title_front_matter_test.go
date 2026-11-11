package cli

import (
	"strings"
	"testing"
)

// jwTfRefusal is the message C-ij gave for words that hold only front matter.
const jwTfRefusal = "task words hold only front matter and no title"

// jwTfRefused runs `grid task new` on words that must be refused and checks the
// exit code, the message and that no task folder was made.
func jwTfRefused(t *testing.T, words string) {
	t.Helper()
	h := newHarness(t)
	code, _ := ijTitleNew(t, h, words)
	if code == 0 {
		t.Errorf("task new %q: exit 0, want non-zero; stdout %q", words, h.out.String())
	}
	if !strings.Contains(h.errb.String(), jwTfRefusal) {
		t.Errorf("task new %q: stderr = %q, want it to hold %q", words, h.errb.String(), jwTfRefusal)
	}
	if made := ijTitleFolders(t, h.gridHome, "T-01"); len(made) != 0 {
		t.Errorf("task new %q: task folders written: %v", words, made)
	}
}

func TestJwTfEmptyBlockIsRefused(t *testing.T) {
	for _, tc := range []struct{ name, words string }{
		{"row1-empty-block", "---\n---\n"},
		{"row1-empty-block-no-final-newline", "---\n---"},
		{"row1-empty-block-blank-body", "\n  ---\n---\n  \n\t\n"},
	} {
		t.Run(tc.name, func(t *testing.T) { jwTfRefused(t, tc.words) })
	}
}

func TestJwTfEmptyBlockTitlesFromBody(t *testing.T) {
	for _, tc := range []struct{ name, words, title string }{
		{"row2-lf", "---\n---\nFix the login\nmore\n", "Fix the login"},
		{"row2-blank-lines-between", "---\n---\n\n\nFix the login\n", "Fix the login"},
		{"row2-leading-blank-lines", "\n  \n---\n---\nFix the login\n", "Fix the login"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			title, _ := ijTitleMade(t, h, tc.words)
			if title != tc.title {
				t.Errorf("%s: title = %q, want %q", tc.name, title, tc.title)
			}
		})
	}
}
