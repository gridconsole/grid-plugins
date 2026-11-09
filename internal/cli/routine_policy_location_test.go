package cli

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/routine"
)

func TestRoutineWorkflowBadLimitNamesLevel(t *testing.T) {
	for _, level := range []string{"workspace", "project"} {
		for _, key := range []string{"routine_timeout", "routine_max_turns", "routine_budget"} {
			t.Run(level+"/"+key, func(t *testing.T) {
				h := newHarness(t)
				folder := filepath.Join(h.root, "work", "example")
				h.mkdir(folder)
				if code := h.run(folder, "project", "add"); code != 0 {
					t.Fatal(h.errb.String())
				}
				s := routine.Store{GridHome: h.gridHome, Project: "example"}
				// Explicit entry overrides must not hide invalid inherited policy.
				var p routine.LimitPolicy
				for k, v := range map[string]string{"routine_timeout": "1m", "routine_max_turns": "1", "routine_budget": "1"} {
					if err := p.SetText(k, v); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := s.Add("daily", "1h", "grid version", p); err != nil {
					t.Fatal(err)
				}
				config := brain.Store{GridHome: h.gridHome, Project: s.Project}
				before, err := config.ReadConfig("project", "routines", "daily")
				if err != nil {
					t.Fatal(err)
				}
				dir := h.gridHome
				if level == "project" {
					dir = filepath.Join(dir, "projects", s.Project)
				}
				h.write(filepath.Join(dir, "workflows.md"), "---\n"+key+": "+strconv.Quote(filepath.Join(h.root, "private-value"))+"\n---\n# Workflows\n")
				h.env.Executable = func() (string, error) { return filepath.Join(h.root, "must-not-run"), nil }
				for _, args := range [][]string{{"routine", "run", "daily"}, {"routine", "run", "daily", "--due"}, {"routine", "list"}, {"routine", "list", "--json"}} {
					code := h.run(folder, append(args, "--project", s.Project)...)
					want := "grid: " + level + " workflows.md: invalid " + key
					if code != 2 || strings.TrimSpace(h.errb.String()) != want {
						t.Fatalf("exit %d, stderr %q; want exit 2 and %q", code, h.errb.String(), want)
					}
					after, err := config.ReadConfig("project", "routines", "daily")
					if err != nil || !bytes.Equal(before, after) {
						t.Fatalf("refusal changed routine: %v", err)
					}
				}
			})
		}
	}
}

func TestRoutineOldWorkflowIgnoredAndWarned(t *testing.T) {
	for _, project := range []string{"", "example"} {
		t.Run("scope_"+project, func(t *testing.T) {
			h := newHarness(t)
			folder := filepath.Join(h.root, "work", "example")
			h.mkdir(folder)
			if code := h.run(folder, "project", "add"); code != 0 {
				t.Fatal(h.errb.String())
			}
			s := routine.Store{GridHome: h.gridHome, Project: project}
			if _, err := s.Add("daily", "1h", "signals"); err != nil {
				t.Fatal(err)
			}
			oldFiles := []string{"workflow.md"}
			if project != "" {
				oldFiles = append(oldFiles, "projects/example/workflow.md")
			}
			for _, file := range oldFiles {
				h.write(filepath.Join(h.gridHome, filepath.FromSlash(file)), "---\nroutine_timeout: 1s\nroutine_max_turns: 99\nroutine_budget: 49\n---\n")
			}
			h.write(filepath.Join(h.gridHome, "projects", "example", "tasks", "T-1", "workflow.md"), "---\nroutine_timeout: 1s\n---\n")
			// A due skip exercises the run command without launching an agent.
			config := brain.Store{GridHome: h.gridHome, Project: project}
			level := "home"
			if project != "" {
				level = "project"
			}
			if err := config.EditConfig(level, "routines", "daily", func(old []byte) ([]byte, error) {
				return bytes.Replace(old, []byte("---\n"), []byte("---\nlast_run: 2099-01-01T00:00:00Z\n"), 1), nil
			}); err != nil {
				t.Fatal(err)
			}
			args := []string{"routine", "run", "daily", "--due", "--json"}
			if project != "" {
				args = append(args, "--project", project)
			}
			if code := h.run(folder, args...); code != 0 {
				t.Fatalf("old policy affected run: exit %d: %s", code, h.errb)
			}
			if strings.Count(h.errb.String(), "warning:") != len(oldFiles) {
				t.Fatalf("warnings: %q", h.errb.String())
			}
			for _, file := range oldFiles {
				want := "grid: warning: " + file + " contains ignored routine_* keys; move them to " + strings.TrimSuffix(file, "workflow.md") + "workflows.md"
				if !strings.Contains(h.errb.String(), want+"\n") {
					t.Fatalf("missing warning %q: %s", want, h.errb)
				}
			}
			e, err := s.Get("daily")
			if err != nil || e.Limits != (routine.Limits{Timeout: "10m", MaxTurns: 10, Budget: 1}) {
				t.Fatalf("old policy changed limits: %+v %v", e.Limits, err)
			}
		})
	}
}

func TestRoutineRunDoesNotWarnForTaskPolicyOrProse(t *testing.T) {
	h := newHarness(t)
	s := routine.Store{GridHome: h.gridHome}
	if _, err := s.Add("daily", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(h.gridHome, "workflow.md"), "---\nunrelated: true\n---\nroutine_budget: 50\n")
	h.write(filepath.Join(h.gridHome, "projects", "example", "tasks", "T-1", "workflow.md"), "---\nroutine_budget: 50\n---\n")
	h.env.Executable = func() (string, error) { return filepath.Join(h.root, "missing-grid"), nil }
	if code := h.run(h.root, "routine", "run", "daily"); code != 1 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if strings.Contains(h.errb.String(), "warning:") {
		t.Fatalf("unexpected warning: %s", h.errb)
	}
}
