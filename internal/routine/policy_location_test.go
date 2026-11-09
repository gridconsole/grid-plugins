package routine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func TestRoutineWorkflowsPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, workspace, project, entry string
		want                            Limits
	}{
		{"defaults", "", "", "", Limits{"10m", 10, 1}},
		{"workspace", "routine_timeout: 20m\nroutine_max_turns: 20\nroutine_budget: 2", "", "", Limits{"20m", 20, 2}},
		{"project", "routine_timeout: 20m\nroutine_max_turns: 20\nroutine_budget: 2", "routine_timeout: 30m\nroutine_max_turns: 30\nroutine_budget: 3", "", Limits{"30m", 30, 3}},
		{"entry", "routine_timeout: 20m\nroutine_max_turns: 20\nroutine_budget: 2", "routine_timeout: 30m\nroutine_max_turns: 30\nroutine_budget: 3", "all", Limits{"40m", 40, 4}},
		{"per-key", "routine_timeout: 20m\nroutine_max_turns: 20\nroutine_budget: 2", "routine_max_turns: 30", "budget", Limits{"20m", 30, 4}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			s := Store{GridHome: home, Project: "example"}
			writePluralPolicy(t, home, tc.workspace)
			writePluralPolicy(t, filepath.Join(home, "projects", s.Project), tc.project)
			var p LimitPolicy
			if tc.entry != "" {
				p = policyForTest(t, map[string]string{"routine_budget": "4"})
				if tc.entry == "all" {
					p = policyForTest(t, map[string]string{"routine_timeout": "40m", "routine_max_turns": "40", "routine_budget": "4"})
				}
			}
			e, err := s.Add("daily", "1h", "signals", p)
			if err != nil || e.Limits != tc.want {
				t.Fatalf("limits = %+v, %v; want %+v", e.Limits, err, tc.want)
			}
			got, err := s.Get("daily")
			if err != nil || got.Limits != tc.want {
				t.Fatalf("read limits = %+v, %v; want %+v", got.Limits, err, tc.want)
			}
		})
	}
}

func writePluralPolicy(t *testing.T, dir, front string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workflows.md"), []byte("---\n"+front+"\n---\n# Workflows\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRoutinePolicyWithoutFrontMatter(t *testing.T) {
	s := Store{GridHome: gridhome.Real(t.TempDir())}
	if err := os.WriteFile(filepath.Join(s.GridHome, "workflows.md"), []byte("# Workflows\n"), 0600); err != nil {
		t.Fatal(err)
	}
	e, err := s.Add("daily", "1h", "signals")
	if err != nil || e.Limits != (Limits{"10m", 10, 1}) {
		t.Fatalf("prose policy: %+v %v", e.Limits, err)
	}
}
