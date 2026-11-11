package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

const hdGDeployLine = " · brain given · brain:project/deploy · by grid job render · via J-1"

// hdGKeepFixture is the job fixture with no brain link at all: the plan keeps
// brain:project/export-format through keep: only, the plan is approved again, and
// the task holds an empty history.md.
func hdGKeepFixture(t *testing.T) (Env, string) {
	t.Helper()
	h := jobtest.New(t, filepath.Join("..", "..", "..", "testdata"))
	planPath := filepath.Join(h.Task, "plan.md")
	b, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	withLink := "      links:\n" + exportLinkLine
	text := string(b)
	if !strings.Contains(text, withLink) || !strings.Contains(text, "keep: [brain:project/export-format]") {
		t.Fatal("fixture plan lost its brain link or its keep")
	}
	text = strings.Replace(text, withLink, "      links: []\n", 1)
	if err := os.WriteFile(planPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	hist := filepath.Join(h.Task, "history.md")
	if err := os.WriteFile(hist, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs := newBuffers()
	return Env{Resolver: ownerResolver(h.GridHome), Cwd: filepath.Dir(h.Task), GridHome: h.GridHome, Machine: jobtest.Machine, Stdout: out, Stderr: errs}, hist
}

// A linked note carries the link's why on its brain given line, exactly once.
func TestHdGGivenWhy(t *testing.T) {
	e, _, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the deploy script from the release branch.\n")
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	got := given(historyLines(t, hist))
	n := 0
	for _, l := range got {
		if strings.HasSuffix(l, hdGDeployLine+" · why Carry this note into the Job") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one deploy line ending with the link's why, got %d in %q", n, got)
	}
	// The note the render recalled by the Job's title has no link: no why.
	for _, l := range got {
		if strings.Contains(l, "brain:project/export-format") && strings.Contains(l, " · why ") {
			t.Errorf("a note with no link got a why: %q", l)
		}
	}
	// A second render with unchanged inputs adds nothing, with the why in the line.
	call(t, e, out, errs, 0, "render", "T-01", "J-1", "--json")
	if again := given(historyLines(t, hist)); len(again) != len(got) {
		t.Fatalf("second render changed the brain given lines: %q", again)
	}
}

// A note the Job carries through keep: with no link has no why.
func TestHdGGivenKeepOnlyHasNoWhy(t *testing.T) {
	e, hist := hdGKeepFixture(t)
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	got := given(historyLines(t, hist))
	want := " · brain given · brain:project/export-format · by grid job render · via J-1"
	if len(got) != 1 || !strings.HasSuffix(got[0], want) {
		t.Fatalf("want one line ending %q, got %q", want, got)
	}
	if strings.Contains(got[0], " · why ") {
		t.Fatalf("a keep: note with no link got a why: %q", got[0])
	}
}

// The why is the why of the link whose target is the note, an anchor cut; the
// first link with a why wins, and a note no link names has none.
func TestHdGGivenWhyPicksTheLinkOfTheNote(t *testing.T) {
	j := job.Job{Links: []job.Link{
		{To: "brain:project/other", Rel: "uses", Why: "not this one"},
		{To: "brain:project/deploy#steps", Rel: "uses", Why: "Follow the steps"},
		{To: "brain:project/deploy", Rel: "uses", Why: "later link"},
		{To: "brain:project/blank", Rel: "uses", Why: "  "},
		{To: "brain:project/blank#a", Rel: "uses", Why: "named by the anchor link"},
	}}
	for id, want := range map[string]string{
		"brain:project/deploy":  "Follow the steps",
		"brain:project/other":   "not this one",
		"brain:project/blank":   "named by the anchor link",
		"brain:project/missing": "",
	} {
		if got := givenWhy(j, id); got != want {
			t.Errorf("givenWhy(%s) = %q, want %q", id, got, want)
		}
	}
	if got := givenWhy(job.Job{Keep: []string{"brain:project/deploy"}}, "brain:project/deploy"); got != "" {
		t.Errorf("a keep: note with no link has the why %q", got)
	}
}
