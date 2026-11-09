package cli

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func guestLeakFixture(t *testing.T, role string) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	cwd := filepath.Join(h.root, "alpha")
	h.mkdir(cwd)
	h.record(project.Record{Name: "alpha", ID: "id-alpha", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	h.record(project.Record{Name: "hidden", ID: "id-hidden"})
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	h.env.Getenv = func(key string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "gus"}[key] }
	h.env.Stdin = strings.NewReader(`{"test":"make test"}`)
	return h, cwd
}

func TestGuestProjectSettingsWrites(t *testing.T) {
	for _, role := range []string{"guest", "external", "unknown", "owner", "member"} {
		for _, tc := range []struct {
			name string
			args []string
		}{
			{"project_remove", []string{"project", "remove", "alpha"}},
			{"hook_level", []string{"hook", "level", "home-allow", "off", "--project", "alpha"}},
			{"agent_set", []string{"agent", "set", "grid-builder", "--model", "opus", "--project", "alpha"}},
			{"skill_disable", []string{"skill", "disable", "research", "--project", "alpha"}},
			{"routine_add", []string{"routine", "add", "review", "--every", "1h", "--run", "grid doctor", "--project", "alpha"}},
			{"setup", []string{"setup", "alpha", "--answers", "-"}},
			{"claude_setup", []string{"setup", "--yes", "--json"}},
			{"signal_add", []string{"signal", "add", "--title", "review", "--project", "alpha"}},
		} {
			t.Run(role+"/"+tc.name, func(t *testing.T) {
				h, cwd := guestLeakFixture(t, role)
				before := bypassTree(t, h.gridHome)
				homeBefore := bypassTree(t, h.home)
				code := h.run(cwd, tc.args...)
				changed := !reflect.DeepEqual(before, bypassTree(t, h.gridHome))
				homeChanged := !reflect.DeepEqual(homeBefore, bypassTree(t, h.home))
				if role == "owner" || role == "member" {
					if code != 0 || !changed {
						t.Errorf("exit=%d, tree changed=%t; want successful write", code, changed)
					}
					if !homeChanged {
						t.Error("HOME tree unchanged after successful write")
					}
					return
				}
				wantCode, wantReason := 1, "guests cannot"
				switch tc.name {
				case "hook_level", "agent_set", "skill_disable", "setup":
					// Floor writes are refused by the origin gate before dispatch.
					wantCode, wantReason = 3, "agent_env"
				}
				if code != wantCode || h.out.Len() != 0 || !strings.Contains(h.errb.String(), wantReason) {
					t.Errorf("exit=%d, refusal=%q; want exit %d, empty stdout and %q", code, h.errb.String(), wantCode, wantReason)
				}
				if changed {
					t.Error("GRID_HOME tree changed after guest write")
				}
				if homeChanged {
					t.Error("HOME tree changed after guest write")
				}
			})
		}
	}
}

func TestGuestClaudeSetupRemove(t *testing.T) {
	for _, role := range []string{"guest", "external", "unknown"} {
		for _, mode := range []string{"plain", "json"} {
			t.Run(role+"/"+mode, func(t *testing.T) {
				h, cwd := guestLeakFixture(t, role)
				before := bypassTree(t, h.gridHome)
				homeBefore := bypassTree(t, h.home)
				args := []string{"setup", "--remove"}
				if mode == "json" {
					args = append(args, "--json")
				}
				if code := h.run(cwd, args...); code != 1 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "guests cannot change project settings") {
					t.Errorf("exit=%d, refusal=%q; want exit 1, empty stdout and guest refusal", code, h.errb.String())
				}
				if !reflect.DeepEqual(before, bypassTree(t, h.gridHome)) || !reflect.DeepEqual(homeBefore, bypassTree(t, h.home)) {
					t.Error("HOME or GRID_HOME tree changed after guest removal")
				}
			})
		}
	}
}

func TestGuestWorkflowVisibility(t *testing.T) {
	for _, mode := range []string{"plain", "json"} {
		t.Run(mode, func(t *testing.T) {
			h, cwd := guestLeakFixture(t, "guest")
			for _, name := range []string{"alpha", "hidden"} {
				h.write(filepath.Join(h.gridHome, "projects", name, "workflows.md"), strings.ReplaceAll(strings.ReplaceAll(cliWorkflow, "Default", name+"-secret"), "nothing else matches", "label example"))
			}
			args := []string{"workflow", "show"}
			if mode == "json" {
				args = append(args, "--json")
			}
			missingCode := h.run(cwd, append(args, "--project", "unknown")...)
			missingOut, missingErr := h.out.String(), h.errb.String()
			for _, selector := range []string{"hidden", "id-hidden"} {
				code := h.run(cwd, append(args, "--project", selector)...)
				if code != missingCode || h.out.String() != missingOut || h.errb.String() != missingErr {
					t.Errorf("%s: hidden response differs from unknown project", selector)
				}
			}
			for _, selector := range []string{"alpha", "id-alpha"} {
				if code := h.run(cwd, append(args, "--project", selector)...); code != 0 || !strings.Contains(h.out.String(), "alpha-secret") {
					t.Errorf("%s: invited workflow missing", selector)
				}
			}
			// A hidden malformed record must not introduce a diagnostic or hide invited files.
			h.write(filepath.Join(h.gridHome, "projects", "hidden", "project.md"), "---\nbroken: [\n---\n")
			if code := h.run(cwd, args...); code != 0 || strings.Contains(h.out.String()+h.errb.String(), "hidden") || !strings.Contains(h.out.String(), "alpha-secret") {
				t.Error("listing exposes hidden project or omits invited workflow")
			}
		})
	}
}

func TestExternalAndUnknownProjectAccess(t *testing.T) {
	for _, role := range []string{"external", "unknown", " GRID-EXTERNAL ", " GRID-UNKNOWN "} {
		t.Run(role, func(t *testing.T) {
			h, cwd := guestLeakFixture(t, role)
			if code := h.run(cwd, "project", "list", "--json"); code != 0 || strings.Contains(h.out.String(), "hidden") || !strings.Contains(h.out.String(), "alpha") {
				t.Error("role can list uninvited projects or cannot list invited project")
			}
			if code := h.run(cwd, "project-md", "get", "hidden", "id"); code == 0 {
				t.Error("role can read uninvited project")
			}
			before := bypassTree(t, h.gridHome)
			if code := h.run(cwd, "project-md", "set", "alpha", "title", "changed"); code != 1 {
				t.Error("role can write invited project settings")
			}
			if !reflect.DeepEqual(before, bypassTree(t, h.gridHome)) {
				t.Error("GRID_HOME tree changed after restricted-role write")
			}
		})
	}
}
