package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectPersonVerbs(t *testing.T) {
	defaults := map[string][]string{"signal act turn": nil, "signal act dismiss": nil, "signal act snooze": nil}
	for _, tc := range []struct {
		name, workspace, project string
		want                     map[string][]string
		bad                      bool
	}{
		{name: "missing", want: defaults},
		{name: "legacy", workspace: "# Workflows\n", project: "# Workflows\n", want: defaults},
		{name: "unset", workspace: "---\nother: true\n---\n", want: defaults},
		{name: "workspace", workspace: "---\nperson_verbs: [signal act note]\n---\n", want: map[string][]string{"signal act note": nil}},
		{name: "empty", workspace: "---\nperson_verbs: [signal act note]\n---\n", project: "---\nperson_verbs: []\n---\n", want: map[string][]string{}},
		{name: "roles", project: "---\nperson_verbs: {signal act turn: [owner, member, cli, lead, guest]}\n---\n", want: map[string][]string{"signal act turn": {"owner", "member", "cli", "lead", "guest"}}},
		{name: "invalid", project: "---\nperson_verbs: false\n---\n", bad: true},
		{name: "invalid_inherited", workspace: "---\nperson_verbs: false\n---\n", bad: true},
		{name: "replaced_invalid", workspace: "---\nperson_verbs: false\n---\n", project: "---\nperson_verbs: []\n---\n", want: map[string][]string{}},
		{name: "agent_role", project: "---\nperson_verbs: {signal act: [agent]}\n---\n", bad: true},
		{name: "malformed", project: "---\nperson_verbs: [\n---\n", bad: true},
		{name: "unclosed", project: "---\nperson_verbs: []\n", bad: true},
		{name: "duplicate", project: "---\nperson_verbs: []\nperson_verbs: []\n---\n", bad: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, "projects", "example")
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			for path, body := range map[string]string{filepath.Join(home, "workflows.md"): tc.workspace, filepath.Join(dir, "workflows.md"): tc.project} {
				if body != "" {
					if err := os.WriteFile(path, []byte(body), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			got, err := ProjectPersonVerbs(home, "example")
			if (err != nil) != tc.bad || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("policy %v error %v", got, err)
			}
			if tc.bad && (!errors.Is(err, ErrPersonVerbs) || strings.Contains(err.Error(), home)) {
				t.Fatalf("unsafe error %v", err)
			}
		})
	}
}

func TestProjectPersonFileBounds(t *testing.T) {
	for _, scope := range []string{"workspace", "project"} {
		for _, kind := range []string{"oversize", "symlink", "directory", "invalid_utf8"} {
			t.Run(scope+"/"+kind, func(t *testing.T) {
				home := t.TempDir()
				dir := home
				if scope == "project" {
					dir = filepath.Join(home, "projects", "example")
				}
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(dir, "workflows.md")
				var err error
				switch kind {
				case "oversize":
					err = os.WriteFile(path, []byte(strings.Repeat("x", 32769)), 0600)
				case "symlink":
					err = os.Symlink("absent.md", path)
				case "directory":
					err = os.Mkdir(path, 0700)
				case "invalid_utf8":
					err = os.WriteFile(path, []byte{255}, 0600)
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, err := ProjectPersonVerbs(home, "example"); !errors.Is(err, ErrPersonVerbs) || strings.Contains(err.Error(), home) {
					t.Fatalf("unsafe file error %v", err)
				}
			})
		}
	}
}
