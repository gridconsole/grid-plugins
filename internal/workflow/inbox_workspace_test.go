package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInboxWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		bad        bool
	}{
		{"missing", "", false},
		{"legacy", "# Workflows\n", false},
		{"unset", "---\nother: true\n---\n", false},
		{"empty-front", "---\n---\n", true},
		{"personal", "---\ninbox_workspace: personal\n---\n", false},
		{"crlf", "---\r\ninbox_workspace: personal\r\n---\r\n", false},
		{"quoted", "---\ninbox_workspace: 'personal'\n---\n", false},
		{"shared", "---\ninbox_workspace: shared\n---\n", true},
		{"case", "---\ninbox_workspace: Personal\n---\n", true},
		{"whitespace", "---\ninbox_workspace: ' personal '\n---\n", true},
		{"empty", "---\ninbox_workspace: ''\n---\n", true},
		{"null", "---\ninbox_workspace: null\n---\n", true},
		{"boolean", "---\ninbox_workspace: false\n---\n", true},
		{"number", "---\ninbox_workspace: 42\n---\n", true},
		{"list", "---\ninbox_workspace: [personal]\n---\n", true},
		{"map", "---\ninbox_workspace: {name: personal}\n---\n", true},
		{"duplicate", "---\ninbox_workspace: personal\ninbox_workspace: shared\n---\n", true},
		{"malformed", "---\ninbox_workspace: [\n---\n", true},
		{"unclosed", "---\ninbox_workspace: personal\n", true},
		{"unrelated-malformed", "---\nother: [\n---\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.text != "" {
				if err := os.WriteFile(filepath.Join(home, "workflows.md"), []byte(tc.text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := InboxWorkspace(home)
			if tc.bad {
				if got != "" || !errors.Is(err, ErrInboxWorkspace) || err.Error() != ErrInboxWorkspace.Error() {
					t.Fatalf("policy %q, error %v", got, err)
				}
			} else if got != "personal" || err != nil {
				t.Fatalf("policy %q, error %v", got, err)
			}
		})
	}
}

func TestInboxWorkspaceFileBounds(t *testing.T) {
	for _, kind := range []string{"oversize", "invalid-utf8", "symlink", "dangling-symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "workflows.md")
			var err error
			switch kind {
			case "oversize":
				err = os.WriteFile(path, []byte("---\ninbox_workspace: personal\n---\n"+strings.Repeat("x", 32769)), 0600)
			case "invalid-utf8":
				err = os.WriteFile(path, []byte{255}, 0600)
			case "symlink", "dangling-symlink":
				target := filepath.Join(t.TempDir(), "policy.md")
				if kind == "symlink" {
					if err := os.WriteFile(target, []byte("---\ninbox_workspace: personal\n---\n"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				err = os.Symlink(target, path)
			case "directory":
				err = os.Mkdir(path, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got, err := InboxWorkspace(home); got != "" || !errors.Is(err, ErrInboxWorkspace) {
				t.Fatalf("policy %q, error %v", got, err)
			}
		})
	}
}
