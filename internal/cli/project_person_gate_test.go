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
		{name: "implicit_note", workspace: "[signal act note]", act: "", want: 3},
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
			args := []string{"signal", "act", actID, "--json"}
			if tc.act != "" {
				args = append(args, "--act", tc.act)
			}
			if code := h.run(cwd, args...); code != tc.want {
				t.Fatalf("exit %d; want %d: %s %s", code, tc.want, h.out, h.errb)
			}
			if tc.person && tc.want == 3 && !strings.Contains(h.out.String(), "agent_env") {
				t.Fatal(h.out)
			}
			if tc.name == "invalid_warns" && (!strings.Contains(h.errb.String(), invalidPersonWarning) || strings.Contains(h.errb.String(), h.root)) {
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
	h.write(filepath.Join(h.gridHome, "workflows.md"), "# Workflows\n")
	h.env.Cwd = fixture.Path
	if code := gateCommand([]string{"signal", "act", actID, "--act", "turn"}, &h.env); code != 3 {
		t.Fatalf("project default bypassed from task: %d", code)
	}
	h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: [signal act]\n---\n")
	if code := gateCommand([]string{"signal", "act", actID, "--act", "note"}, &h.env); code != 3 {
		t.Fatalf("task policy ignored: %d", code)
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

func TestProjectPersonSignalFlagSemantics(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
		want  int
	}{
		{"single_dash", []string{"-project", "example", "-act", "turn"}, 3},
		{"last_act_gated", []string{"--act", "note", "--act=turn", "--project", "example"}, 3},
		{"last_act_open", []string{"--act=turn", "--act", "note", "--project", "example"}, 0},
		{"last_project", []string{"--project", "absent", "--project=example", "--act", "turn"}, 3},
		{"option_value", []string{"--until", "--act=note", "--act", "turn", "--project", "example"}, 3},
		{"to_value", []string{"--project", "example", "--act", "turn", "--to", "--act=note"}, 3},
		{"fence", []string{"--project", "example", "--", "--act=turn"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			h.env.Getenv = func(string) string { return "" }
			h.env.Procs = testProcs("codex")
			h.env.Cwd = h.root
			args := append([]string{"signal", "act", actID}, tc.flags...)
			if code := gateCommand(args, &h.env); code != tc.want {
				t.Fatalf("gate exit %d; want %d", code, tc.want)
			}
		})
	}
}

func TestProjectPersonUnreadableKeepsFloor(t *testing.T) {
	h, _, _ := actFixture(t, "new")
	h.env.Getenv = func(string) string { return "" }
	h.env.Procs = testProcs("codex")
	h.mkdir(filepath.Join(h.gridHome, "projects", "example", "workflows.md"))
	before := fileBytes(t, h.gridHome)
	if code := h.run(h.root, "approve", "--project", "example", "--json"); code != 3 || !strings.Contains(h.errb.String(), invalidPersonWarning) || strings.Contains(h.errb.String(), h.root) {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
		t.Fatal("refusal changed files")
	}
}

func TestProjectPersonSignalTaskUnion(t *testing.T) {
	for _, tc := range []struct {
		name, workspace, project, task, act string
		explicit                            bool
	}{
		{name: "task_cwd_without_policy", act: "turn"},
		{name: "task_cwd_project", project: "[signal act note]", task: "[]", act: "note"},
		{name: "task_cwd_workspace", workspace: "[signal act note]", task: "[task show]", act: "note"},
		{name: "add_task_selector", project: "[signal act add]", task: "[task show]", act: "add", explicit: true},
		{name: "task_act_specific", project: "[]", task: "[signal act note]", act: "note"},
		{name: "task_dismiss_alias", project: "[]", task: "[signal act dismiss]", act: "dismiss"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			fixture := taskFixtureProject(t, h, "example")
			h.env.Getenv = func(string) string { return "" }
			h.env.Procs = testProcs("codex")
			for path, policy := range map[string]string{
				filepath.Join(h.gridHome, "workflows.md"):                        tc.workspace,
				filepath.Join(h.gridHome, "projects", "example", "workflows.md"): tc.project,
				filepath.Join(fixture.Path, "workflow.md"):                       tc.task,
			} {
				if policy != "" {
					h.write(path, "---\nperson_verbs: "+policy+"\n---\n")
				}
			}
			cwd := fixture.Path
			args := []string{"signal", "act", actID, "--act", tc.act, "--json"}
			if tc.act == "dismiss" {
				args = []string{"signal", "dismiss", actID, "--json"}
			}
			if tc.explicit {
				cwd = h.root
				args = append(args, "--project", "example", "--task", fixture.ID)
			}
			before := fileBytes(t, h.gridHome)
			if code := h.run(cwd, args...); code != 3 {
				t.Fatalf("exit %d; want person refusal", code)
			}
			if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
				t.Fatal("refusal changed files")
			}
		})
	}
}

func TestProjectPersonBadTaskWarning(t *testing.T) {
	for _, body := range []string{"---\nperson_verbs: false\n---\n", "---\nperson_verbs: [task show\n---\n"} {
		h := newHarness(t)
		fixture := taskFixture(t, h)
		h.write(filepath.Join(fixture.Path, "workflow.md"), body)
		h.env.Getenv = func(string) string { return "" }
		h.env.Procs = testProcs("codex")
		if code := h.run(fixture.Path, "task", "show", fixture.ID); code != 0 || h.errb.String() != invalidPersonWarning {
			t.Errorf("exit %d; want successful read with exact invalid policy warning", code)
		}
	}
}
