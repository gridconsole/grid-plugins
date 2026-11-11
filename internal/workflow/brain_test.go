package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// noteText is a workflow note: the brain note keys (type, name) plus the same
// policy keys and template body a task's workflow.md carries.
func noteText(name, member string) string {
	return "---\ntype: workflow\nname: " + name + "\nstatus: accepted\n" +
		"gate_post_roles:\n  person: [owner, " + member + "]\n  ci: []\n---\n" +
		"# Workflows\n\n## Quick\napplies when: label quick\napprovers: sam, robin\nreview rounds: 2\n\n" +
		"| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n" +
		"| 1 | Do the work | run | grid-builder |  |\n| 2 | Approve it | approve | sam | Do the work |\n"
}

func writeNote(t *testing.T, home, project, topic, name, text string) {
	t.Helper()
	dir := filepath.Join(home, "knowledge", topic)
	if project != "" {
		dir = filepath.Join(home, "projects", project, "knowledge", topic)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeTaskPolicy(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "workflow.md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReadForNoNoteGivesTheDefault(t *testing.T) {
	home := t.TempDir()
	got := ReadFor(where.Resolver{GridHome: home}, "notes-app", t.TempDir())
	if got.Source != SourceDefault || got.Note != "" {
		t.Fatalf("source = %q note = %q, want the default", got.Source, got.Note)
	}
	if !reflect.DeepEqual(got.GatePostRoles, defaultGatePostRoles()) {
		t.Fatalf("roles = %v", got.GatePostRoles)
	}
	if len(got.Errors) != 0 || len(got.Templates) != 0 {
		t.Fatalf("errors = %v templates = %v", got.Errors, got.Templates)
	}
	// An empty home and no task folder behave the same.
	if got := ReadFor(where.Resolver{}, "", ""); got.Source != SourceDefault {
		t.Fatalf("source = %q", got.Source)
	}
}

func TestReadForOrder(t *testing.T) {
	home := t.TempDir()
	writeNote(t, home, "", "decisions", "home-flow", noteText("home-flow", "lead"))
	r := where.Resolver{GridHome: home}

	got := ReadFor(r, "notes-app", "")
	if got.Source != SourceHomeBrain || got.Note != "home-flow" {
		t.Fatalf("home note: source = %q note = %q", got.Source, got.Note)
	}

	writeNote(t, home, "notes-app", "conventions", "project-flow", noteText("project-flow", "member"))
	got = ReadFor(r, "notes-app", "")
	if got.Source != SourceProjectBrain || got.Note != "project-flow" {
		t.Fatalf("project note must beat the home note: source = %q note = %q", got.Source, got.Note)
	}
	if want := []string{"owner", "member"}; !reflect.DeepEqual(got.GatePostRoles["person"], want) {
		t.Fatalf("person roles = %v, want %v", got.GatePostRoles["person"], want)
	}
	if got.GatePostRoles["ci"] == nil || len(got.GatePostRoles["ci"]) != 0 {
		t.Fatalf("an empty list forbids the kind, got %v", got.GatePostRoles["ci"])
	}
	// Another project does not see it and falls to the home note.
	if got := ReadFor(r, "other", ""); got.Source != SourceHomeBrain {
		t.Fatalf("other project: source = %q", got.Source)
	}

	task := writeTaskPolicy(t, noteText("ignored", "lead"))
	got = ReadFor(r, "notes-app", task)
	if got.Source != SourceTask || got.Note != "" {
		t.Fatalf("task workflow.md must come first: source = %q note = %q", got.Source, got.Note)
	}
}

func TestReadForBrokenNoteGivesTheDefaultAndAProblem(t *testing.T) {
	home := t.TempDir()
	r := where.Resolver{GridHome: home}
	broken := "---\ntype: workflow\nname: bad\ngate_post_roles:\n  nobody: [owner]\n---\n# Workflows\n"
	writeNote(t, home, "", "decisions", "bad", broken)

	got := ReadFor(r, "", "")
	if got.Source != SourceDefault {
		t.Fatalf("source = %q, want the default", got.Source)
	}
	if len(got.Errors) != 1 || got.Errors[0].File != "brain:home/bad" || !strings.Contains(got.Errors[0].Message, "unknown gate kind") {
		t.Fatalf("errors = %+v", got.Errors)
	}

	// Front matter that is not valid YAML still gives the default and a problem line.
	writeNote(t, home, "", "decisions", "bad", "---\ntype: workflow\nname: bad\ngate_post_roles: [unclosed\n---\n")
	got = ReadFor(r, "", "")
	if got.Source != SourceDefault || len(got.Errors) != 1 || got.Errors[0].File != "brain:home/bad" {
		t.Fatalf("invalid yaml: source = %q errors = %+v", got.Source, got.Errors)
	}
	// ...and it skips only that file: a good note beside it still answers.
	writeNote(t, home, "", "decisions", "good", noteText("good", "lead"))
	got = ReadFor(r, "", "")
	if got.Source != SourceHomeBrain || got.Note != "good" || len(got.Errors) != 1 || got.Errors[0].File != "brain:home/bad" {
		t.Fatalf("invalid yaml beside a good note: source = %q note = %q errors = %+v", got.Source, got.Note, got.Errors)
	}
	// A non-workflow note with broken front matter stays silent.
	quiet := t.TempDir()
	writeNote(t, quiet, "", "decisions", "other", "---\ntype: decision\nname: other\nx: [unclosed\n---\n")
	if got := ReadFor(where.Resolver{GridHome: quiet}, "", ""); got.Source != SourceDefault || len(got.Errors) != 0 {
		t.Fatalf("non-workflow: source = %q errors = %+v", got.Source, got.Errors)
	}
	writeNote(t, home, "", "decisions", "good", "---\ntype: decision\nname: good\n---\n")

	// A broken project note falls through to a good home note, with its line kept.
	writeNote(t, home, "", "decisions", "bad", noteText("bad", "lead"))
	writeNote(t, home, "notes-app", "conventions", "worse", strings.Replace(broken, "name: bad", "name: worse", 1))
	got = ReadFor(r, "notes-app", "")
	if got.Source != SourceHomeBrain || len(got.Errors) != 1 || got.Errors[0].File != "brain:project/worse" {
		t.Fatalf("source = %q errors = %+v", got.Source, got.Errors)
	}

	// A note whose name is not its file, and a second workflow note, are problems too.
	writeNote(t, home, "x", "decisions", "one", "---\ntype: workflow\nname: other\n---\n")
	if got := ReadFor(r, "x", ""); got.Source != SourceHomeBrain || len(got.Errors) != 1 {
		t.Fatalf("name mismatch: source = %q errors = %+v", got.Source, got.Errors)
	}
	writeNote(t, home, "y", "decisions", "a", noteText("a", "lead"))
	writeNote(t, home, "y", "decisions", "b", noteText("b", "lead"))
	if got := ReadFor(r, "y", ""); got.Source != SourceHomeBrain || len(got.Errors) != 1 || !strings.Contains(got.Errors[0].Message, "more than one") {
		t.Fatalf("two notes: source = %q errors = %+v", got.Source, got.Errors)
	}

	// A broken task workflow.md also falls through instead of stopping the task.
	task := writeTaskPolicy(t, "not front matter\n")
	got = ReadFor(where.Resolver{GridHome: t.TempDir()}, "", task)
	if got.Source != SourceDefault || len(got.Errors) != 1 || got.Errors[0].File != "workflow.md" {
		t.Fatalf("task: source = %q errors = %+v", got.Source, got.Errors)
	}
}

func TestReadForSkipsNotesThatAreNotAcceptedWorkflows(t *testing.T) {
	home := t.TempDir()
	r := where.Resolver{GridHome: home}
	writeNote(t, home, "", "decisions", "plain", "---\ntype: decision\nname: plain\ngate_post_roles:\n  nobody: [owner]\n---\nbody\n")
	writeNote(t, home, "", "decisions", "draft", strings.Replace(noteText("draft", "lead"), "status: accepted", "status: proposed", 1))
	writeNote(t, home, "", "decisions", "prose", "no front matter at all\n")
	got := ReadFor(r, "", "")
	if got.Source != SourceDefault || len(got.Errors) != 0 {
		t.Fatalf("source = %q errors = %+v", got.Source, got.Errors)
	}
}

// One parser for both places: the same text read from a task's workflow.md and
// from a brain note gives the same policy and templates.
func TestWorkflowNoteAndWorkflowMdShareOneParser(t *testing.T) {
	text := noteText("flow", "lead")
	fromTask := ReadFor(where.Resolver{GridHome: t.TempDir()}, "notes-app", writeTaskPolicy(t, text))

	home := t.TempDir()
	writeNote(t, home, "notes-app", "conventions", "flow", text)
	fromNote := ReadFor(where.Resolver{GridHome: home}, "notes-app", "")

	if fromTask.Source != SourceTask || fromNote.Source != SourceProjectBrain {
		t.Fatalf("sources = %q, %q", fromTask.Source, fromNote.Source)
	}
	if len(fromTask.Templates) != 1 || len(fromTask.Templates[0].Steps) != 2 || !reflect.DeepEqual(fromTask.Templates[0].Approvers, []string{"sam", "robin"}) {
		t.Fatalf("templates = %+v", fromTask.Templates)
	}
	// Only where it was read from differs: Source, Note and each File label.
	for _, a := range []*Answer{&fromTask, &fromNote} {
		a.Source, a.Note = "", ""
		for i := range a.Templates {
			a.Templates[i].File = ""
		}
	}
	if !reflect.DeepEqual(fromTask, fromNote) {
		t.Fatalf("task answer %+v\nnote answer %+v", fromTask, fromNote)
	}
}
