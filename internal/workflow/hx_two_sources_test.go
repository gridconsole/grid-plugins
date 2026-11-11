package workflow

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// hxTwoFileQuick is the project workflows.md section "Quick": its one step
// differs from the two steps of the brain note's "Quick" (noteText).
const hxTwoFileQuick = "## Quick\napplies when: label quick\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | ship | run | grid-builder | all | |\n\n"

// hxTwoSteps is the steps of a template as "name:kind" pairs joined by commas.
func hxTwoSteps(tpl Template) string {
	var parts []string
	for _, s := range tpl.Steps {
		parts = append(parts, s.Name+":"+s.Kind)
	}
	return strings.Join(parts, ",")
}

// hxTwoHome lays out one home as TestTaskSteps does, with a task folder that
// holds no workflow.md. withFile adds the project workflows.md section "Quick";
// withNote adds the project brain workflow note. The workspace workflows.md
// always holds the default section only.
func hxTwoHome(t *testing.T, withFile, withNote bool) (where.Resolver, string) {
	t.Helper()
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), stepsWorkspace(stepsStandard))
	if withFile {
		putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "workflows.md"), stepsWorkspace(hxTwoFileQuick))
	}
	if withNote {
		writeNote(t, home, "alpha", "decisions", "flow", noteText("flow", "member"))
	}
	return where.Resolver{GridHome: home}, brainTask(t, home, "projects/alpha/tasks/T-01-x")
}

// hxTwoWantRoles fails unless GatePostRoles answers person [owner, member] and
// ci [] on top of the default kinds.
func hxTwoWantRoles(t *testing.T, taskDir string) {
	t.Helper()
	got, err := GatePostRoles(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	want := defaultGatePostRoles()
	want["person"] = []string{"owner", "member"}
	want["ci"] = []string{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GatePostRoles = %v, want the note's roles %v", got, want)
	}
}

// Two sources, two answers. The brain workflow note and workflows.md both hold
// a template "Quick". The steps reader and the gate-roles reader read different
// sources, and this test pins what each returns today.
//
// Known gap, not the intended answer: TaskSteps still reads workflows.md. The
// owner decided on 10 Nov that workflows are brain notes (backlog-11nov-hx
// INDEX.md, owner question 1, default: the brain note wins).
func TestHxTwoSourcesDisagree(t *testing.T) {
	r, taskDir := hxTwoHome(t, true, true)
	text := stepsTask("normal", "Quick")

	got, err := stepsString(t, r, "alpha", text)
	if err != nil || got != "ship:run" {
		t.Fatalf("known gap closed or moved: TaskSteps no longer answers from workflows.md when the brain note has the same template. "+
			"If it now answers from the brain note, that is the intended source (owner, 10 Nov): replace this pin with the note's steps. "+
			"Seen: %q, %v", got, err)
	}

	hxTwoWantRoles(t, taskDir)

	ans := ReadFor(r, "alpha", taskDir)
	if ans.Source != SourceProjectBrain || ans.Note != "flow" {
		t.Fatalf("ReadFor source = %q note = %q, want %q note flow", ans.Source, ans.Note, SourceProjectBrain)
	}
	var quick *Template
	for i := range ans.Templates {
		if ans.Templates[i].Name == "Quick" {
			quick = &ans.Templates[i]
		}
	}
	if quick == nil {
		t.Fatalf("ReadFor templates = %+v, want the note's Quick", ans.Templates)
	}
	if got := hxTwoSteps(*quick); got != "Do the work:run,Approve it:approve" {
		t.Fatalf("ReadFor Quick steps = %q, want the note's two steps", got)
	}
}

// The note holds "Quick" and workflows.md does not: the gate roles still come
// from the note, the steps reader finds no such template.
func TestHxTwoNoteOnly(t *testing.T) {
	r, taskDir := hxTwoHome(t, false, true)

	hxTwoWantRoles(t, taskDir)

	steps, err := TaskSteps(r, "alpha", stepsTask("normal", "Quick"))
	if err == nil || len(steps) != 0 {
		t.Fatalf("known gap closed or moved: TaskSteps found a template that only the brain note has; "+
			"that is the intended source (owner, 10 Nov): replace this pin with the note's steps. Seen: %v, %v", steps, err)
	}
	// The error text is printed by namedTemplate (tasksteps.go:152).
	if want := `no workflow is named "Quick"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("TaskSteps error = %v, want it to contain %q", err, want)
	}
}

// workflows.md holds "Quick" and there is no brain note: the steps come from
// the file and the gate roles are the shipped default.
func TestHxTwoFileOnly(t *testing.T) {
	r, taskDir := hxTwoHome(t, true, false)

	got, err := GatePostRoles(taskDir)
	if err != nil || !reflect.DeepEqual(got, defaultGatePostRoles()) {
		t.Fatalf("GatePostRoles = %v, %v; want the default %v", got, err, defaultGatePostRoles())
	}

	steps, err := stepsString(t, r, "alpha", stepsTask("normal", "Quick"))
	if err != nil || steps != "ship:run" {
		t.Fatalf("TaskSteps = %q, %v; want ship:run from workflows.md", steps, err)
	}
}
