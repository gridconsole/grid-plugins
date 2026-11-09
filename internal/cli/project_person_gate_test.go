package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestProjectPersonSignalDefault(t *testing.T) {
	for _, act := range []string{"turn", "dismiss", "snooze", "alias", "equals", "note", "implicit"} {
		for _, person := range []bool{false, true} {
			t.Run(act+"/person="+map[bool]string{false: "false", true: "true"}[person], func(t *testing.T) {
				h, _, _ := actFixture(t, "new")
				h.env.Getenv = func(string) string { return "" }
				if !person {
					h.env.Procs = testProcs("codex")
				}
				args := actArgs(act)
				switch act {
				case "alias":
					args = []string{"signal", "dismiss", actID, "--project", "example", "--json"}
				case "equals":
					args = []string{"signal", "act", actID, "--project=example", "--act=turn", "--json"}
				case "implicit":
					args = []string{"signal", "act", actID, "--project", "example", "--json"}
				}
				before := fileBytes(t, h.gridHome)
				code := h.run(h.root, args...)
				if !person && act != "note" && act != "implicit" {
					if code != 3 {
						t.Fatalf("exit %d; want person refusal", code)
					}
					var got map[string]string
					if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
						t.Fatal(err)
					}
					want := map[string]string{"error": "person_required", "reason": "agent_ancestor", "hint": personHint}
					if !reflect.DeepEqual(got, want) || !strings.HasPrefix(h.errb.String(), "grid signal act: a person must run this") {
						t.Fatalf("refusal: %s %s", h.out, h.errb)
					}
					if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
						t.Fatal("refusal changed files")
					}
				} else if code != 0 {
					t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
				}
			})
		}
	}
}

func TestProjectPersonPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, workspace, project, act, role string
		person                              bool
		want                                int
	}{
		{name: "empty", project: "[]", act: "turn"},
		{name: "workspace_note", workspace: "[signal act note]", act: "note", want: 3},
		{name: "workspace_replaces_default", workspace: "[signal act note]", act: "turn"},
		{name: "project_replaces_workspace", workspace: "[signal act note]", project: "[]", act: "note"},
		{name: "all_acts", project: "[signal act]", act: "note", want: 3},
		{name: "member", project: "{signal act turn: [owner]}", act: "turn", person: true, want: 3},
		{name: "owner", project: "{signal act turn: [owner]}", act: "turn", role: "owner", person: true},
		{name: "invalid_warns", project: "false", act: "turn"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			h.env.Getenv = func(k string) string {
				if k == "GRID_ROLE" {
					return tc.role
				}
				return ""
			}
			if !tc.person {
				h.env.Procs = testProcs("codex")
			}
			if tc.workspace != "" {
				h.write(filepath.Join(h.gridHome, "workflows.md"), "---\nperson_verbs: "+tc.workspace+"\n---\n")
			}
			projectDir := filepath.Join(h.gridHome, "projects", "example")
			if tc.project != "" {
				h.write(filepath.Join(projectDir, "workflows.md"), "---\nperson_verbs: "+tc.project+"\n---\n")
			}
			cwd := filepath.Join(h.root, "work")
			h.mkdir(cwd)
			h.record(project.Record{Name: "example", ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
			// Resolve from cwd as well as the explicit selector tested above.
			args := []string{"signal", "act", actID, "--act", tc.act, "--json"}
			if code := h.run(cwd, args...); code != tc.want {
				t.Fatalf("exit %d; want %d: %s %s", code, tc.want, h.out, h.errb)
			}
			if tc.person && tc.want == 3 && !strings.Contains(h.out.String(), "agent_env") {
				t.Fatal(h.out)
			}
			if tc.name == "invalid_warns" && (!strings.Contains(h.errb.String(), "workflow: invalid person_verbs:") || strings.Contains(h.errb.String(), h.root)) {
				t.Fatal("missing or unsafe warning")
			}
		})
	}
}

func TestProjectPersonTaskIsolationAndFloor(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	h.env.Procs = testProcs("codex")
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "workflows.md"), "---\nperson_verbs: [task show]\n---\n")
	if code := h.run(fixture.Path, "task", "show", "T-01", "--json"); code != 0 {
		t.Fatalf("project policy leaked into task: %d", code)
	}
	for _, policy := range []string{"[]", "false"} {
		h.write(filepath.Join(h.gridHome, "workflows.md"), "---\nperson_verbs: "+policy+"\n---\n")
		if code := h.run(h.home, "approve", "--json"); code != 3 {
			t.Fatalf("project policy removed floor: %d", code)
		}
		h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: "+policy+"\n---\n")
		if code := h.run(fixture.Path, "approve", "T-01", "--json"); code != 3 {
			t.Fatalf("task policy removed floor: %d", code)
		}
	}
}

func TestProjectPersonHookBypass(t *testing.T) {
	h := newHarness(t)
	h.env.Procs = testProcs("codex")
	h.env.Getenv = func(string) string { return "" }
	h.env.Cwd = h.home
	for _, policy := range []string{"[hook]", "false"} {
		h.write(filepath.Join(h.gridHome, "workflows.md"), "---\nperson_verbs: "+policy+"\n---\n")
		if code := gateCommand([]string{"hook", "PreToolUse"}, &h.env); code != 0 || h.errb.Len() != 0 {
			t.Fatalf("hook gated: %d %s", code, h.errb)
		}
	}
}
