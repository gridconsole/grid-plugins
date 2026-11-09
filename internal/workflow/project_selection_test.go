package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestWorkflowProjectSelectionGuestScope(t *testing.T) {
	for _, tc := range []struct {
		name, alphaID, betaID, invited, selector, want string
	}{
		{"visible name wins", "beta", "id-beta", "both", "beta", "beta"},
		{"hidden name cannot win", "beta", "id-beta", "alpha", "beta", "alpha"},
		{"hidden id cannot conflict", "shared-id", "shared-id", "beta", "shared-id", "beta"},
		{"hidden name refused", "id-alpha", "id-beta", "alpha", "beta", ""},
		{"hidden id refused", "id-alpha", "id-beta", "alpha", "id-beta", ""},
		{"visible duplicate id refused", "shared-id", "shared-id", "both", "shared-id", ""},
	} {
		for _, action := range []string{"show", "write"} {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				home := t.TempDir()
				for name, id := range map[string]string{"alpha": tc.alphaID, "beta": tc.betaID} {
					putWorkflowFixture(t, filepath.Join(home, "projects", name, "project.md"), "---\nid: "+id+"\n---\n")
					if tc.invited == "both" || tc.invited == name {
						putWorkflowFixture(t, filepath.Join(home, "projects", name, "project-roles.json"), `{"people":[{"id":"guest@example.com"}]}`)
					}
				}
				r := where.Resolver{GridHome: home, Caller: where.Caller{Role: "guest", Member: "guest@example.com"}}
				var selected string
				var err error
				if action == "show" {
					var got Answer
					got, err = Read(r, tc.selector)
					if err == nil {
						if len(got.Files) != 2 || got.Files[1].Project == nil {
							t.Fatalf("unexpected files: %+v", got.Files)
						}
						selected = *got.Files[1].Project
					}
				} else {
					// Exercise library visibility independently of the CLI's guest write denial.
					var got WriteResult
					got, err = Write(r, tc.selector, "none", strings.NewReader("# Workflows\n"))
					selected = strings.TrimSuffix(got.File, "/workflows.md")
				}
				if tc.want == "" {
					if err == nil || err.Error() != "project is missing or ambiguous" {
						t.Fatalf("want guest refusal, got %v", err)
					}
				} else if err != nil || selected != tc.want {
					t.Fatalf("selected %q, err=%v; want %q", selected, err, tc.want)
				}
				if action == "write" {
					for _, name := range []string{"alpha", "beta"} {
						data, err := os.ReadFile(filepath.Join(home, "projects", name, "workflows.md"))
						if name == tc.want {
							if err != nil || string(data) != "# Workflows\n" {
								t.Fatalf("selected file: %q, %v", data, err)
							}
						} else if !os.IsNotExist(err) {
							t.Fatalf("unselected %s workflow was created", name)
						}
					}
				}
			})
		}
	}
}
