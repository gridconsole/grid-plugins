package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestPersonalGuardRefusesHostCallers(t *testing.T) {
	for _, role := range []string{"guest", "member", "lead", "external", " guest \n"} {
		for _, route := range []string{"personal", "PERSONAL", "grid-home", "task-folder", "task-id", "task-path"} {
			t.Run(role+"/"+route, func(t *testing.T) {
				h := newHarness(t)
				h.env.Getenv = func(key string) string {
					if key == "GRID_ROLE" {
						return role
					}
					if key == "GRID_MEMBER" {
						return "sam"
					}
					return ""
				}
				task := filepath.Join(h.gridHome, "personal", "tasks", "T-1-private")
				h.mkdir(filepath.Join(task, "artifacts"))
				cwd := h.home
				args := []string{"where", "--json"}
				switch route {
				case "personal", "PERSONAL":
					args = append(args, "--project", route)
				case "grid-home":
					cwd = filepath.Join(h.gridHome, "personal")
				case "task-folder":
					cwd = filepath.Join(task, "artifacts")
				case "task-id":
					cwd = task
					args = append(args, "--task", "T-1")
				case "task-path":
					args = append(args, "--task", task)
				}
				if code := h.run(cwd, args...); code != 1 {
					t.Errorf("exit = %d, want 1", code)
				}
				if got := h.out.String(); got != "" {
					t.Errorf("stdout = %q, want no output or paths", got)
				}
				want := "grid: this caller cannot use the Personal project\n"
				switch route {
				case "personal", "PERSONAL":
					want = fmt.Sprintf("grid: project %q is not registered\n", route)
				case "task-path":
					want = "grid: task not found\n"
				}
				if got := h.errb.String(); got != want {
					t.Errorf("stderr = %q, want path-free Personal refusal", got)
				}
			})
		}
	}
}

func TestPersonalGuardAllowsLocalAndOwner(t *testing.T) {
	for _, role := range []string{"", "owner", " owner \n", " \t"} {
		t.Run(role, func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("GRID_ROLE", role)
			if code := h.run(h.home, "where"); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, "where-personal-plain", h.norm(h.out.String()))
			for _, name := range []string{"personal", "PERSONAL"} {
				if code := h.run(h.home, "where", "--project", name, "--json"); code != 0 {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				var got where.Result
				if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Project != where.Personal || got.Rule != where.RuleExplicit {
					t.Errorf("unexpected Personal result: %+v", got)
				}
			}
		})
	}
}

func TestPersonalGuardPreservesExplicitProjectMatching(t *testing.T) {
	for _, role := range []string{"guest", "member", "owner", "unknown"} {
		t.Run(role, func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("GRID_ROLE", role)
			h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
			for _, name := range []string{"alpha", "01ALPHA"} {
				if code := h.run(h.home, "where", "--project", name, "--json"); code != 0 {
					t.Fatalf("project %s: exit %d: %s", name, code, h.errb)
				}
				var got where.Result
				if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				if got.Project != "alpha" || got.ID != "01ALPHA" || !got.Record {
					t.Errorf("unexpected project result: %+v", got)
				}
			}
			for _, name := range []string{"ALPHA", "01alpha", "unregistered"} {
				if code := h.run(h.home, "where", "--project", name, "--json"); code != 1 || h.out.Len() != 0 {
					t.Errorf("project %s: exit %d, stdout %q; want failure without result", name, code, h.out)
				}
			}
		})
	}
}
