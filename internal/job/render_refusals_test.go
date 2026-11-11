package job_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// rrResolver answers from a map and fails for the pointers in fail.
type rrResolver struct {
	src  map[string]job.Source
	fail map[string]error
}

func (r rrResolver) Resolve(p string) (job.Source, error) {
	if e, ok := r.fail[p]; ok {
		return job.Source{}, e
	}
	s, ok := r.src[p]
	if !ok {
		return job.Source{}, fmt.Errorf("%s: unknown to the test resolver", p)
	}
	return s, nil
}

func rrNote(summary string, links ...job.Link) job.Source {
	return job.Source{Summary: summary, Text: "text of " + summary, Tier: contract.PersonTeam, Topic: "topic", Links: links}
}

func rrResolverWith(extra map[string]job.Source) rrResolver {
	src := map[string]job.Source{
		"owner.md": rrNote("owner"),
		"item.md":  rrNote("item"),
		"goal.md":  rrNote("goal"),
	}
	for k, v := range extra {
		src[k] = v
	}
	return rrResolver{src: src, fail: map[string]error{}}
}

func rrJob() job.Job {
	return job.Job{
		ID: "J-1", Title: "Refusal fixture",
		Intent:    job.Intent{Owner: "owner.md", Item: "item.md", Goal: "goal.md"},
		Verify:    job.Verify{Kind: "tests", Path: "internal/x"},
		OnBlocked: "stop",
	}
}

func rrRender(t *testing.T, j job.Job, r job.Resolver, recall ...job.Link) job.Rendered {
	t.Helper()
	got, e := job.Render(j, job.RenderOptions{Resolver: r, Recall: recall})
	if e != nil {
		t.Fatal(e)
	}
	return got
}

func rrRefused(t *testing.T, j job.Job, o job.RenderOptions, want string) {
	t.Helper()
	_, e := job.Render(j, o)
	if e == nil || e.Error() != want {
		t.Fatalf("got %v, want %q", e, want)
	}
}

func rrResolved(r job.Rendered, pointer string) bool {
	for _, s := range r.Inputs.Sources {
		if s.Pointer == pointer {
			return true
		}
	}
	return false
}

func TestRenderRefusesNilResolverAndBadBudget(t *testing.T) {
	j := rrJob()
	r := rrResolverWith(nil)
	t.Run("nil resolver", func(t *testing.T) {
		rrRefused(t, j, job.RenderOptions{}, "a source resolver is required")
	})
	for _, n := range []int{-1, 2001} {
		t.Run(fmt.Sprintf("budget %d", n), func(t *testing.T) {
			rrRefused(t, j, job.RenderOptions{Resolver: r, BrainTokens: n}, "brain budget must be between 1 and 2000 tokens")
		})
	}
	for _, n := range []int{1, 2000} {
		t.Run(fmt.Sprintf("budget %d accepted", n), func(t *testing.T) {
			if _, e := job.Render(j, job.RenderOptions{Resolver: r, BrainTokens: n}); e != nil {
				t.Fatal(e)
			}
		})
	}
	t.Run("budget 0 renders as 2000", func(t *testing.T) {
		// One note of about 1,100 tokens fits the default 2,000 and does not fit 1,000.
		big := rrNote("big note")
		big.Text = strings.Repeat("x", 4400)
		j := rrJob()
		j.Decisions = []string{"brain:project/big"}
		res := rrResolverWith(map[string]job.Source{"brain:project/big": big})
		zero := rrRender(t, j, res)
		if zero.Inputs.BrainTokens <= 1000 || zero.Inputs.BrainTokens > 2000 || len(zero.Inputs.NotRendered) != 0 {
			t.Fatalf("budget 0 did not render as 2000: %+v", zero.Inputs)
		}
		// Under 1,000 the note is cut and the banner is the whole brain part.
		got, e := job.Render(j, job.RenderOptions{Resolver: res, BrainTokens: 1000})
		if e != nil || len(got.Inputs.NotRendered) != 1 || got.Inputs.BrainTokens > 1000 {
			t.Fatalf("a budget of 1000 kept the note: %v %v %d", e, got.Inputs.NotRendered, got.Inputs.BrainTokens)
		}
		full := rrRender(t, j, res)
		two, e := job.Render(j, job.RenderOptions{Resolver: res, BrainTokens: 2000})
		if e != nil || two.Brief != full.Brief {
			t.Fatalf("budget 0 and 2000 differ: %v", e)
		}
	})
	t.Run("budget below the elision banner", func(t *testing.T) {
		j := rrJob()
		j.Decisions = []string{"brain:project/big"}
		big := rrNote("big note")
		big.Text = strings.Repeat("x", 4400)
		res := rrResolverWith(map[string]job.Source{"brain:project/big": big})
		rrRefused(t, j, job.RenderOptions{Resolver: res, BrainTokens: 1}, "brain budget too small for the elision banner")
	})
}

