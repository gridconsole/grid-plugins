package jobcmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const (
	recallDeployDesc = "How we deploy the web app to Netlify"
	recallTitle      = "- [ ] 1 Document the export format\n"
)

type recallInputs struct {
	BrainTokens int `json:"brain_tokens"`
	Sources     []struct {
		Pointer string `json:"pointer"`
		Summary string `json:"summary"`
	} `json:"sources"`
	NotRendered []string `json:"not_rendered"`
}

func (in recallInputs) count(pointer string) int {
	n := 0
	for _, s := range in.Sources {
		if s.Pointer == pointer {
			n++
		}
	}
	return n
}

func recallParse(t *testing.T, inputs string) recallInputs {
	t.Helper()
	var in recallInputs
	if err := json.Unmarshal([]byte(inputs), &in); err != nil {
		t.Fatal(err)
	}
	return in
}

// recallPlan edits the fixture plan, then records the approval of the new text.
func recallPlan(t *testing.T, e Env, edits map[string]string) {
	t.Helper()
	path := filepath.Join(e.Cwd, "T-01", "plan.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for old, now := range edits {
		if !strings.Contains(text, old) {
			t.Fatalf("fixture plan lost %q", old)
		}
		text = strings.Replace(text, old, now, 1)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	(jobtest.Home{GridHome: e.GridHome, Task: filepath.Join(e.Cwd, "T-01")}).Approve(t)
}

// recallDeployFixture is a Job titled "Deploy the export page" and an accepted
// project note `deploy` that shares the word "deploy" with it.
func recallDeployFixture(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer, brain.Store) {
	t.Helper()
	e, out, errs := fixture(t)
	h := jobtest.Home{GridHome: e.GridHome, Task: filepath.Join(e.Cwd, "T-01")}
	h.Store = brain.Store{GridHome: e.GridHome, Project: jobtest.Project}
	h.Note("project", "deploy", "conventions", recallDeployDesc, "Run the Netlify deploy from main.\n")
	recallPlan(t, e, map[string]string{recallTitle: "- [ ] 1 Deploy the export page\n"})
	return e, out, errs, h.Store
}

// recallNoBrain strips every brain pointer the fixture Job names.
var recallNoBrain = map[string]string{
	"      links:\n        - {to: brain:project/export-format, rel: uses, why: \"Keep the documented column order stable for readers\"}\n": "      links: []\n",
	"keep: [brain:project/export-format]": "keep: []",
	"read: [task:research.md#format]":     "read: []",
}

func TestRenderRecallsANoteThatMatchesTheTitle(t *testing.T) {
	e, out, errs, _ := recallDeployFixture(t)
	brief, inputs := render(t, e, out, errs)
	in := recallParse(t, inputs)
	if in.BrainTokens == 0 {
		t.Fatalf("brain_tokens is 0:\n%s", inputs)
	}
	if n := in.count("brain:project/deploy"); n != 1 {
		t.Fatalf("%d sources with pointer brain:project/deploy, want 1:\n%s", n, inputs)
	}
	for _, s := range in.Sources {
		if s.Pointer == "brain:project/deploy" && s.Summary != recallDeployDesc {
			t.Fatalf("summary %q, want %q", s.Summary, recallDeployDesc)
		}
	}
	line := "- brain:project/deploy · uses · " + recallDeployDesc + " · " + recallWhy + " ["
	if n := strings.Count(brief, line); n != 1 {
		t.Fatalf("%d pointer blocks for the recalled note, want 1:\n%s", n, brief)
	}
	if !strings.Contains(brief, "### Excerpt [") || !strings.Contains(brief, "(brain:project/deploy)") {
		t.Fatalf("brief lacks the recalled note's excerpt:\n%s", brief)
	}
}

func TestRecalledLinksCarryARelAndAWhy(t *testing.T) {
	e, _, _, _ := recallDeployFixture(t)
	task := filepath.Join(e.Cwd, "T-01")
	w, err := sources(task, e)
	if err != nil {
		t.Fatal(err)
	}
	j, err := loadJob(task, "1")
	if err != nil {
		t.Fatal(err)
	}
	links := w.recall(j)
	if len(links) == 0 {
		t.Fatal("no recall")
	}
	for _, l := range links {
		if l.Rel != "uses" || l.Why != recallWhy || !strings.HasPrefix(l.To, "brain:") {
			t.Errorf("%+v", l)
		}
	}
}

func TestRecalledNoteAlsoNamedInLinksIsRenderedOnce(t *testing.T) {
	e, out, errs, _ := recallDeployFixture(t)
	named := "      links:\n        - {to: brain:project/deploy, rel: uses, why: \"The planner wants the deploy steps kept\"}\n"
	recallPlan(t, e, map[string]string{"      links:\n": named})
	brief, inputs := render(t, e, out, errs)
	in := recallParse(t, inputs)
	if n := in.count("brain:project/deploy"); n != 1 {
		t.Fatalf("%d sources, want 1:\n%s", n, inputs)
	}
	if n := strings.Count(brief, "- brain:project/deploy · "); n != 1 {
		t.Fatalf("%d blocks, want 1:\n%s", n, brief)
	}
	if !strings.Contains(brief, "· The planner wants the deploy steps kept [") || strings.Contains(brief, recallWhy) {
		t.Fatalf("the planner's own link lost to the recalled one:\n%s", brief)
	}
}

func TestSupersededNoteIsNeverRecalled(t *testing.T) {
	e, out, errs, store := recallDeployFixture(t)
	h := jobtest.Home{GridHome: e.GridHome, Store: store}
	old := h.Note("project", "old-deploy", "conventions", "Deploy by hand to the old server", "Deploy by hand.\n")
	path := filepath.Join(e.GridHome, filepath.FromSlash(old.Path))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(b), "superseded_by: ''", "superseded_by: brain:project/deploy", 1)
	if edited == string(b) {
		t.Fatalf("note file has no superseded_by line:\n%s", b)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	_, inputs := render(t, e, out, errs)
	in := recallParse(t, inputs)
	if in.count("brain:project/old-deploy") != 0 {
		t.Fatalf("a superseded note was recalled:\n%s", inputs)
	}
	if in.count("brain:project/deploy") != 1 {
		t.Fatalf("the current note was not recalled:\n%s", inputs)
	}
}

// A Personal task has no project, so recall searches the Grid home's brain only.
func TestPersonalJobRecallsFromTheHomeBrainOnly(t *testing.T) {
	plan, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "job", "task", "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(plan)
	for old, now := range recallNoBrain {
		if !strings.Contains(text, old) {
			t.Fatalf("fixture plan lost %q", old)
		}
		text = strings.Replace(text, old, now, 1)
	}
	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatal(err)
	}
	gridHome := filepath.Join(home, ".grid")
	ts := task.Store{Resolver: where.Resolver{GridHome: gridHome, Home: home, Ceiling: root}}
	made, err := ts.New(task.NewOptions{Cwd: home, Project: where.Personal, Words: "I want the export format documented."})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(made.Path, "plan.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.Change(task.WriteOptions{Cwd: made.Path, ID: made.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	(jobtest.Home{GridHome: gridHome, Task: made.Path}).Approve(t)
	// One note in the home brain and one in a project brain; both match the title.
	put := func(project, level, name string) {
		s := brain.Store{GridHome: gridHome, Project: project}
		n := brain.Note{Name: name, Topic: "decisions", Type: "convention", Description: "How the export format is documented", Summary: "How the export format is documented", Source: "fixture", Level: level, Body: "Body.\n"}
		if _, err := s.Propose(n, brain.Origin{Kind: "session", ID: "fixture"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Accept(level, name); err != nil {
			t.Fatal(err)
		}
	}
	put("", "home", "export-habit")
	put("demo", "project", "export-rule")
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	e := Env{Resolver: ownerResolver(gridHome), Cwd: home, GridHome: gridHome, Machine: jobtest.Machine, Stdout: out, Stderr: errs}
	r := call(t, e, out, errs, 0, "render", made.Path, "1", "--json")
	b, err := os.ReadFile(r.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	in := recallParse(t, string(b))
	if in.count("brain:home/export-habit") != 1 {
		t.Fatalf("the home note was not recalled:\n%s", b)
	}
	if in.count("brain:project/export-rule") != 0 {
		t.Fatalf("a project note reached a Personal Job:\n%s", b)
	}
}

func TestNoBrainRendersWithoutRecall(t *testing.T) {
	e, out, errs := fixture(t)
	recallPlan(t, e, recallNoBrain)
	knowledge := filepath.Join(e.GridHome, "projects", jobtest.Project, "knowledge")
	if err := os.RemoveAll(knowledge); err != nil {
		t.Fatal(err)
	}
	_, inputs := render(t, e, out, errs)
	in := recallParse(t, inputs)
	if in.BrainTokens != 0 || in.count("brain:project/export-format") != 0 {
		t.Fatalf("recall without a brain:\n%s", inputs)
	}
	if errs.Len() != 0 {
		t.Fatalf("stderr: %s", errs)
	}
}

func TestSearchErrorRendersWithoutRecall(t *testing.T) {
	e, out, errs := fixture(t)
	recallPlan(t, e, recallNoBrain)
	knowledge := filepath.Join(e.GridHome, "projects", jobtest.Project, "knowledge")
	if err := os.RemoveAll(knowledge); err != nil {
		t.Fatal(err)
	}
	// A symbolic link inside the knowledge folder makes the search fail.
	if err := os.MkdirAll(filepath.Join(knowledge, "conventions"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(e.GridHome, filepath.Join(knowledge, "conventions", "link.md")); err != nil {
		t.Skip("symbolic links are not available:", err)
	}
	task := filepath.Join(e.Cwd, "T-01")
	w, err := sources(task, e)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.store.Search("export", "", recallLimit); err == nil {
		t.Fatal("the fixture should make Search fail")
	}
	j, err := loadJob(task, "1")
	if err != nil {
		t.Fatal(err)
	}
	if links := w.recall(j); links != nil {
		t.Fatalf("recall after a search error: %+v", links)
	}
	// The same fault already stops the render at the generation check on main,
	// before recall runs, so the entry exits as on main and recall adds no text.
	r := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if !strings.Contains(r.Error, "symbolic links") || errs.Len() != 0 {
		t.Fatalf("error %q stderr %q", r.Error, errs)
	}
}

func TestRecallRenderIsByteIdentical(t *testing.T) {
	e, out, errs, _ := recallDeployFixture(t)
	brief1, inputs1 := render(t, e, out, errs)
	brief2, inputs2 := render(t, e, out, errs)
	if brief1 != brief2 || inputs1 != inputs2 {
		t.Fatal("two renders of the same Job on the same brain differ")
	}
	if !strings.Contains(inputs1, "brain:project/deploy") {
		t.Fatalf("the renders hold no recall:\n%s", inputs1)
	}
}
