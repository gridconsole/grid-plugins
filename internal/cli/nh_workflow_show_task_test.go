package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// nhWfTable is the steps table head with a sizes column.
const nhWfTable = "| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n"

// nhWfStandard is the workspace default.
const nhWfStandard = "## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + nhWfTable +
	"| 1 | build | run | grid-builder | all | |\n| 2 | review | check | grid-reviewer | all | |\n\n"

// nhWfReview is a named workspace template with a step only large tasks run.
const nhWfReview = "## Review\napplies when: label review\nreview rounds: 3\n\n" + nhWfTable +
	"| 1 | check-it | check | grid-runner | all | |\n| 2 | deep-check | check | grid-reviewer | L | |\n| 3 | sign-off | approve | sam | all | |\n\n"

// nhWfProjectCopy is the project's copy of the workspace default.
const nhWfProjectCopy = "## Standard\napplies when: as in the workspace Standard\nreview rounds: 5\n\n" + nhWfTable +
	"| 1 | ship | run | grid-builder | all | |\n\n"

// nhWfNote is a brain workflow note named name whose one template is a default.
func nhWfNote(name, template, step string) string {
	return "---\ntype: workflow\nname: " + name + "\nstatus: accepted\n---\n# Workflows\n\n## " + template +
		"\napplies when: nothing else matches\nreview rounds: 3\n\n" + nhWfTable +
		"| 1 | " + step + " | run | grid-builder | all | |\n\n"
}

func nhWfFile(sections ...string) string { return "# Workflows\n\n" + strings.Join(sections, "") }

// nhWfTaskText is a normal-size task.md of project example with the given workflow lines.
func nhWfTaskText(workflowLines string) string {
	return "---\nid: T-1\ntitle: Show the workflow\nproject: example\nsize: normal\n" + workflowLines +
		"state: queued\ncreated: 2026-11-01T09:00:00Z\n---\n"
}

// nhWfHome is a harness with the registered project example and its checkout.
type nhWfHome struct {
	h        *harness
	checkout string
	project  string // the project folder under the Grid home
}

func nhWfSetup(t *testing.T) nhWfHome {
	t.Helper()
	h := newHarness(t)
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER GRID_ROLE") {
		t.Setenv(key, "")
	}
	checkout := filepath.Join(h.root, "checkout")
	h.mkdir(checkout)
	testgit.Init(t, checkout, "https://example.com/org/example.git", true)
	if code := h.run(checkout, "project", "add", "--name", "example"); code != 0 {
		t.Fatalf("project add: exit %d: %s", code, h.errb)
	}
	h.env.Machine = gridhome.Machine(h.gridHome)
	return nhWfHome{h: h, checkout: checkout, project: filepath.Join(h.gridHome, "projects", "example")}
}

// out runs grid and returns stdout; a non-zero exit fails the test.
func (w nhWfHome) out(t *testing.T, cwd string, args ...string) []byte {
	t.Helper()
	if code := w.h.run(cwd, args...); code != 0 {
		t.Fatalf("%s: exit %d: %s", strings.Join(args, " "), code, w.h.errb)
	}
	return append([]byte(nil), w.h.out.Bytes()...)
}

// nhWfGotStep and nhWfGotTask decode the task answer as a reader would.
type nhWfGotStep struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Agent     *string  `json:"agent"`
	Approvers []string `json:"approvers"`
}

type nhWfGotTask struct {
	ID       string        `json:"id"`
	Workflow *string       `json:"workflow"`
	Source   string        `json:"source"`
	File     *string       `json:"file"`
	Rounds   *int          `json:"rounds"`
	Steps    []nhWfGotStep `json:"steps"`
}

func nhWfStr(s string) *string { return &s }
func nhWfInt(n int) *int       { return &n }

func nhWfDecode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, raw)
	}
	return doc
}

