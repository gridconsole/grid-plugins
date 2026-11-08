package where

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestProjectRoleScope(t *testing.T) {
	for _, tc := range []struct {
		role       string
		restricted bool
	}{
		{"", false}, {"owner", false}, {"member", false}, {"lead", false},
		{"cli", false}, {"host", false}, {"worker", false}, {"agent", false},
		{"builder", false}, {"checker", false}, {"curator", false}, {"designer", false},
		{"library", false}, {"operator", false}, {"planner", false}, {"researcher", false},
		{"reviewer", false}, {"runner", false}, {"scout", false},
		{"guest", true}, {"external", true}, {"unknown", true}, {"grid-", true},
		{"grid-grid-owner", true}, {"grid-grid-member", true}, {"grid-grid-worker", true},
	} {
		spellings := []string{tc.role, " " + strings.ToUpper(tc.role) + " "}
		if tc.role != "" {
			spellings = append(spellings, "grid-"+tc.role, " GRID-"+strings.ToUpper(tc.role)+" ")
		}
		for _, role := range spellings {
			t.Run(role, func(t *testing.T) {
				f := newFixture(t)
				f.record(project.Record{Name: "alpha", ID: "id-alpha"})
				f.record(project.Record{Name: "beta", ID: "id-beta"})
				if err := os.WriteFile(filepath.Join(f.gridHome, "projects", "alpha", "project-roles.json"), []byte(`{"people":[{"id":"gus"}]}`), 0600); err != nil {
					t.Fatal(err)
				}
				f.r.Caller = CallerFrom(func(key string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "gus"}[key] })
				if _, err := f.r.Resolve(Query{Cwd: f.home, Project: "alpha"}); err != nil {
					t.Fatalf("invited project: %v", err)
				}
				for _, selector := range []string{"beta", "id-beta"} {
					_, err := f.r.Resolve(Query{Cwd: f.home, Project: selector})
					if (err != nil) != tc.restricted {
						t.Errorf("hidden selector %q: err=%v, restricted=%t", selector, err, tc.restricted)
					}
				}
			})
		}
	}
}
