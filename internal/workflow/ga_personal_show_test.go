package workflow

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestGaPersonalShow(t *testing.T) {
	const text = "# Workflows\n\n" + stepsBuildDefault
	newHome := func(t *testing.T) string {
		home := t.TempDir()
		putWorkflowFixture(t, filepath.Join(home, "workflows.md"), text)
		return home
	}
	owner := where.Caller{Role: "owner", Member: "sam"}
	allow := func(ok bool) func(where.Caller) bool { return func(where.Caller) bool { return ok } }

	for _, selector := range []string{"personal", "Personal"} {
		t.Run("owner/"+selector, func(t *testing.T) {
			r := where.Resolver{GridHome: newHome(t), Caller: owner, PersonalAllowed: allow(true)}
			got, err := Read(r, selector)
			if err != nil {
				t.Fatalf("Read(%q) error = %v", selector, err)
			}
			if len(got.Files) != 1 || got.Files[0].Scope != "workspace" || got.Files[0].Project != nil ||
				got.Files[0].Text != text || got.Files[0].Rev == nil || len(got.Errors) != 0 {
				t.Fatalf("Read(%q) = %+v", selector, got)
			}
		})
	}

	t.Run("owner without workflows.md", func(t *testing.T) {
		r := where.Resolver{GridHome: t.TempDir(), Caller: owner, PersonalAllowed: allow(true)}
		got, err := Read(r, "personal")
		if err != nil || len(got.Files) != 1 || got.Files[0].Scope != "workspace" || got.Files[0].Rev != nil {
			t.Fatalf("Read = %+v, %v", got, err)
		}
	})

	t.Run("guest", func(t *testing.T) {
		// Even a policy that says yes must not let a guest read Personal.
		r := where.Resolver{GridHome: newHome(t), Caller: where.Caller{Role: "guest", Member: "guest@example.com"}, PersonalAllowed: allow(true)}
		for _, selector := range []string{"personal", "Personal"} {
			if _, err := Read(r, selector); err == nil || err.Error() != "project is missing or ambiguous" {
				t.Fatalf("Read(%q) refusal = %v", selector, err)
			}
		}
	})

	t.Run("owner without Personal access", func(t *testing.T) {
		r := where.Resolver{GridHome: newHome(t), Caller: owner, PersonalAllowed: allow(false)}
		for _, selector := range []string{"personal", "Personal"} {
			if _, err := Read(r, selector); err == nil || err.Error() != "project is missing or ambiguous" {
				t.Fatalf("Read(%q) refusal = %v", selector, err)
			}
		}
	})

	t.Run("unknown name", func(t *testing.T) {
		r := where.Resolver{GridHome: newHome(t), Caller: owner, PersonalAllowed: allow(true)}
		if _, err := Read(r, "nosuch"); err == nil || err.Error() != `project "nosuch" is missing or ambiguous` {
			t.Fatalf("Read(nosuch) = %v", err)
		}
	})
}
