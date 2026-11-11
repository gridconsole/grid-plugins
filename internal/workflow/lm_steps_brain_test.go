package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// lmSbQuick is a "Quick" section whose one step is named step.
func lmSbQuick(step string) string {
	return "## Quick\napplies when: label quick\nreview rounds: 2\n\n" + stepsTable +
		"| 1 | " + step + " | run | grid-builder | all | |\n\n"
}

// lmSbDefault is a default "Standard" section whose one step is named step.
func lmSbDefault(step string) string {
	return "## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + stepsTable +
		"| 1 | " + step + " | run | grid-builder | all | |\n\n"
}

const lmSbHomeOnly = "## Homeonly\napplies when: label home\nreview rounds: 2\n\n" + stepsTable +
	"| 1 | home-only | run | grid-builder | all | |\n\n"

// lmSbNote is a workflow note with the given template sections and no policy.
func lmSbNote(name string, sections ...string) string {
	return "---\ntype: workflow\nname: " + name + "\nstatus: accepted\n---\n" + stepsWorkspace(sections...)
}

// lmSbHome lays out projects/alpha with its record and the workspace file.
func lmSbHome(t *testing.T, workspace string) string {
	t.Helper()
	home := t.TempDir()
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	putWorkflowFixture(t, filepath.Join(home, "workflows.md"), workspace)
	return home
}

func lmSbWant(t *testing.T, r where.Resolver, project, task, want string) {
	t.Helper()
	got, err := stepsString(t, r, project, task)
	if err != nil || got != want {
		t.Fatalf("TaskSteps(%s) = %q, %v; want %q", project, got, err, want)
	}
}

func lmSbRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}

// Row 1: a template only in the project note drives the task's steps.
func TestLmSbProjectNoteDrivesSteps(t *testing.T) {
	home := lmSbHome(t, stepsWorkspace(stepsStandard))
	writeNote(t, home, "alpha", "decisions", "flow", noteText("flow", "member"))
	r := where.Resolver{GridHome: home}
	task := stepsTask("normal", "Quick")

	tpl, _, err := TaskWorkflow(r, "alpha", task)
	if tpl == nil || tpl.File != "brain:project/flow" {
		t.Fatalf("TaskWorkflow template = %+v, %v; want the note's Quick from brain:project/flow", tpl, err)
	}
	if got := hxTwoSteps(*tpl); got != "Do the work:run,Approve it:approve" {
		t.Fatalf("TaskWorkflow Quick steps = %q, want the note's two steps", got)
	}
	// noteText's step names hold spaces, which the steps reader refuses as it
	// does in workflows.md (TestTaskSteps "step name with a space").
	if err == nil || !strings.Contains(err.Error(), `workflow "Quick" step "Do the work" is not a usable step name`) {
		t.Fatalf("TaskWorkflow error = %v, want the note's step name refused", err)
	}

	// The same note with usable step names gives the task its steps.
	slug := strings.NewReplacer("Do the work", "do-the-work", "Approve it", "approve-it").Replace(noteText("flow", "member"))
	writeNote(t, home, "alpha", "decisions", "flow", slug)
	lmSbWant(t, r, "alpha", task, "do-the-work:run,approve-it:approve")
}

// Row 2: the project note, then the home note, then the project file, then
// the workspace file; a rejected note falls through; a note's default wins.
func TestLmSbOrder(t *testing.T) {
	home := lmSbHome(t, stepsWorkspace(stepsStandard, lmSbQuick("ws-step")))
	projectFile := filepath.Join(home, "projects", "alpha", "workflows.md")
	putWorkflowFixture(t, projectFile, stepsWorkspace(lmSbQuick("file-step")))
	projectNote := filepath.Join(home, "projects", "alpha", "knowledge", "decisions", "pflow.md")
	homeNote := filepath.Join(home, "knowledge", "decisions", "hflow.md")
	writeNote(t, home, "alpha", "decisions", "pflow", lmSbNote("pflow", lmSbQuick("project-note")))
	writeNote(t, home, "", "decisions", "hflow", lmSbNote("hflow", lmSbQuick("home-note")))
	r := where.Resolver{GridHome: home}
	task := stepsTask("normal", "Quick")

	lmSbWant(t, r, "alpha", task, "project-note:run")

	// A second workflow note at the project level rejects that level.
	writeNote(t, home, "alpha", "conventions", "pflow2", lmSbNote("pflow2", lmSbQuick("second-note")))
	lmSbWant(t, r, "alpha", task, "home-note:run")
	const twoNotes = "more than one workflow note; keep one"
	if ans := ReadFor(r, "alpha", ""); !lmSbHasProblem(ans, twoNotes) {
		t.Fatalf("ReadFor errors = %v, want %q for the project level", ans.Errors, twoNotes)
	}
	if ans, err := Read(r, "alpha"); err != nil || lmSbHasProblem(ans, twoNotes) {
		t.Fatalf("Read errors = %v, %v; want the rejected note's problems dropped", ans.Errors, err)
	}
	lmSbRemove(t, filepath.Join(home, "projects", "alpha", "knowledge", "conventions", "pflow2.md"))

	lmSbRemove(t, projectNote)
	lmSbWant(t, r, "alpha", task, "home-note:run")
	lmSbRemove(t, homeNote)
	lmSbWant(t, r, "alpha", task, "file-step:run")
	lmSbRemove(t, projectFile)
	lmSbWant(t, r, "alpha", task, "ws-step:run")
}

