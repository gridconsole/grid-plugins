package job_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// rcxResolver answers from a map of pointer to source; an unknown pointer is an error.
type rcxResolver map[string]job.Source

func (r rcxResolver) Resolve(p string) (job.Source, error) {
	s, ok := r[p]
	if !ok {
		return job.Source{}, fmt.Errorf("%s: unknown to the test resolver", p)
	}
	return s, nil
}

// rcxNote is a team note of 100 words, so a budget decides how many fit.
func rcxNote(summary, topic string, links ...job.Link) job.Source {
	return job.Source{Summary: summary, Text: strings.Repeat("w ", 100), Tier: contract.PersonTeam, Topic: topic, Links: links}
}

// rcxJob has its three intent sources resolved by the resolver rcxWith builds.
func rcxJob() job.Job {
	return job.Job{
		ID: "J-1", Title: "Context fixture",
		Intent:    job.Intent{Owner: "owner.md", Item: "item.md", Goal: "goal.md"},
		Verify:    job.Verify{Kind: "tests", Path: "x"},
		OnBlocked: "stop",
	}
}

func rcxWith(extra map[string]job.Source) rcxResolver {
	r := rcxResolver{
		"owner.md": rcxNote("owner", ""),
		"item.md":  rcxNote("item", ""),
		"goal.md":  rcxNote("goal", ""),
	}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func rcxRefused(t *testing.T, j job.Job, r job.Resolver, want string) {
	t.Helper()
	_, e := job.Render(j, job.RenderOptions{Resolver: r})
	if e == nil || e.Error() != want {
		t.Fatalf("got %v, want %q", e, want)
	}
}

// An explicit brain link to a note with no summary is refused (render.go, the
// add call in the explicit-links loop).
func TestRenderRefusesExplicitBrainLinkWithoutSummary(t *testing.T) {
	j := rcxJob()
	j.Links = []job.Link{{To: "brain:project/a", Rel: "uses"}}
	rcxRefused(t, j, rcxWith(map[string]job.Source{"brain:project/a": rcxNote("", "")}),
		"brain:project/a: summary must have 1 to 300 characters")
}

// A depends-on link, followed one hop, to a note with no summary is refused too.
func TestRenderRefusesOneHopLinkWithoutSummary(t *testing.T) {
	j := rcxJob()
	j.Links = []job.Link{{To: "brain:project/a", Rel: "uses", Why: "w"}}
	r := rcxWith(map[string]job.Source{
		"brain:project/a": rcxNote("a", "", job.Link{To: "brain:project/z", Rel: "depends-on"}),
		"brain:project/z": rcxNote("", ""),
	})
	rcxRefused(t, j, r, "brain:project/z: summary must have 1 to 300 characters")
}

// Two notes with no topic, cut by a small budget: the banner names `all`. Each
// budget below succeeded on the probe; the cut ids are what NotRendered lists.
// At 40 to 80 both notes are cut by the first pass and at 100 only b is. Budget
// 150 is the one that enters the second cut pass (the loop that moves kept[last]
// to cut): both blocks fit in the first pass, the banner then does not fit
// beside them, so b is dropped. The per-budget coverprofile shows the second
// pass at count 0 for 40, 60, 80 and 100 and count 1 for 150.
func TestRenderCutBannerNamesAllWhenNoCutNoteHasATopic(t *testing.T) {
	j := rcxJob()
	j.Links = []job.Link{
		{To: "brain:project/a", Rel: "uses", Why: "w"},
		{To: "brain:project/b", Rel: "uses", Why: "w"},
	}
	r := rcxWith(map[string]job.Source{
		"brain:project/a": rcxNote("note a", ""),
		"brain:project/b": rcxNote("note b", ""),
	})
	both := []string{"brain:project/a", "brain:project/b"}
	for _, c := range []struct {
		budget int
		cut    []string
	}{
		{40, both}, {60, both}, {80, both},
		{100, both[1:]}, {150, both[1:]},
	} {
		t.Run(fmt.Sprintf("budget %d", c.budget), func(t *testing.T) {
			got, e := job.Render(j, job.RenderOptions{Resolver: r, BrainTokens: c.budget})
			if e != nil {
				t.Fatal(e)
			}
			for _, want := range []string{"entries cut; ", "grid brain topic all"} {
				if !strings.Contains(got.Brief, want) {
					t.Errorf("brief lacks %q", want)
				}
			}
			if !reflect.DeepEqual(got.Inputs.NotRendered, c.cut) {
				t.Errorf("NotRendered = %v, want %v", got.Inputs.NotRendered, c.cut)
			}
		})
	}
}

// A pointer that already has an anchor is kept as written; one without gets the
// #L anchor.
func TestSourceKeyKeepsAnchoredRange(t *testing.T) {
	for in, want := range map[string]string{
		"a.go:1-5#x": "a.go:1-5#x",
		"a.go:1-5":   "a.go#L1-L5",
	} {
		if got := job.SourceKey(in); got != want {
			t.Errorf("SourceKey(%q) = %q, want %q", in, got, want)
		}
	}
}