func TestRenderRefusesBadBrainSummary(t *testing.T) {
	render := func(extra job.Source) error {
		j := rrJob()
		j.Decisions = []string{"brain:project/s"}
		_, e := job.Render(j, job.RenderOptions{Resolver: rrResolverWith(map[string]job.Source{"brain:project/s": extra})})
		return e
	}
	const want = "brain:project/s: summary must have 1 to 300 characters"
	for name, summary := range map[string]string{
		"empty":          "",
		"blank":          "  \t ",
		"301 characters": strings.Repeat("a", 301),
		"301 multi-byte": strings.Repeat("é", 301),
	} {
		t.Run(name, func(t *testing.T) {
			e := render(rrNote(summary))
			if e == nil || e.Error() != want {
				t.Fatalf("got %v, want %q", e, want)
			}
		})
	}
	t.Run("300 characters accepted", func(t *testing.T) {
		if e := render(rrNote(strings.Repeat("a", 300))); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("300 multi-byte accepted", func(t *testing.T) {
		if e := render(rrNote(strings.Repeat("é", 300))); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("pointer-only source needs no summary", func(t *testing.T) {
		s := rrNote("")
		s.PointerOnly = true
		if e := render(s); e != nil {
			t.Fatal(e)
		}
	})
	t.Run("recalled entry is checked too", func(t *testing.T) {
		res := rrResolverWith(map[string]job.Source{"brain:project/r": rrNote("")})
		rrRefused(t, rrJob(), job.RenderOptions{Resolver: res, Recall: []job.Link{{To: "brain:project/r", Rel: "uses", Why: "Recalled"}}}, "brain:project/r: summary must have 1 to 300 characters")
	})
}

func TestRenderPassesResolverErrorsThrough(t *testing.T) {
	boom := errors.New("resolver boom")
	cont := "J-0"
	cases := []struct {
		name string
		key  string
		edit func(*job.Job)
	}{
		{"intent owner", "owner.md", func(*job.Job) {}},
		{"intent item", "item.md", func(*job.Job) {}},
		{"intent goal", "goal.md", func(*job.Job) {}},
		{"relation", "repo:rel.go", func(j *job.Job) {
			j.Links = []job.Link{{To: "repo:rel.go", Rel: "uses", Why: "Related file"}}
		}},
		{"continues", "job:J-0#acceptance", func(j *job.Job) { j.Continues = &cont }},
		{"read", "repo:read.go", func(j *job.Job) { j.Read = []string{"repo:read.go"} }},
		{"decision", "brain:project/d", func(j *job.Job) { j.Decisions = []string{"brain:project/d"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			j := rrJob()
			c.edit(&j)
			res := rrResolverWith(nil)
			res.fail[c.key] = boom
			_, e := job.Render(j, job.RenderOptions{Resolver: res})
			if !errors.Is(e, boom) {
				t.Fatalf("got %v, want the resolver error", e)
			}
		})
	}
	t.Run("recalled entry", func(t *testing.T) {
		res := rrResolverWith(nil)
		res.fail["brain:project/r"] = boom
		_, e := job.Render(rrJob(), job.RenderOptions{Resolver: res, Recall: []job.Link{{To: "brain:project/r", Rel: "uses", Why: "Recalled"}}})
		if !errors.Is(e, boom) {
			t.Fatalf("got %v, want the resolver error", e)
		}
	})
}

// The one-hop walk follows depends-on and decided-by links to a brain: target
// from an explicit candidate (render.go:181) and from nothing else.
func TestRenderWalksExactlyOneBrainHop(t *testing.T) {
	notes := func() map[string]job.Source {
		return map[string]job.Source{
			"brain:project/a": rrNote("note a",
				job.Link{To: "brain:project/b", Rel: "depends-on", Why: "A needs B"},
				job.Link{To: "brain:project/u", Rel: "uses", Why: "A uses U"},
				job.Link{To: "repo:file.go", Rel: "depends-on", Why: "A needs a file"},
				job.Link{To: "brain:project/d", Rel: "decided-by", Why: "A was decided by D"}),
			"brain:project/b": rrNote("note b", job.Link{To: "brain:project/c", Rel: "decided-by", Why: "B was decided by C"}),
			"brain:project/c": rrNote("note c"),
			"brain:project/d": rrNote("note d"),
			"brain:project/u": rrNote("note u"),
			"repo:file.go":    rrNote("a file"),
		}
	}
	check := func(t *testing.T, got job.Rendered) {
		t.Helper()
		for _, p := range []string{"brain:project/a", "brain:project/b", "brain:project/d"} {
			if !strings.Contains(got.Brief, "- "+p+" · ") || !rrResolved(got, p) {
				t.Errorf("%s is not in the rendered brain part", p)
			}
		}
		for _, p := range []string{"brain:project/c", "brain:project/u"} {
			if strings.Contains(got.Brief, p) || rrResolved(got, p) {
				t.Errorf("%s was pulled in", p)
			}
		}
		if rrResolved(got, "repo:file.go") {
			t.Error("a target that does not start with brain: was pulled in")
		}
	}
	t.Run("from Decisions", func(t *testing.T) {
		j := rrJob()
		j.Decisions = []string{"brain:project/a"}
		check(t, rrRender(t, j, rrResolverWith(notes())))
	})
	t.Run("from Keep", func(t *testing.T) {
		j := rrJob()
		j.Keep = []string{"brain:project/a"}
		check(t, rrRender(t, j, rrResolverWith(notes())))
	})
	t.Run("from a brain link", func(t *testing.T) {
		j := rrJob()
		j.Links = []job.Link{{To: "brain:project/a", Rel: "uses", Why: "Explains the plan"}}
		check(t, rrRender(t, j, rrResolverWith(notes())))
	})
	t.Run("a recalled entry starts no hop", func(t *testing.T) {
		res := rrResolverWith(notes())
		res.src["brain:project/r"] = rrNote("note r", job.Link{To: "brain:project/b", Rel: "depends-on", Why: "R needs B"})
		got := rrRender(t, rrJob(), res, job.Link{To: "brain:project/r", Rel: "uses", Why: "Recalled"})
		if !rrResolved(got, "brain:project/r") || !strings.Contains(got.Brief, "- brain:project/r · ") {
			t.Fatal("the recalled entry is missing")
		}
		if rrResolved(got, "brain:project/b") || strings.Contains(got.Brief, "brain:project/b") {
			t.Fatal("a link of a recalled entry was followed")
		}
	})
}

func TestRenderJobFieldLines(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		j := rrJob()
		j.Checkpoint = &job.Checkpoint{Step: "step-2", Commit: "abc123"}
		j.Acceptance = []job.Acceptance{{ID: "R1", Text: "It works", Traces: "T1", State: "done"}}
		j.Verify = job.Verify{Command: "go test ./...", MinTests: 3}
		got := rrRender(t, j, rrResolverWith(nil)).Brief
		for _, want := range []string{
			"Checkpoint: step-2 (abc123)\n\n",
			"- R1: It works (traces: T1; state: done)\n",
			"\nVerify: go test ./...\nMinimum tests: 3\n",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in\n%s", want, got)
			}
		}
	})
	t.Run("unset", func(t *testing.T) {
		j := rrJob()
		j.Acceptance = []job.Acceptance{{ID: "R1", Text: "It works", Traces: "T1"}}
		j.Verify = job.Verify{Kind: "tests", Path: "internal/x", Job: "#J-2"}
		got := rrRender(t, j, rrResolverWith(nil)).Brief
		if strings.Contains(got, "Checkpoint:") || strings.Contains(got, "; state:") {
			t.Errorf("unset fields were printed in\n%s", got)
		}
		for _, want := range []string{"- R1: It works (traces: T1)\n", "\nVerify: tests internal/x#J-2\nOn blocked: stop\n"} {
			if !strings.Contains(got, want) {
				t.Errorf("missing %q in\n%s", want, got)
			}
		}
		if strings.Contains(got, "Minimum tests:") {
			t.Error("the command form was printed for a Job without a command")
		}
	})
}

func TestReplayResolveRefusesAndAliases(t *testing.T) {
	saved := job.Replay{
		"task:note.md:2-3": {Pointer: "task:note.md:2-3", Summary: "range", Text: "two\nthree"},
		"brain:project/a":  {Pointer: "brain:project/a", Summary: "a"},
	}
	t.Run("not in the inputs", func(t *testing.T) {
		for _, key := range []string{"brain:project/zzz", "note.md#L2-L4"} {
			_, e := saved.Resolve(key)
			if e == nil || e.Error() != key+": not in render inputs" {
				t.Errorf("%s: got %v", key, e)
			}
		}
	})
	t.Run("exact key", func(t *testing.T) {
		s, e := saved.Resolve("brain:project/a")
		if e != nil || s.Summary != "a" {
			t.Fatalf("%+v %v", s, e)
		}
	})
	t.Run("alias spelling", func(t *testing.T) {
		s, e := saved.Resolve("note.md#L2-L3")
		if e != nil || s.Pointer != "task:note.md:2-3" || s.Text != "two\nthree" {
			t.Fatalf("%+v %v", s, e)
		}
	})
}
