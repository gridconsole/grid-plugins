package job_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// These tests pin how the Job source reader handles note links, task-file front
// matter, headings, task folder names, plan.md reads and a broken Dependency.

// jsrNamedTask makes an empty task folder with the given name.
func jsrNamedTask(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestResolveBrainNoteCopiesItsLinksOntoTheSource(t *testing.T) {
	e := srNote("project", "deploy", "accepted", "# Deploy\nRun it.\n")
	e.Note.Links = []brain.Link{{To: "brain:project/other", Rel: "uses", Why: "because"}}
	notes := &srNotes{entries: map[string]brain.Entry{e.ID: e}}

	s, err := (&job.Files{Task: srTask(t), Notes: notes}).Resolve("brain:project/deploy")
	if err != nil {
		t.Fatal(err)
	}
	want := []job.Link{{To: "brain:project/other", Rel: "uses", Why: "because"}}
	if !reflect.DeepEqual(s.Links, want) {
		t.Errorf("links %+v, want %+v", s.Links, want)
	}
}

func TestResolveTaskFileWithBrokenFrontMatterIsRefused(t *testing.T) {
	task := srTask(t)
	srWrite(t, task, "bad.md", "---\nx: [\n---\nbody\n")

	_, err := (&job.Files{Task: task}).Resolve("bad.md")
	if err == nil || !strings.HasPrefix(err.Error(), "bad.md: frontmatter:") {
		t.Errorf("error %v, want one starting %q", err, "bad.md: frontmatter:")
	}
}

func TestResolveTaskFileSummaryFallsBackToItsTitle(t *testing.T) {
	task := srTask(t)
	srWrite(t, task, "titled.md", "---\ntitle: The title\n---\nbody\n")

	s, err := (&job.Files{Task: task}).Resolve("titled.md")
	if err != nil {
		t.Fatal(err)
	}
	if s.Summary != "The title" {
		t.Errorf("summary %q, want %q", s.Summary, "The title")
	}
}

func TestResolveAnchorSkipsAHashLineThatIsNotAHeading(t *testing.T) {
	task := srTask(t)
	srWrite(t, task, "h.md", "#nospace\nline\n# Real\nreal body\n")
	f := &job.Files{Task: task}

	_, err := f.Resolve("h.md#nospace")
	if err == nil || err.Error() != `h.md#nospace: anchor "nospace" not found` {
		t.Errorf("error %v, want %q", err, `h.md#nospace: anchor "nospace" not found`)
	}
	s, err := f.Resolve("h.md#real")
	if err != nil {
		t.Fatal(err)
	}
	if s.Text != "# Real\nreal body" {
		t.Errorf("text %q, want %q", s.Text, "# Real\nreal body")
	}
}

func TestResolveJobPointerForATaskFolderWithoutADashNeedsAnIndexResolver(t *testing.T) {
	f := &job.Files{Task: jsrNamedTask(t, "task")}

	_, err := f.Resolve("job:T-07/J-1")
	want := "job:T-07/J-1: cross-task Job requires an index resolver"
	if err == nil || err.Error() != want {
		t.Errorf("error %v, want %q", err, want)
	}
}

func TestLoadPlanNamesAPlanThatCannotBeRead(t *testing.T) {
	dir := srTask(t)
	if err := os.MkdirAll(filepath.Join(dir, "plan.md"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := job.LoadPlan(dir, "J-1")
	if err == nil {
		t.Fatal("want an error, got none")
	}
	// The path may print in its symlink-resolved form, so check its last element.
	if !strings.HasPrefix(err.Error(), "read ") || !strings.Contains(err.Error(), string(filepath.Separator)+"plan.md") {
		t.Errorf("error %q, want one starting %q and naming plan.md", err, "read ")
	}
}

func TestDependencyUnmarshalJSONRefusesABrokenString(t *testing.T) {
	var d job.Dependency
	err := d.UnmarshalJSON([]byte(`"\q"`))
	if err == nil || !strings.Contains(err.Error(), "invalid escape sequence") {
		t.Errorf("error %v, want one containing %q", err, "invalid escape sequence")
	}
	if d != (job.Dependency{}) {
		t.Errorf("dependency %+v, want zero", d)
	}
}
