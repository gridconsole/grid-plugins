package workflow

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestReadPreservesPersonalPolicy(t *testing.T) {
	for _, role := range []string{"owner", "member"} {
		for _, allowed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%t", role, allowed), func(t *testing.T) {
				home := t.TempDir()
				putWorkflowFixture(t, filepath.Join(home, "projects", "personal", "project.md"), "---\nid: id-personal\n---\n")
				const text = "# Workflows\n"
				putWorkflowFixture(t, filepath.Join(home, "projects", "personal", "workflows.md"), text)
				caller := where.Caller{Role: role, Member: "sam"}
				r := where.Resolver{GridHome: home, Caller: caller, PersonalAllowed: func(got where.Caller) bool {
					if got != caller {
						t.Fatalf("policy caller = %+v, want %+v", got, caller)
					}
					return allowed
				}}
				for _, selector := range []string{"personal", "id-personal"} {
					got, err := Read(r, selector)
					if allowed {
						if err != nil || len(got.Files) != 2 || got.Files[1].Text != text || got.Files[1].Rev == nil || len(got.Errors) != 0 {
							t.Fatalf("%s: allowed workflow = %+v, %v", selector, got, err)
						}
					} else if err == nil || err.Error() != "project is missing or ambiguous" {
						t.Fatalf("%s: refusal = %v", selector, err)
					}
				}
			})
		}
	}
}
