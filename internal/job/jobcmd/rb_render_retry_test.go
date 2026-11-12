package jobcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// A failed attempt is made Ready again under the same Job id and renders again.
// An unchanged Job whose brain alone changed reuses its first saved render and
// keeps the brain it was first given; a changed Job, plan or file is still
// refused and needs a new Job id.

const (
	rbRetryChangedLine = "A constraint added after the first attempt."
	rbRetryRefusal     = "Job J-1 already has different saved inputs; create a new Job for revised work"
)

var rbRetryNames = []string{"brief.md", "inputs.json", "job.yaml"}

type rbRetryFile struct {
	data []byte
	mod  time.Time
}

// rbRetryRead returns the three saved files of a Job folder with their bytes and
// modification times, so a rewrite shows even when it wrote the same bytes.
func rbRetryRead(t *testing.T, dir string) map[string]rbRetryFile {
	t.Helper()
	got := map[string]rbRetryFile{}
	for _, name := range rbRetryNames {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = rbRetryFile{b, info.ModTime()}
	}
	return got
}

// rbRetryUnchanged fails when any saved file differs from before in bytes or time.
func rbRetryUnchanged(t *testing.T, dir string, before map[string]rbRetryFile) {
	t.Helper()
	after := rbRetryRead(t, dir)
	for _, name := range rbRetryNames {
		if !bytes.Equal(before[name].data, after[name].data) {
			t.Errorf("%s changed:\nbefore %q\nafter  %q", name, before[name].data, after[name].data)
		}
		if !before[name].mod.Equal(after[name].mod) {
			t.Errorf("%s was rewritten: modified %v, then %v", name, before[name].mod, after[name].mod)
		}
	}
}

// rbRetryAppendNote adds a line to the project note the Job recalls: the brain
// change a retried attempt meets between its first render and its second.
func rbRetryAppendNote(t *testing.T, gridHome string) {
	t.Helper()
	note := filepath.Join(gridHome, "projects", jobtest.Project, "knowledge", "conventions", "export-format.md")
	f, err := os.OpenFile(note, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString(rbRetryChangedLine + "\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRbRetryBrainChangeReusesFirstRender(t *testing.T) {
	e, out, errs := fixture(t)
	first := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	dir := filepath.Dir(first.Path)
	before := rbRetryRead(t, dir)
	if !strings.Contains(string(before["brief.md"].data), "Keep the id column before the title column.") {
		t.Fatalf("the first brief does not carry the recalled note:\n%s", before["brief.md"].data)
	}
	rbRetryAppendNote(t, e.GridHome)
	second := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if !second.OK || second.ID != "J-1" || second.Path != first.Path || second.Inputs != first.Inputs {
		t.Fatalf("second render %+v; want the first render's brief and inputs %+v", second, first)
	}
	rbRetryUnchanged(t, dir, before)
	for _, name := range rbRetryNames {
		if strings.Contains(string(rbRetryRead(t, dir)[name].data), rbRetryChangedLine) {
			t.Errorf("%s carries the brain the Job was not first given", name)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "J-1" {
		t.Errorf("jobs/ holds %d entries; want only J-1", len(entries))
	}
}

func TestRbRetryNoSecondBrainGiven(t *testing.T) {
	e, _, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the deploy script from the release branch.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	first := historyLines(t, hist)
	if len(given(first)) != 2 {
		t.Fatalf("the first render wants a brain given line for deploy and for export-format, got %q", given(first))
	}
	rbRetryAppendNote(t, e.GridHome)
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	second := historyLines(t, hist)
	if len(given(second)) != len(given(first)) {
		t.Fatalf("the second render added brain given lines: %q", given(second))
	}
	if len(second) != len(first) {
		t.Fatalf("the second render added history lines: %q", second[len(first):])
	}
}

func TestRbRetryChangedJobStillRefused(t *testing.T) {
	task := t.TempDir()
	j := job.Job{ID: "J-1", Title: "First title"}
	path, err := save(task, j, job.Rendered{Brief: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	before := rbRetryRead(t, dir)
	revised := j
	revised.Title = "Revised title"
	got, err := save(task, revised, job.Rendered{Brief: "brief"})
	if err == nil || got != "" || err.Error() != rbRetryRefusal {
		t.Fatalf("save of a changed Job = %q, %v; want %q", got, err, rbRetryRefusal)
	}
	rbRetryUnchanged(t, dir, before)
}

// A person revises the plan after the first render: the Job's own fields stay as
// they were, so job.yaml is equal, but the plan text the Job points at changed.
// The render is refused, so a builder never works on the old plan unaware.
func TestRbRetryPlanChangeStillRefused(t *testing.T) {
	e, out, errs := fixture(t)
	first := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	dir := filepath.Dir(first.Path)
	before := rbRetryRead(t, dir)
	f, err := os.OpenFile(filepath.Join(e.Cwd, "T-01", "plan.md"), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("\nThe export also documents compatibility.\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	second := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if second.OK || second.Error != rbRetryRefusal {
		t.Fatalf("render after a plan edit %+v; want the refusal %q", second, rbRetryRefusal)
	}
	rbRetryUnchanged(t, dir, before)
}

// The same Job saved again with another brief and other inputs finds the first
// files and writes nothing.
func TestRbRetrySaveReportReusesSavedBundle(t *testing.T) {
	task := t.TempDir()
	j := job.Job{ID: "J-1", Title: "Title"}
	path, wrote, err := saveReport(task, j, job.Rendered{Brief: "first brief"})
	if err != nil || !wrote {
		t.Fatalf("first save = %q, wrote %v, %v", path, wrote, err)
	}
	dir := filepath.Dir(path)
	before := rbRetryRead(t, dir)
	again, wrote, err := saveReport(task, j, job.Rendered{Brief: "a brief from a changed brain"})
	if err != nil || wrote || again != path {
		t.Fatalf("second save = %q, wrote %v, %v; want %q, wrote false", again, wrote, err, path)
	}
	rbRetryUnchanged(t, dir, before)
}

// An unchanged Job whose saved bundle lost a file is not reused and not repaired.
func TestRbRetryMissingSavedFileStillRefused(t *testing.T) {
	task := t.TempDir()
	j := job.Job{ID: "J-1"}
	path, err := save(task, j, job.Rendered{Brief: "brief"})
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(filepath.Dir(path), "inputs.json")
	if err = os.Remove(missing); err != nil {
		t.Fatal(err)
	}
	got, err := save(task, j, job.Rendered{Brief: "brief"})
	if err == nil || got != "" || err.Error() != rbRetryRefusal {
		t.Fatalf("save with inputs.json gone = %q, %v; want %q", got, err, rbRetryRefusal)
	}
	if _, err = os.Lstat(missing); !os.IsNotExist(err) {
		t.Fatalf("the refused save recreated inputs.json: %v", err)
	}
}
