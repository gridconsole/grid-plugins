package contract

import (
	"reflect"
	"slices"
	"testing"
)

// plBd2Loop is a loop post of J-1 from its fixture, of this attempt, with id and
// seq n.
func plBd2Loop(t *testing.T, typ string, n, attempt int) Post {
	t.Helper()
	p := side(t, typ, n)
	p.Job, p.Attempt = pointer("J-1"), pointer(attempt)
	return p
}

// plBd2Exit is the launcher's exit of J-1's attempt with this reason.
func plBd2Exit(t *testing.T, n, attempt int, reason string) Post {
	t.Helper()
	p := plBd2Loop(t, "exit", n, attempt)
	changeBody(t, &p, func(v map[string]any) { v["reason"] = reason })
	return p
}

// plBd2Orders is the posts as given, reversed and shuffled.
func plBd2Orders(posts []Post) [][]Post {
	reversed := slices.Clone(posts)
	slices.Reverse(reversed)
	return [][]Post{posts, reversed, plBdShuffle(posts)}
}

// plBd2Counted fails unless every post folds counted.
func plBd2Counted(t *testing.T, state State, posts ...Post) {
	t.Helper()
	for _, p := range posts {
		if v := verdict(state, p.ID); v.Kind != "counted" {
			t.Fatalf("%s %s folds %s %q, want counted", p.Type, p.ID, v.Kind, v.Reason)
		}
	}
}

// Row 2 (6, V26): an attempt that posted blocked and then exited folds Blocked,
// not Submitted, whichever order the writers sort in.
func TestPlBd2BlockedThenExitFoldsBlocked(t *testing.T) {
	const rule = "blocked on Bash(make deploy-prod)"
	for _, tc := range []struct {
		name   string
		posts  func(*testing.T) []Post
		status Status
		reason string
	}{
		{"started, result, blocked, exit done", func(t *testing.T) []Post {
			return append(base(t), plBd2Loop(t, "started", 4, 1), side(t, "result", 5), plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "done"))
		}, Blocked, rule},
		{"the same with no blocked", func(t *testing.T) []Post {
			return append(base(t), plBd2Loop(t, "started", 4, 1), side(t, "result", 5), plBd2Exit(t, 7, 1, "done"))
		}, Submitted, "awaiting judgement"},
		{"blocked and a failed exit, no started and no result", func(t *testing.T) []Post {
			return append(base(t), plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "failed"))
		}, Blocked, rule},
		{"blocked and an exit over a worker-exit done with a result", func(t *testing.T) []Post {
			x := side(t, "worker-exit", 5)
			body(t, &x, map[string]any{"reason": "done"})
			return append(base(t), side(t, "result", 4), x, plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "done"))
		}, Blocked, rule},
		{"blocked writer sorting before the launcher", func(t *testing.T) []Post {
			b := plBd2Loop(t, "blocked", 6, 1)
			b.Worker = "agent@01K9J000000000000000000002"
			return append(base(t), side(t, "result", 5), b, plBd2Exit(t, 7, 1, "done"))
		}, Blocked, rule},
		{"blocked with no exit yet", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 5), plBd2Loop(t, "blocked", 6, 1))
		}, Running, ""},
		{"an accept after the blocked exit stands", func(t *testing.T) []Post {
			return append(base(t), plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "done"), move(t, 8, Accepted))
		}, Accepted, "The task holder made this decision"},
		{"started moves an assigned Job to running", func(t *testing.T) []Post {
			return append(base(t)[:2], plBd2Loop(t, "started", 4, 1))
		}, Running, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := tc.posts(t)
			for _, in := range plBd2Orders(posts) {
				state := Fold(in)
				for _, p := range posts {
					if isAttemptSide(p.Type) {
						plBd2Counted(t, state, p)
					}
				}
				if j := state.Jobs["J-1"]; j == nil || j.Status != tc.status || j.Reason != tc.reason {
					t.Fatalf("Job = %+v, want %s %q", j, tc.status, tc.reason)
				}
			}
		})
	}
}