// Row 1: one task per source answers its workflow, source, file, rounds and the
// steps TaskWorkflow resolves; a home with no workflow answers the built-in flow.
// UPDATE_NH_WF_FIXTURES=1 rewrites contract/fixtures/cmd-workflow-show/valid/task-*.json.
func TestNhWfShowTaskSources(t *testing.T) {
	update := os.Getenv("UPDATE_NH_WF_FIXTURES") == "1"
	run, check := "run", "check"
	cases := []struct {
		name                   string
		workspace, projectFile string // "" leaves the file absent
		projectNote, homeNote  string // "" writes no note
		workflowLines          string
		want                   nhWfGotTask
		text                   string
		builtIn                bool
		agents                 []*string
	}{
		{
			name:          "inline",
			workflowLines: "workflow:\n  - {step: plan, kind: approve}\n  - {step: build, kind: run}\n",
			want: nhWfGotTask{ID: "T-1", Source: "inline", Steps: []nhWfGotStep{
				{Name: "plan", Kind: "approve"}, {Name: "build", Kind: run}}},
			text: "workflow: (none)\nsource: inline\nrounds: (none)\nstep: plan (approve)\nstep: build (run)\n",
		},
		{
			name:          "workspace",
			workspace:     nhWfFile(nhWfStandard, nhWfReview),
			workflowLines: "workflow: Review\n",
			want: nhWfGotTask{ID: "T-1", Workflow: nhWfStr("Review"), Source: "workspace", File: nhWfStr("workflows.md"), Rounds: nhWfInt(3),
				Steps: []nhWfGotStep{
					{Name: "check-it", Kind: check, Agent: nhWfStr("grid-runner")},
					{Name: "sign-off", Kind: "approve", Approvers: []string{"sam"}}}},
			text: "workflow: Review\nsource: workspace\nrounds: 3\nstep: check-it (check, grid-runner)\nstep: sign-off (approve)\n",
		},
		{
			name:        "project",
			workspace:   nhWfFile(nhWfStandard),
			projectFile: nhWfFile(nhWfProjectCopy),
			want: nhWfGotTask{ID: "T-1", Workflow: nhWfStr("Standard"), Source: "project", File: nhWfStr("example/workflows.md"), Rounds: nhWfInt(5),
				Steps: []nhWfGotStep{{Name: "ship", Kind: run, Agent: nhWfStr("grid-builder")}}},
			text: "workflow: Standard\nsource: project\nrounds: 5\nstep: ship (run, grid-builder)\n",
		},
		{
			name:        "brain-project",
			workspace:   nhWfFile(nhWfStandard),
			projectNote: nhWfNote("flow", "Noted", "note-step"),
			homeNote:    nhWfNote("hflow", "Homed", "home-step"),
			want: nhWfGotTask{ID: "T-1", Workflow: nhWfStr("Noted"), Source: "brain:project", File: nhWfStr("brain:project/flow"), Rounds: nhWfInt(3),
				Steps: []nhWfGotStep{{Name: "note-step", Kind: run, Agent: nhWfStr("grid-builder")}}},
			text: "workflow: Noted\nsource: brain:project\nrounds: 3\nstep: note-step (run, grid-builder)\n",
		},
		{
			name:      "brain-home",
			workspace: nhWfFile(nhWfStandard),
			homeNote:  nhWfNote("hflow", "Homed", "home-step"),
			want: nhWfGotTask{ID: "T-1", Workflow: nhWfStr("Homed"), Source: "brain:home", File: nhWfStr("brain:home/hflow"), Rounds: nhWfInt(3),
				Steps: []nhWfGotStep{{Name: "home-step", Kind: run, Agent: nhWfStr("grid-builder")}}},
			text: "workflow: Homed\nsource: brain:home\nrounds: 3\nstep: home-step (run, grid-builder)\n",
		},
		{
			name:    "built-in",
			builtIn: true,
			want: nhWfGotTask{ID: "T-1", Source: "built-in", Rounds: nhWfInt(2),
				Steps: []nhWfGotStep{{Name: "Run", Kind: run}, {Name: "Check", Kind: check, Agent: nhWfStr("grid-reviewer")}}},
			text: "workflow: (none)\nsource: built-in\nrounds: 2\nstep: Run (run)\nstep: Check (check, grid-reviewer)\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := nhWfSetup(t)
			h := w.h
			if c.workspace != "" {
				h.write(filepath.Join(h.gridHome, "workflows.md"), c.workspace)
			}
			if c.projectFile != "" {
				h.write(filepath.Join(w.project, "workflows.md"), c.projectFile)
			}
			if c.projectNote != "" {
				h.write(filepath.Join(w.project, "knowledge", "decisions", "flow.md"), c.projectNote)
			}
			if c.homeNote != "" {
				h.write(filepath.Join(h.gridHome, "knowledge", "decisions", "hflow.md"), c.homeNote)
			}
			taskText := nhWfTaskText(c.workflowLines)
			h.write(filepath.Join(w.project, "tasks", "T-1-show", "task.md"), taskText)

			raw := w.out(t, w.checkout, "workflow", "show", "--task", "T-1", "--json")
			if err := contract.Validate("cmd-workflow-show", raw); err != nil {
				t.Fatalf("answer does not pass cmd-workflow-show: %v\n%s", err, raw)
			}
			doc := nhWfDecode(t, raw)
			taskRaw, ok := doc["task"]
			if !ok {
				t.Fatalf("answer has no task\n%s", raw)
			}
			var got struct {
				Task nhWfGotTask `json:"task"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Task, c.want) {
				t.Fatalf("task = %s\nwant %+v", mustJSON(t, taskRaw), c.want)
			}

			// The steps are the ones the plugin's own resolver gives the task.
			_, steps, err := workflow.TaskWorkflow(resolver(h.env), "example", taskText)
			if c.builtIn {
				if err == nil || err.Error() != "the workspace has no default workflow" {
					t.Fatalf("TaskWorkflow = %v, %v; want the no-default error the built-in flow stands for", steps, err)
				}
			} else {
				if err != nil {
					t.Fatalf("TaskWorkflow: %v", err)
				}
				if len(steps) != len(got.Task.Steps) {
					t.Fatalf("TaskWorkflow steps %v, answer steps %v", steps, got.Task.Steps)
				}
				for i, s := range steps {
					if got.Task.Steps[i].Name != s.Step || got.Task.Steps[i].Kind != s.Kind {
						t.Fatalf("step %d = %+v, TaskWorkflow gives %+v", i, got.Task.Steps[i], s)
					}
				}
			}

			// Beside task, the answer is the project's own read.
			project := nhWfDecode(t, w.out(t, w.checkout, "workflow", "show", "--project", "example", "--json"))
			delete(doc, "task")
			if !reflect.DeepEqual(doc, project) {
				t.Fatalf("envelope beside task differs from workflow show --project example --json\n%s", raw)
			}

			// --project names the same task from outside its checkout.
			again := w.out(t, h.root, "workflow", "show", "--task", "T-1", "--project", "example", "--json")
			if !bytes.Equal(again, raw) {
				t.Fatalf("with --project from another folder:\n%s\nwant\n%s", again, raw)
			}

			if text := string(w.out(t, w.checkout, "workflow", "show", "--task", "T-1")); text != c.text {
				t.Fatalf("text answer:\n%s\nwant\n%s", text, c.text)
			}

			path := yiWfFixturePath("cmd-workflow-show", "valid", "task-"+c.name)
			full := nhWfDecode(t, raw)
			if update {
				yiWfWriteFixture(t, path, full)
			}
			fixture, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(nhWfDecode(t, fixture), full) {
				t.Fatalf("fixture %s is not the captured answer; run with UPDATE_NH_WF_FIXTURES=1\n%s", path, raw)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Row 2: a task naming a workflow no template has is refused with the
// resolver's error and no JSON, as TaskWorkflow refuses it.
func TestNhWfShowTaskUnknownName(t *testing.T) {
	w := nhWfSetup(t)
	h := w.h
	h.write(filepath.Join(h.gridHome, "workflows.md"), nhWfFile(nhWfStandard))
	taskText := nhWfTaskText("workflow: Nonesuch\n")
	h.write(filepath.Join(w.project, "tasks", "T-1-show", "task.md"), taskText)

	_, _, want := workflow.TaskWorkflow(resolver(h.env), "example", taskText)
	if want == nil || !strings.Contains(want.Error(), `no workflow is named "Nonesuch"`) {
		t.Fatalf("TaskWorkflow error = %v, want no workflow is named", want)
	}
	for _, args := range [][]string{
		{"workflow", "show", "--task", "T-1", "--json"},
		{"workflow", "show", "--task", "T-1"},
	} {
		code := h.run(w.checkout, args...)
		if code == 0 || code == 2 {
			t.Fatalf("%s: exit %d, want a failure that is not a usage error; stderr %s", strings.Join(args, " "), code, h.errb)
		}
		if !strings.Contains(h.errb.String(), `no workflow is named "Nonesuch"`) {
			t.Fatalf("%s: stderr %q, want the resolver's error", strings.Join(args, " "), h.errb)
		}
		if h.out.Len() != 0 {
			t.Fatalf("%s: stdout %q, want none", strings.Join(args, " "), h.out)
		}
	}
	// An empty --task is a usage error, as an empty --project is.
	if code := h.run(w.checkout, "workflow", "show", "--task", ""); code != 2 {
		t.Fatalf("empty --task: exit %d, want 2", code)
	}
}

// Row 2: without --task every answer is the one main gives. The yi fixtures
// are main's captured answers; the bytes are the read main's code path writes.
func TestNhWfShowWithoutTaskIsMain(t *testing.T) {
	for name, raw := range yiWfCollect(t).show {
		fixture, err := os.ReadFile(yiWfFixturePath("cmd-workflow-show", "valid", name))
		if err != nil {
			t.Fatal(err)
		}
		doc := nhWfDecode(t, raw)
		if _, has := doc["task"]; has {
			t.Fatalf("%s: an answer without --task carries task\n%s", name, raw)
		}
		if !reflect.DeepEqual(nhWfDecode(t, fixture), doc) {
			t.Fatalf("%s: answer differs from main's fixture\n%s", name, raw)
		}
	}

	w := nhWfSetup(t)
	h := w.h
	h.write(filepath.Join(h.gridHome, "workflows.md"), nhWfFile(nhWfStandard))
	h.write(filepath.Join(w.project, "workflows.md"), nhWfFile(nhWfProjectCopy))
	h.write(filepath.Join(w.project, "tasks", "T-1-show", "task.md"), nhWfTaskText(""))
	for _, policy := range []bool{false, true} {
		args := []string{"workflow", "show", "--project", "example", "--json"}
		read := workflow.Read
		if policy {
			args = append(args, "--policy")
			read = workflow.ReadPolicy
		}
		got := w.out(t, w.checkout, args...)
		answer, err := read(resolver(h.env), "example")
		if err != nil {
			t.Fatal(err)
		}
		var want bytes.Buffer
		if code := writeJSON(&want, answer); code != 0 {
			t.Fatal("writeJSON failed")
		}
		if !bytes.Equal(got, want.Bytes()) {
			t.Fatalf("%s:\n%s\nwant main's bytes\n%s", strings.Join(args, " "), got, want.Bytes())
		}
	}
	text := string(w.out(t, w.checkout, "workflow", "show", "--project", "example"))
	if want := "file: workflows.md\ntemplates: Standard\nerrors: 0\nfile: example/workflows.md\ntemplates: Standard\nerrors: 0\n"; text != want {
		t.Fatalf("text answer:\n%s\nwant\n%s", text, want)
	}
}

// Row 3: the task fixtures pass cmd-workflow-show; an unknown task.source and
// an unknown key in task are refused. UPDATE_NH_WF_FIXTURES=1 writes the
// invalid ones from the brain-project answer.
func TestNhWfTaskFixtures(t *testing.T) {
	if os.Getenv("UPDATE_NH_WF_FIXTURES") == "1" {
		raw, err := os.ReadFile(yiWfFixturePath("cmd-workflow-show", "valid", "task-brain-project"))
		if err != nil {
			t.Fatal(err)
		}
		doc := nhWfDecode(t, raw)
		doc["task"].(map[string]any)["source"] = "elsewhere"
		yiWfWriteFixture(t, yiWfFixturePath("cmd-workflow-show", "invalid", "task-unknown-source"), doc)
		doc = nhWfDecode(t, raw)
		doc["task"].(map[string]any)["approved"] = true
		yiWfWriteFixture(t, yiWfFixturePath("cmd-workflow-show", "invalid", "task-unknown-key"), doc)
	}
	valid, err := filepath.Glob(yiWfFixturePath("cmd-workflow-show", "valid", "task-*"))
	if err != nil || len(valid) != 6 {
		t.Fatalf("valid task fixtures %v, err %v; want one per source", valid, err)
	}
	for _, path := range valid {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate("cmd-workflow-show", raw); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	for name, word := range map[string]string{"task-unknown-source": "elsewhere", "task-unknown-key": "approved"} {
		raw, err := os.ReadFile(yiWfFixturePath("cmd-workflow-show", "invalid", name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), word) {
			t.Fatalf("%s does not hold %q", name, word)
		}
		err = contract.Validate("cmd-workflow-show", raw)
		if err == nil {
			t.Fatalf("%s passes cmd-workflow-show, want it refused", name)
		}
		if !strings.Contains(err.Error(), "task") {
			t.Fatalf("%s refused for %v, want a task problem", name, err)
		}
	}
}
