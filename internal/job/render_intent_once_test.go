package job_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

func renderIntent(t *testing.T, owner, item, goal string, read ...string) job.Rendered {
	t.Helper()
	j, f, _ := sample(t)
	j.Intent = job.Intent{Owner: owner, Item: item, Goal: goal}
	j.Read = read
	got, e := job.Render(j, job.RenderOptions{Resolver: f, Generation: "fixture-1"})
	if e != nil {
		t.Fatal(e)
	}
	return got
}

func sourcesFor(r job.Rendered, file string) []job.Source {
	out := []job.Source{}
	for _, s := range r.Inputs.Sources {
		if strings.Contains(s.Pointer, file) {
			out = append(out, s)
		}
	}
	return out
}

func TestRequestShownOnceWhenAllIntentSlotsAgree(t *testing.T) {
	got := renderIntent(t, "intent.md#request", "intent.md#request", "intent.md#request", "task:intent.md#request")
	req := "## Request {#request}"
	if n := strings.Count(got.Brief, req); n != 1 {
		t.Fatalf("request text appears %d times, want 1", n)
	}
	if !strings.Contains(got.Brief, "### Owner, Item, Goal [person-team] (intent.md#request)\n") {
		t.Fatalf("no joined Intent header in:\n%s", got.Brief)
	}
	for _, old := range []string{"### Owner [", "### Item [", "### Goal ["} {
		if strings.Contains(got.Brief, old) {
			t.Fatalf("brief still has %q", old)
		}
	}
	// the Context list still names the duplicate read pointer
	if !strings.Contains(got.Brief, "- Read task:intent.md#request [person-team]\n") {
		t.Fatal("the Read line for the second spelling is gone")
	}
}

func TestRequestIsOneSourceInInputs(t *testing.T) {
	got := renderIntent(t, "intent.md#request", "intent.md#request", "intent.md#request", "task:intent.md#request")
	srcs := sourcesFor(got, "intent.md")
	if len(srcs) != 1 {
		t.Fatalf("%d intent sources in inputs.json, want 1: %+v", len(srcs), srcs)
	}
	if srcs[0].Pointer != "intent.md#request" {
		t.Fatalf("kept pointer %q, want the first spelling met", srcs[0].Pointer)
	}
}

func TestSpellingsOfOneRangeAreOneSource(t *testing.T) {
	// :N-M and #LN-LM name the same lines; a different range is another source.
	got := renderIntent(t, "intent.md#request", "intent.md#request", "intent.md#request", "task:intent.md:1-2", "intent.md#L1-L2", "intent.md:1-3")
	srcs := sourcesFor(got, "intent.md")
	if len(srcs) != 3 {
		t.Fatalf("%d sources, want 3 (anchor, 1-2, 1-3): %+v", len(srcs), srcs)
	}
	var one int
	for _, s := range srcs {
		if s.Pointer == "task:intent.md:1-2" {
			one++
		}
		if s.Pointer == "intent.md#L1-L2" {
			t.Fatal("the second spelling replaced the first")
		}
	}
	if one != 1 {
		t.Fatal("the first spelling of the 1-2 range was not kept")
	}
}

func TestDifferentAnchorsStayDifferent(t *testing.T) {
	got := renderIntent(t, "intent.md#request", "intent.md:1-2", "intent.md:1-3")
	for _, label := range []string{"### Owner [", "### Item [", "### Goal ["} {
		if !strings.Contains(got.Brief, label) {
			t.Fatalf("brief lacks %q", label)
		}
	}
	if n := len(sourcesFor(got, "intent.md")); n != 3 {
		t.Fatalf("%d sources, want 3", n)
	}
}

func TestPartlyEqualIntentSlotsJoinOnlyThoseSlots(t *testing.T) {
	got := renderIntent(t, "intent.md#request", "intent.md:1-2", "task:intent.md#request")
	if !strings.Contains(got.Brief, "### Owner, Goal [person-team] (intent.md#request)\n") {
		t.Fatalf("no Owner, Goal header in:\n%s", got.Brief)
	}
	if !strings.Contains(got.Brief, "### Item [person-team] (intent.md:1-2)\n") {
		t.Fatal("Item with its own range lost its header")
	}
}

func TestNonTaskPointersKeepTheirText(t *testing.T) {
	// brain:/repo:/prototype:/job: are keyed by text: no task: strip, no range rewrite.
	if got := job.SourceKey("task:intent.md:1-2"); got != "intent.md#L1-L2" {
		t.Fatalf("task range key %q", got)
	}
	if got := job.SourceKey("intent.md#request"); got != "intent.md#request" {
		t.Fatalf("anchor key %q", got)
	}
	for _, p := range []string{"brain:home/x", "repo:a.go:1-2", "prototype:p.html#x", "job:J-1#acceptance"} {
		if got := job.SourceKey(p); got != p {
			t.Fatalf("%s keyed as %q", p, got)
		}
	}
}

func TestGenerationIsStableAcrossRenders(t *testing.T) {
	a := renderIntent(t, "intent.md#request", "intent.md#request", "intent.md#request", "task:intent.md#request")
	b := renderIntent(t, "intent.md#request", "intent.md#request", "intent.md#request", "task:intent.md#request")
	if a.Inputs.Generation != b.Inputs.Generation || !reflect.DeepEqual(a, b) {
		t.Fatal("two renders of one Job differ")
	}
}

func TestReplayResolvesEverySpelling(t *testing.T) {
	j, f, _ := sample(t)
	j.Intent = job.Intent{Owner: "intent.md#request", Item: "intent.md#request", Goal: "intent.md#request"}
	j.Read = []string{"task:intent.md#request"}
	got, e := job.Render(j, job.RenderOptions{Resolver: f, Generation: "fixture-1"})
	if e != nil {
		t.Fatal(e)
	}
	replay, e := job.Render(j, job.RenderOptions{Resolver: got.Inputs.Resolver(), Generation: got.Inputs.Generation})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, replay) {
		t.Fatal("replay of a deduplicated render differs")
	}
}