// Row 2: a blocked of attempt 1 does not block attempt 2, and the assign of
// attempt 2 after a Blocked Job is granted.
func TestPlBd2BlockedIsPerAttempt(t *testing.T) {
	old := []Post{plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "failed")}
	blocked := Fold(append(base(t), old...)).Jobs["J-1"]
	if blocked.Status != Blocked {
		t.Fatalf("attempt 1 = %+v, want blocked", blocked)
	}
	a, k := retry(t, 20, 2, 3)
	posts := append(append(base(t), old...), a, k, plBdSide(t, "result", 30), plBd2Exit(t, 31, 2, "done"))
	for _, in := range plBd2Orders(posts) {
		state := Fold(in)
		if v := verdict(state, a.ID); v.Kind != "counted" {
			t.Fatalf("assign after Blocked folds %+v", v)
		}
		for _, p := range old {
			if v := verdict(state, p.ID); v.Kind != "stale" || v.Reason != "older attempt" {
				t.Errorf("%s of attempt 1 folds %+v, want stale older attempt", p.Type, v)
			}
		}
		if j := state.Jobs["J-1"]; j.Attempt != 2 || j.Status != Submitted || j.Reason != "awaiting judgement" {
			t.Fatalf("attempt 2 = %+v, want submitted", j)
		}
	}
}

// The attempt rule and what each loop post leaves alone.
func TestPlBd2LoopPostAttemptRule(t *testing.T) {
	t.Run("waiting, cancel and wake count and move nothing", func(t *testing.T) {
		want := Fold(base(t)).Jobs["J-1"]
		w, c, k := plBd2Loop(t, "waiting", 4, 1), plBd2Loop(t, "cancel", 5, 1), side(t, "wake", 6)
		state := Fold(append(base(t), w, c, k))
		plBd2Counted(t, state, w, c, k)
		got := state.Jobs["J-1"]
		if got.Status != want.Status || got.Reason != want.Reason || got.Exited != "" || !reflect.DeepEqual(got.History, append(slices.Clone(want.History), w.ID, c.ID)) {
			t.Fatalf("Job = %+v, want %+v with the two posts in history", got, want)
		}
	})
	t.Run("exit sets the exit fields once", func(t *testing.T) {
		x, y := plBd2Exit(t, 4, 1, "crash"), plBd2Exit(t, 5, 1, "done")
		state := Fold(append(base(t), x, y))
		j := state.Jobs["J-1"]
		if j.Exited != x.ID || j.ExitReason != "crash" || string(j.ExitCode) != "0" || j.Status != Running {
			t.Fatalf("Job = %+v, want exited %s crash code 0 running", j, x.ID)
		}
		if v := verdict(state, y.ID); v.Kind != "ignored" || v.Reason != "attempt already exited" {
			t.Fatalf("second exit folds %+v", v)
		}
	})
	for _, tc := range []struct {
		name, kind, reason string
		mutate             func(*Post)
	}{
		{"an attempt above the Job's", "ignored", "no counted assign for this attempt", func(p *Post) { p.Attempt = pointer(2) }},
		{"a Job never assigned", "ignored", "no counted assign for this attempt", func(p *Post) { p.Job = pointer("J-1") }},
		{"an unknown Job", "ignored", "unknown Job target", func(p *Post) { p.Job = pointer("J-9") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, typ := range []string{"started", "exit", "blocked", "waiting", "cancel"} {
				p := plBd2Loop(t, typ, 9, 1)
				tc.mutate(&p)
				posts := base(t)
				if tc.name == "a Job never assigned" {
					posts = posts[:1]
				}
				state := Fold(append(posts, p))
				if v := verdict(state, p.ID); v.Kind != tc.kind || v.Reason != tc.reason {
					t.Errorf("%s folds %s %q, want %s %q", typ, v.Kind, v.Reason, tc.kind, tc.reason)
				}
			}
		})
	}
}

// Row 1 at the fold (V27): a landed post for a known Job is the Job's landing.
func TestPlBd2LandedFolds(t *testing.T) {
	l := side(t, "landed", 4)
	l.Job = pointer("J-1")
	other := side(t, "landed", 5)
	other.Job = pointer("J-9")
	state, loop := FoldLoop(append(base(t), l, other))
	plBd2Counted(t, state, l)
	if v := verdict(state, other.ID); v.Kind != "ignored" || v.Reason != "unknown Job target" {
		t.Fatalf("landed for an unknown Job folds %+v", v)
	}
	if !reflect.DeepEqual(loop.Landed, map[string]string{"J-1": l.ID}) {
		t.Fatalf("Landed = %v, want J-1: %s", loop.Landed, l.ID)
	}
	if _, loop := FoldLoop(base(t)); len(loop.Landed) != 0 {
		t.Fatalf("an accept-free Feed with no landed post landed %v", loop.Landed)
	}
}

