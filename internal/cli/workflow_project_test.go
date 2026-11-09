package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

func TestWorkflowProjectSelection(t *testing.T) {
	for _, tc := range []struct {
		name, alphaID, betaID, selector, want string
	}{
		{"name before id", "beta", "id-beta", "beta", "beta"},
		{"name before later id", "id-alpha", "alpha", "alpha", "alpha"},
		{"name before duplicate ids", "beta", "beta", "beta", "beta"},
		{"unique id", "id-alpha", "id-beta", "id-beta", "beta"},
		{"duplicate id", "shared-id", "shared-id", "shared-id", ""},
		{"missing", "id-alpha", "id-beta", "unknown", ""},
	} {
		for _, action := range []string{"show", "write"} {
			t.Run(tc.name+"/"+action, func(t *testing.T) {
				h := newHarness(t)
				h.env.Getenv = func(string) string { return "" }
				for name, id := range map[string]string{"alpha": tc.alphaID, "beta": tc.betaID} {
					h.write(filepath.Join(h.gridHome, "projects", name, "project.md"), "---\nid: "+id+"\n---\n")
					if action == "show" {
						h.write(filepath.Join(h.gridHome, "projects", name, "workflows.md"), "# "+name+" workflows\n")
					}
				}
				args := []string{"workflow", "show", "--json"}
				text := "# Workflows\n"
				if action == "write" {
					args = workflowWriteArgs("none")
					h.env.Stdin = strings.NewReader(text)
				}
				code := h.run(h.root, append(args, "--project", tc.selector)...)
				if tc.want == "" {
					prefix := "grid: "
					if action == "write" {
						prefix = "grid workflow write: "
					}
					wantErr := fmt.Sprintf("%sproject %q is missing or ambiguous\n", prefix, tc.selector)
					if code != 1 || h.out.Len() != 0 || h.errb.String() != wantErr {
						t.Errorf("exit %d: out=%q err=%q, want exit 1 and %q", code, h.out, h.errb, wantErr)
					}
				} else {
					if code != 0 || h.errb.Len() != 0 {
						t.Fatalf("--project %s: exit %d: %s", tc.selector, code, h.errb)
					}
					if action == "show" {
						var got workflow.Answer
						if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
							t.Fatal(err)
						}
						if len(got.Files) != 2 || got.Files[0].Scope != "workspace" || got.Files[1].Project == nil || *got.Files[1].Project != tc.want || got.Files[1].Text != "# "+tc.want+" workflows\n" || len(got.Errors) != 0 {
							t.Fatalf("wrong project: %s", h.out)
						}
					} else {
						var got workflow.WriteResult
						if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
							t.Fatal(err)
						}
						if got.File != tc.want+"/workflows.md" || len(got.Rev) != 12 {
							t.Fatalf("wrong project: %s", h.out)
						}
					}
				}
				if action == "write" {
					for _, name := range []string{"alpha", "beta"} {
						data, err := os.ReadFile(filepath.Join(h.gridHome, "projects", name, "workflows.md"))
						if name == tc.want {
							if err != nil || string(data) != text {
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