// Row 2: a task that names no workflow takes the project note's default, then
// the home note's, then the workspace default.
func TestLmSbNoteDefault(t *testing.T) {
	home := lmSbHome(t, stepsWorkspace(stepsStandard))
	projectNote := filepath.Join(home, "projects", "alpha", "knowledge", "decisions", "pflow.md")
	homeNote := filepath.Join(home, "knowledge", "decisions", "hflow.md")
	writeNote(t, home, "alpha", "decisions", "pflow", lmSbNote("pflow", lmSbDefault("project-default")))
	writeNote(t, home, "", "decisions", "hflow", lmSbNote("hflow", lmSbDefault("home-default")))
	r := where.Resolver{GridHome: home}
	task := stepsTask("normal", "-")

	tpl, _, err := TaskWorkflow(r, "alpha", task)
	if err != nil || tpl == nil || tpl.File != "brain:project/pflow" {
		t.Fatalf("TaskWorkflow = %+v, %v; want the project note's default", tpl, err)
	}
	lmSbWant(t, r, "alpha", task, "project-default:run")
	lmSbRemove(t, projectNote)
	lmSbWant(t, r, "alpha", task, "home-default:run")
	lmSbRemove(t, homeNote)
	lmSbWant(t, r, "alpha", task, "plan:approve,build:run")
}

// Row 3: a guest never gets a home-note template and gets a project note only
// for a project it is invited to; a Personal task gets the home note's.
func TestLmSbGuestAndPersonal(t *testing.T) {
	home := lmSbHome(t, stepsWorkspace(stepsStandard, lmSbQuick("ws-step")))
	putWorkflowFixture(t, filepath.Join(home, "projects", "beta", "project.md"), "---\nid: id-beta\n---\n")
	putWorkflowFixture(t, filepath.Join(home, "projects", "alpha", "project-roles.json"), `{"people":[{"id":"guest@example.com"}]}`)
	writeNote(t, home, "alpha", "decisions", "aflow", lmSbNote("aflow", lmSbQuick("alpha-note")))
	writeNote(t, home, "beta", "decisions", "bflow", lmSbNote("bflow", lmSbQuick("beta-note")))
	writeNote(t, home, "", "decisions", "hflow", lmSbNote("hflow", lmSbQuick("home-note"), lmSbHomeOnly))
	guest := where.Resolver{GridHome: home, Caller: where.Caller{Role: "guest", Member: "guest@example.com"}}
	person := where.Resolver{GridHome: home}

	lmSbWant(t, guest, "alpha", stepsTask("normal", "Quick"), "alpha-note:run")
	if _, err := TaskSteps(guest, "alpha", stepsTask("normal", "Homeonly")); err == nil || !strings.Contains(err.Error(), `no workflow is named "Homeonly"`) {
		t.Fatalf("guest Homeonly = %v, want no such workflow", err)
	}
	ans, err := Read(guest, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	for _, tpl := range ans.Templates {
		if strings.HasPrefix(tpl.File, "brain:home/") {
			t.Fatalf("guest Read lists a home-note template: %+v", tpl)
		}
	}
	if _, err := TaskSteps(guest, "beta", stepsTask("normal", "Quick")); err == nil {
		t.Fatal("guest got steps in a project it is not invited to")
	}
	// The helper's own check, apart from the file reader's project filter.
	if got, _ := lmSbBrainTemplates(guest, "beta"); len(got) != 0 {
		t.Fatalf("guest note templates for beta = %+v, want none", got)
	}

	lmSbWant(t, person, "beta", stepsTask("normal", "Quick"), "beta-note:run")
	lmSbWant(t, person, "personal", stepsTask("normal", "Homeonly"), "home-only:run")
	lmSbWant(t, person, "personal", stepsTask("normal", "Quick"), "home-note:run")

	// No project: the home note beside the files, no project note.
	all, err := Read(person, "")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, tpl := range all.Templates {
		files = append(files, tpl.File+":"+tpl.Name)
	}
	if got := strings.Join(files, ","); got != "brain:home/hflow:Quick,brain:home/hflow:Homeonly,workflows.md:Standard,workflows.md:Quick" {
		t.Fatalf("Read with no project templates = %s", got)
	}
	if all.Source != "" || all.Note != "" || all.GatePostRoles != nil {
		t.Fatalf("Read set source %q note %q roles %v; want them empty", all.Source, all.Note, all.GatePostRoles)
	}
}

func lmSbHasProblem(ans Answer, message string) bool {
	for _, p := range ans.Errors {
		if p.Message == message {
			return true
		}
	}
	return false
}
