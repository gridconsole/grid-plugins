package jobcmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/task"
)

const exportLinkLine = "        - {to: brain:project/export-format, rel: uses, why: \"Keep the documented column order stable for readers\"}\n"

// givenFixture is the job fixture with its plan's brain links replaced by links,
// the plan approved again, and an empty history.md in the task.
func givenFixture(t *testing.T, links []string, notes func(h jobtest.Home)) (Env, string, string) {
	t.Helper()
	h := jobtest.New(t, filepath.Join("..", "..", "..", "testdata"))
	if notes != nil {
		notes(h)
	}
	planPath := filepath.Join(h.Task, "plan.md")
	b, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if !strings.Contains(text, exportLinkLine) || !strings.Contains(text, "keep: [brain:project/export-format]") {
		t.Fatal("fixture plan lost its brain link")
	}
	var sb strings.Builder
	for _, l := range links {
		sb.WriteString("        - {to: " + l + ", rel: uses, why: \"Carry this note into the Job\"}\n")
	}
	text = strings.Replace(text, exportLinkLine, sb.String(), 1)
	text = strings.Replace(text, "keep: [brain:project/export-format]", "keep: []", 1)
	if err := os.WriteFile(planPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	hist := filepath.Join(h.Task, "history.md")
	if err := os.WriteFile(hist, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs := newBuffers()
	e := Env{Resolver: ownerResolver(h.GridHome), Cwd: filepath.Dir(h.Task), GridHome: h.GridHome, Machine: jobtest.Machine, Stdout: out, Stderr: errs}
	return e, h.Task, hist
}

func newBuffers() (*bytes.Buffer, *bytes.Buffer) { return &bytes.Buffer{}, &bytes.Buffer{} }

func historyLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func given(lines []string) []string {
	var got []string
	for _, l := range lines {
		if strings.Contains(l, " · brain given · ") {
			got = append(got, l)
		}
	}
	return got
}

func TestRenderLogsBrainGivenOnce(t *testing.T) {
	e, taskDir, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the deploy script from the release branch.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	carried := []string{"brain:project/deploy", "brain:project/export-format"}
	brief, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	// Prove each logged note is really in the brief the agent gets.
	for _, want := range []string{"Run the deploy script from the release branch.", "Keep the id column before the title column."} {
		if !strings.Contains(string(brief), want) {
			t.Fatalf("the brief does not carry %q; the given lines would not match it", want)
		}
	}
	all := historyLines(t, hist)
	got := given(all)
	// One line per note the brief carries: the linked deploy note, and the
	// export-format note that recall adds because it matches the Job's title
	// ("Export documentation"). Both are pasted into the brief, so both are given.
	if len(got) != len(carried) {
		t.Fatalf("want one brain given line per carried note (%d), got %d in %q", len(carried), len(got), all)
	}
	// The deploy note is linked, so its line ends with the link's why; the recalled
	// export-format note has no link and no why.
	whys := map[string]string{"brain:project/deploy": " · why Carry this note into the Job"}
	for i, want := range carried {
		if !strings.HasSuffix(got[i], " · brain given · "+want+" · by grid job render · via J-1"+whys[want]) {
			t.Errorf("line %q does not end with the line for %s", got[i], want)
		}
	}
	// The same Job rendered again with unchanged inputs adds nothing.
	call(t, e, out, errs, 0, "render", "T-01", "J-1", "--json")
	if again := given(historyLines(t, hist)); len(again) != len(carried) {
		t.Fatalf("second render changed the brain given lines: %q", again)
	}
	_ = taskDir
}

func TestRenderLogsNoLineForCutHomeAndFileSources(t *testing.T) {
	names := []string{}
	for i := 1; i <= 9; i++ {
		names = append(names, fmt.Sprintf("note-%d", i))
	}
	links := []string{"brain:home/my-habits"}
	for _, n := range names {
		links = append(links, "brain:project/"+n)
	}
	e, _, hist := givenFixture(t, links, func(h jobtest.Home) {
		h.Note("home", "my-habits", "decisions", "A personal habit.", "Personal body.\n")
		for _, n := range names {
			h.Note("project", n, "conventions", "Summary of "+n+".", "Body of "+n+".\n")
		}
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	in, err := os.ReadFile(r.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	// The render cuts the ninth brain candidate: the Job keeps at most eight.
	cut := ""
	for _, n := range names {
		if i := strings.Index(string(in), `"not_rendered": [`); i >= 0 && strings.Contains(string(in)[i:], "brain:project/"+n) {
			cut = "brain:project/" + n
		}
	}
	if cut == "" {
		t.Fatalf("the fixture cut no note; inputs %s", in)
	}
	got := given(historyLines(t, hist))
	for _, l := range got {
		if strings.HasSuffix(strings.Split(l, " · by ")[0], cut) || strings.Contains(l, "my-habits") || strings.Contains(l, "research.md") || strings.Contains(l, "intent.md") {
			t.Errorf("a note that was not carried got a line: %q", l)
		}
	}
	if len(got) == 0 || len(got) > 8 {
		t.Fatalf("want between 1 and 8 brain given lines, got %d: %q", len(got), got)
	}
}

func TestRenderFailureLogsNoBrainGiven(t *testing.T) {
	e, taskDir, hist := givenFixture(t, []string{"brain:project/deploy", "brain:project/missing"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the script.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	// A link to a note that does not exist fails the render before anything is saved.
	call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if got := given(historyLines(t, hist)); len(got) != 0 {
		t.Fatalf("a failed render logged %q", got)
	}
	_ = taskDir
}

func TestRenderChangedSavedInputsLogsNoBrainGiven(t *testing.T) {
	e, taskDir, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the script.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if err := os.WriteFile(filepath.Join(taskDir, "jobs", "J-1", "inputs.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hist, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	call(t, e, out, errs, 1, "render", "T-01", "J-1", "--json")
	if got := given(historyLines(t, hist)); len(got) != 0 {
		t.Fatalf("a refused render logged %q", got)
	}
}

func TestRenderWithoutHistoryFileSucceeds(t *testing.T) {
	e, _, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the script.\n")
	})
	if err := os.Remove(hist); err != nil {
		t.Fatal(err)
	}
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
}

// Rendering takes .task.lock only after save returns; other writers using
// AppendHistory at the same time each land once and nothing deadlocks.
func TestRenderBrainGivenAmongConcurrentHistoryAppends(t *testing.T) {
	e, taskDir, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the script.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	const writers, each = 4, 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < each; i++ {
				if err := task.AppendHistory(taskDir, "note", fmt.Sprintf("w%d-i%d", w, i), "tester", ""); err != nil {
					t.Errorf("append: %v", err)
				}
			}
		}(w)
	}
	close(start)
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	wg.Wait()
	lines := historyLines(t, hist)
	if len(given(lines)) != 2 {
		t.Fatalf("want one brain given line for deploy and one for the recalled export-format, got %q", given(lines))
	}
	for w := 0; w < writers; w++ {
		for i := 0; i < each; i++ {
			needle := fmt.Sprintf(" · note · w%d-i%d · ", w, i)
			n := 0
			for _, l := range lines {
				if strings.Contains(l, needle) {
					n++
				}
			}
			if n != 1 {
				t.Errorf("line %s landed %d times", needle, n)
			}
		}
	}
}