// Row 3 (7, V26, V22): the allow of an assign counts under a person's approval
// of those rules for that Job. An agent's approval with no grant, an approval
// naming another rule or Job, a decline, and a grant naming anything but a
// counted approval fold it absent. The assign is granted in every case.
func TestPlBd2AllowNeedsPersonApproval(t *testing.T) {
	const rule = "Bash(make deploy-prod)"
	for _, tc := range []struct {
		name     string
		approval func(*Post)
		allow    []string
		grant    func(blocked, approval Post) string
		want     bool
	}{
		{"owner approves", func(*Post) {}, []string{rule}, nil, true},
		{"member approves", func(p *Post) { p.Author = Author{"member", "member"}; p.Worker = "member@01K9J000000000000000000002" }, []string{rule}, nil, true},
		{"approval lists more rules than the allow", func(p *Post) {
			p.Body = []byte(`{"rules":["Bash(make deploy-prod)","Bash(make release)"],"decision":"approve"}`)
		}, []string{rule}, nil, true},
		{"agent-authored approval", func(p *Post) {
			p.Author, p.Tier, p.Worker = Author{"agent", "agent"}, Agent, "agent@01K9J000000000000000000002"
		}, []string{rule}, nil, false},
		{"cli-authored approval", func(p *Post) {
			p.Author, p.Tier, p.Worker = Author{"cli", "cli"}, Agent, "cli@01K9J000000000000000000002"
		}, []string{rule}, nil, false},
		{"approval of another rule", func(*Post) {}, []string{"Bash(make release)"}, nil, false},
		{"allow with a rule the approval lacks", func(*Post) {}, []string{rule, "Bash(make release)"}, nil, false},
		{"approval naming another Job", func(p *Post) { p.Job = pointer("J-2") }, []string{rule}, nil, false},
		{"decline", func(p *Post) {
			p.Body = []byte(`{"rules":["Bash(make deploy-prod)"],"decision":"decline","why":"Not on this run"}`)
		}, []string{rule}, nil, false},
		{"grant naming the blocked post", func(*Post) {}, []string{rule}, func(b, _ Post) string { return b.ID }, false},
		{"grant naming a held approval", func(p *Post) { p.Re = pointer("00000000000000000000000099") }, []string{rule}, nil, false},
		{"no grant", func(*Post) {}, []string{rule}, func(Post, Post) string { return "" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := plBd2Loop(t, "blocked", 6, 1)
			x := plBd2Exit(t, 7, 1, "done")
			ap := plBd2Loop(t, "approval", 8, 1)
			ap.Re = &b.ID
			tc.approval(&ap)
			grant := ap.ID
			if tc.grant != nil {
				grant = tc.grant(b, ap)
			}
			a, k := retry(t, 20, 2, 3)
			changeBody(t, &a, func(v map[string]any) {
				assign := v["assign"].(map[string]any)
				assign["allow"] = tc.allow
				if grant != "" {
					assign["grant"] = grant
				}
			})
			posts := append(base(t), b, x, ap, a, k)
			for _, in := range plBd2Orders(posts) {
				state, loop := FoldLoop(in)
				plBd2Counted(t, state, a, k)
				if tc.name != "grant naming a held approval" {
					plBd2Counted(t, state, ap)
				} else if v := verdict(state, ap.ID); v.Kind != "held" {
					t.Fatalf("approval re a missing post folds %+v, want held", v)
				}
				if j := state.Jobs["J-1"]; j.Attempt != 2 || j.Status != Running {
					t.Fatalf("assign after Blocked: Job = %+v, want attempt 2 running", j)
				}
				if !reflect.DeepEqual(state, Fold(in)) {
					t.Fatal("FoldLoop's State differs from Fold's")
				}
				got, ok := loop.Allow["J-1"]
				if ok != tc.want || ok && !reflect.DeepEqual(got, tc.allow) {
					t.Fatalf("Allow = %v (present %v), want present %v", loop.Allow, ok, tc.want)
				}
			}
		})
	}
}
