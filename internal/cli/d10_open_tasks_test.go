package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// d10OtTask writes the task folder of task n (task.md and intent.md, the pair
// task.List needs to read it) with the given workflow lines and state. The id is
// the display id T-NN that task.Fold requires.
func d10OtTask(w nhWfHome, n int, workflowLines, state string) {
	id := fmt.Sprintf("T-%02d", n)
	text := "---\nid: " + id + "\ntitle: Task " + id + "\nproject: example\nsize: normal\n" + workflowLines +
		"state: " + state + "\ncreated: 2026-11-01T09:00:00Z\n---\n"
	dir := filepath.Join(w.project, "tasks", id+"-work")
	w.h.write(filepath.Join(dir, "task.md"), text)
	w.h.write(filepath.Join(dir, "intent.md"), "Do "+id+".\n")
}

// d10OtShow runs workflow show --json with args and returns the answer as
// template name -> openTasks, after the answer passes cmd-workflow-show.
func d10OtShow(t *testing.T, w nhWfHome, args ...string) (map[string][]string, []byte) {
	t.Helper()
	raw := w.out(t, w.checkout, append([]string{"workflow", "show", "--json"}, args...)...)
	if err := contract.Validate("cmd-workflow-show", raw); err != nil {
		t.Fatalf("answer does not pass cmd-workflow-show: %v\n%s", err, raw)
	}
	var doc struct {
		Templates []struct {
			Name      string   `json:"name"`
			OpenTasks []string `json:"openTasks"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, raw)
	}
	got := map[string][]string{}
	for _, tpl := range doc.Templates {
		if tpl.OpenTasks == nil {
			t.Fatalf("template %s has a null openTasks\n%s", tpl.Name, raw)
		}
		got[tpl.Name] = tpl.OpenTasks
	}
	return got, raw
}

// Row 1: each open task is listed under the template it runs; a closed task is
// listed under none.
func TestD10OtListsOpenTasksUnderTheirTemplate(t *testing.T) {
	w := nhWfSetup(t)
	w.h.write(filepath.Join(w.h.gridHome, "workflows.md"), nhWfFile(nhWfStandard, nhWfReview))
	d10OtTask(w, 1, "workflow: Review\n", "queued")
	d10OtTask(w, 2, "workflow: Standard\n", "running")
	d10OtTask(w, 3, "workflow: Review\n", "closed")

	got, raw := d10OtShow(t, w, "--project", "example")
	want := map[string][]string{"Standard": {"T-02"}, "Review": {"T-01"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openTasks by template = %v, want %v\n%s", got, want, raw)
	}
	if strings.Contains(string(raw), "T-03") {
		t.Fatalf("the closed task T-03 is listed\n%s", raw)
	}

	// The captured answer is the fixture with a non-empty openTasks.
	// UPDATE_D10_OT_FIXTURES=1 rewrites contract/fixtures/cmd-workflow-show/valid/project-open-tasks.json.
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	path := yiWfFixturePath("cmd-workflow-show", "valid", "project-open-tasks")
	if os.Getenv("UPDATE_D10_OT_FIXTURES") == "1" {
		yiWfWriteFixture(t, path, doc)
	}
	fixture, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("cmd-workflow-show", fixture); err != nil {
		t.Fatalf("fixture %s does not pass cmd-workflow-show: %v", path, err)
	}
	var fixtureDoc any
	if err := json.Unmarshal(fixture, &fixtureDoc); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(fixtureDoc, doc) {
		t.Fatalf("fixture %s is not the captured answer; run with UPDATE_D10_OT_FIXTURES=1\n%s", path, raw)
	}
}

// Row 2: a task whose workflow names no template and a task with an inline list
// are listed nowhere, and the read still exits 0.
func TestD10OtUnresolvedTaskIsListedNowhere(t *testing.T) {
	w := nhWfSetup(t)
	w.h.write(filepath.Join(w.h.gridHome, "workflows.md"), nhWfFile(nhWfStandard, nhWfReview))
	d10OtTask(w, 1, "workflow: Nonesuch\n", "queued")
	d10OtTask(w, 2, "workflow:\n  - {step: plan, kind: approve}\n  - {step: build, kind: run}\n", "queued")
	d10OtTask(w, 3, "workflow: Standard\n", "queued")

	got, raw := d10OtShow(t, w, "--project", "example")
	want := map[string][]string{"Standard": {"T-03"}, "Review": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openTasks by template = %v, want %v\n%s", got, want, raw)
	}
	if code := w.h.run(w.checkout, "workflow", "show", "--project", "example", "--json"); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, w.h.errb)
	}
}

// A task that names no workflow runs the workspace default (no template has a
// rule, so TaskWorkflow needs no choice), and is listed under it; a project's
// own file lists its tasks under its own copy of the template, by file.
func TestD10OtUnnamedTaskIsListedUnderTheDefault(t *testing.T) {
	w := nhWfSetup(t)
	w.h.write(filepath.Join(w.h.gridHome, "workflows.md"), nhWfFile(nhWfStandard))
	d10OtTask(w, 1, "", "queued")
	d10OtTask(w, 2, "", "paused")
	got, raw := d10OtShow(t, w, "--project", "example")
	want := map[string][]string{"Standard": {"T-01", "T-02"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openTasks by template = %v, want %v\n%s", got, want, raw)
	}

	w.h.write(filepath.Join(w.project, "workflows.md"), nhWfFile(nhWfProjectCopy))
	_, raw = d10OtShow(t, w, "--project", "example")
	var doc struct {
		Templates []struct {
			File      string   `json:"file"`
			OpenTasks []string `json:"openTasks"`
		} `json:"templates"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	byFile := map[string][]string{}
	for _, tpl := range doc.Templates {
		byFile[tpl.File] = tpl.OpenTasks
	}
	wantFile := map[string][]string{"workflows.md": {}, "example/workflows.md": {"T-01", "T-02"}}
	if !reflect.DeepEqual(byFile, wantFile) {
		t.Fatalf("openTasks by file = %v, want %v\n%s", byFile, wantFile, raw)
	}
}

// Row 3: without --project every openTasks stays [].
func TestD10OtNoProjectLeavesEmpty(t *testing.T) {
	w := nhWfSetup(t)
	w.h.write(filepath.Join(w.h.gridHome, "workflows.md"), nhWfFile(nhWfStandard, nhWfReview))
	d10OtTask(w, 1, "workflow: Review\n", "queued")
	d10OtTask(w, 2, "", "running")

	got, raw := d10OtShow(t, w)
	want := map[string][]string{"Standard": {}, "Review": {}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("openTasks by template = %v, want %v\n%s", got, want, raw)
	}
}
