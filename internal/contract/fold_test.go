package contract

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
)

func pointer[T any](v T) *T { return &v }

func sample(t *testing.T, typ string, id int) Post {
	t.Helper()
	var p Post
	if err := json.Unmarshal(fixture(t, typ, "valid/basic"), &p); err != nil {
		t.Fatal(err)
	}
	p.ID = fmt.Sprintf("%026d", id)
	return p
}

func body(t *testing.T, p *Post, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	p.Body = data
}

func changeBody(t *testing.T, p *Post, mutate func(map[string]any)) {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(p.Body, &v); err != nil {
		t.Fatal(err)
	}
	mutate(v)
	body(t, p, v)
}

func base(t *testing.T) []Post {
	t.Helper()
	j := sample(t, "job", 1)
	a := sample(t, "control", 2)
	k := sample(t, "ack", 3)
	k.After = []string{a.ID}
	return []Post{j, a, k}
}

func move(t *testing.T, id int, status Status) Post {
	t.Helper()
	p := sample(t, "control", id)
	p.Seq = int64(id)
	b := map[string]any{"target": "J-1", "status": status, "reason": "The task holder made this decision"}
	if status == Cancelled || status == Rejected {
		b["close_reason"] = "The requested work is no longer needed"
	}
	body(t, &p, b)
	return p
}

func retry(t *testing.T, id, attempt int, fence int64) (Post, Post) {
	t.Helper()
	a := sample(t, "control", id)
	a.Seq = int64(id)
	changeBody(t, &a, func(v map[string]any) {
		assign := v["assign"].(map[string]any)
		assign["attempt"], assign["fence"] = attempt, fence
	})
	k := sample(t, "ack", id+1)
	k.After, k.Attempt, k.Fence = []string{a.ID}, pointer(attempt), fence
	return a, k
}

func side(t *testing.T, typ string, id int) Post {
	t.Helper()
	p := sample(t, typ, id)
	p.Seq = int64(id)
	return p
}

func verdict(state State, id string) Verdict {
	for _, v := range state.Verdicts {
		if v.ID == id {
			return v
		}
	}
	return Verdict{}
}

func TestFoldScenarios(t *testing.T) {
	for _, tc := range []struct {
		name    string
		posts   func(*testing.T) []Post
		status  Status
		attempt int
		check   func(*testing.T, State)
	}{
		{"posted", func(t *testing.T) []Post { return base(t)[:1] }, Posted, 0, nil},
		{"assign", func(t *testing.T) []Post { return base(t)[:2] }, Assigned, 1, nil},
		{"ack-replaces-claim", base, Running, 1, nil},
		{"legacy-claim-does-not-start", func(t *testing.T) []Post {
			p := base(t)
			p[2].Type = "claim"
			return p
		}, Assigned, 1, func(t *testing.T, s State) {
			if verdict(s, fmt.Sprintf("%026d", 3)).Kind != "unknown" {
				t.Fatal("claim must be an unknown legacy type")
			}
		}},
		{"heartbeat-does-not-renew", func(t *testing.T) []Post {
			p := base(t)
			h := side(t, "ack", 4)
			h.Type, h.At, h.After = "heartbeat", "2099-01-01T00:00:00.000Z", []string{}
			return append(p, h)
		}, Running, 1, func(t *testing.T, s State) {
			if s.Jobs["J-1"].LeaseExpires != "2026-11-08T10:10:00.000Z" {
				t.Fatal("Feed heartbeat changed lease")
			}
		}},
		{"expired-until-is-not-stale", func(t *testing.T) []Post {
			p := base(t)
			changeBody(t, &p[1], func(v map[string]any) { v["assign"].(map[string]any)["until"] = "2000-01-01T00:00:00.000Z" })
			return p
		}, Running, 1, nil},
		{"stale-after-holder-confirms-expiry", func(t *testing.T) []Post {
			return append(base(t), move(t, 4, Stale))
		}, Stale, 1, nil},
		{"result-awaits-control", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 4))
		}, Running, 1, func(t *testing.T, s State) {
			if len(s.Jobs["J-1"].Results) != 1 {
				t.Fatal("result was not recorded")
			}
		}},
		{"accepted-result", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 4), move(t, 5, Accepted))
		}, Accepted, 1, nil},
		{"handover-announces-only", func(t *testing.T) []Post {
			h := sample(t, "handover", 4)
			h.Lease, h.Fence = pointer("task"), 5
			return append(base(t), h)
		}, Running, 1, nil},
		{"handover-preserves-earlier-controls", func(t *testing.T) []Post {
			h := sample(t, "handover", 5)
			h.Lease, h.Fence = pointer("task"), 6
			return append(base(t), move(t, 4, Stale), h)
		}, Stale, 1, nil},
		{"retry-after-stale", func(t *testing.T) []Post {
			a, k := retry(t, 5, 2, 3)
			return append(base(t), move(t, 4, Stale), a, k)
		}, Running, 2, nil},
		{"retry-after-crash", func(t *testing.T) []Post {
			a, k := retry(t, 5, 2, 3)
			return append(base(t), side(t, "worker-exit", 4), a, k)
		}, Running, 2, nil},
		{"usage-limit-reoffers-same-attempt", func(t *testing.T) []Post {
			x := side(t, "worker-exit", 4)
			body(t, &x, map[string]any{"reason": "usage_limit"})
			a, k := retry(t, 5, 1, 3)
			return append(base(t), x, a, k)
		}, Running, 1, func(t *testing.T, s State) {
			if s.Jobs["J-1"].Assign.Fence != 3 || s.Jobs["J-1"].Reason != "" {
				t.Fatal("same attempt did not get its new tenure")
			}
		}},
		{"usage-limit-hold", func(t *testing.T) []Post {
			x := side(t, "worker-exit", 4)
			body(t, &x, map[string]any{"reason": "usage_limit"})
			return append(base(t), x)
		}, Running, 1, func(t *testing.T, s State) {
			if s.Jobs["J-1"].Reason != "usage_limit" || s.Jobs["J-1"].ReviewRounds != 0 {
				t.Fatal("usage limit must hold without consuming a rework round")
			}
		}},
		{"missing-exit-reason-means-crash", func(t *testing.T) []Post {
			return append(base(t), side(t, "worker-exit", 4))
		}, Running, 1, func(t *testing.T, s State) {
			if s.Jobs["J-1"].ExitReason != "crash" {
				t.Fatal("missing reason must mean crash")
			}
		}},
		{"loop-costs-one-round", func(t *testing.T) []Post {
			x := side(t, "worker-exit", 4)
			body(t, &x, map[string]any{"reason": "loop"})
			return append(base(t), x)
		}, Running, 1, func(t *testing.T, s State) {
			if s.Jobs["J-1"].ReviewRounds != 1 || !s.Jobs["J-1"].NeedsLead {
				t.Fatal("loop must consume one round and wake the lead")
			}
		}},
		{"checkpoint-survives-retry", func(t *testing.T) []Post {
			a, k := retry(t, 6, 2, 3)
			return append(base(t), side(t, "checkpoint", 4), move(t, 5, Stale), a, k)
		}, Running, 2, func(t *testing.T, s State) {
			if s.Jobs["J-1"].Checkpoint == nil {
				t.Fatal("retry lost checkpoint")
			}
		}},
		{"duplicate-id-counts-once", func(t *testing.T) []Post {
			r := side(t, "result", 4)
			return append(base(t), r, r)
		}, Running, 1, func(t *testing.T, s State) {
			if len(s.Jobs["J-1"].Results) != 1 {
				t.Fatal("duplicate result counted twice")
			}
		}},
		{"stale-old-result-ignored", func(t *testing.T) []Post {
			a, k := retry(t, 5, 2, 3)
			r := side(t, "result", 4)
			r.Stale = true
			return append(base(t), r, a, k)
		}, Running, 2, func(t *testing.T, s State) {
			if len(s.Jobs["J-1"].Results) != 0 {
				t.Fatal("stale result counted")
			}
		}},
		{"unassigned-result-ignored", func(t *testing.T) []Post {
			r := side(t, "result", 4)
			r.Fence = 99
			return append(base(t), r)
		}, Running, 1, func(t *testing.T, s State) {
			if len(s.Jobs["J-1"].Results) != 0 {
				t.Fatal("unassigned result counted")
			}
		}},
		{"result-without-ack-ignored", func(t *testing.T) []Post {
			return append(base(t)[:2], side(t, "result", 4))
		}, Assigned, 1, nil},
		{"duplicate-assign-ignored", func(t *testing.T) []Post {
			a, k := retry(t, 5, 1, 3)
			return append(base(t), a, k)
		}, Running, 1, func(t *testing.T, s State) {
			if s.Jobs["J-1"].Assign.Fence != 2 {
				t.Fatal("duplicate grant replaced the winner")
			}
		}},
		{"unknown-result-version", func(t *testing.T) []Post {
			r := side(t, "result", 4)
			r.TypeV = 99
			return append(base(t), r)
		}, Running, 1, func(t *testing.T, s State) {
			if len(s.Jobs["J-1"].Results) != 0 {
				t.Fatal("unknown version affected state")
			}
		}},
		{"cancelled-needs-close-reason", func(t *testing.T) []Post {
			c := move(t, 4, Cancelled)
			changeBody(t, &c, func(v map[string]any) { delete(v, "close_reason") })
			return append(base(t), c)
		}, Running, 1, nil},
		{"cancelled-with-reason", func(t *testing.T) []Post { return append(base(t), move(t, 4, Cancelled)) }, Cancelled, 1, nil},
		{"rejected-with-reason", func(t *testing.T) []Post { return append(base(t), move(t, 4, Rejected)) }, Rejected, 1, nil},
		{"security-finding-raises-level", func(t *testing.T) []Post {
			r := side(t, "result", 4)
			f := sample(t, "found", 5)
			changeBody(t, &f, func(v map[string]any) { v["kind"], v["re"] = "security", r.ID })
			return append(base(t), r, f)
		}, Running, 1, func(t *testing.T, s State) {
			if j := s.Jobs["J-1"]; j.Review != "L3" || j.LevelRaised == nil || !j.NeedsLead {
				t.Fatal("security finding did not escalate")
			}
		}},
		{"proposal-does-not-create-job", func(t *testing.T) []Post {
			r := side(t, "result", 4)
			changeBody(t, &r, func(v map[string]any) {
				v["proposed_jobs"] = []ProposedJob{{Title: "Example follow-up", Role: "builder", Why: "Cover the missing edge case", Gap: "Boundary coverage", DiscoveredFrom: "J-1"}}
				v["decisions_needed"] = []string{"Choose the boundary behavior"}
			})
			return append(base(t), r)
		}, Running, 1, func(t *testing.T, s State) {
			if len(s.Jobs) != 1 || len(s.Jobs["J-1"].ProposedJobs) != 1 || !s.Jobs["J-1"].NeedsLead {
				t.Fatal("proposal or narrowing decision mishandled")
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := tc.posts(t)
			state := Fold(posts)
			j := state.Jobs["J-1"]
			if j == nil || j.Status != tc.status || j.Attempt != tc.attempt {
				t.Fatalf("Job = %+v, want status %s attempt %d; verdicts %+v", j, tc.status, tc.attempt, state.Verdicts)
			}
			if tc.check != nil {
				tc.check(t, state)
			}
			// Caller ordering is not authority. Permute inputs and display clocks;
			// per-writer seq and lease order must produce the identical fold.
			shuffled := append([]Post(nil), posts...)
			rand.New(rand.NewSource(1)).Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
			for i := range shuffled {
				shuffled[i].At = "2099-01-01T00:00:00.000Z"
			}
			other := Fold(shuffled)
			if !reflect.DeepEqual(state.Jobs, other.Jobs) {
				t.Fatalf("input interleaving or display clock changed state: %+v", other.Jobs["J-1"])
			}
		})
	}
}

func TestFoldAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		mutate     func(*Post)
	}{
		{"missing-reference", "held", func(p *Post) { p.Re = pointer(fmt.Sprintf("%026d", 99)) }},
		{"missing-after", "held", func(p *Post) { p.After = []string{fmt.Sprintf("%026d", 99)} }},
		{"stale", "stale", func(p *Post) { p.Stale = true }},
		{"unknown", "unknown", func(p *Post) { p.Type = "future-type" }},
		{"known-new-version", "unknown", func(p *Post) { p.TypeV = 99 }},
		{"bad-sequence", "bad", func(p *Post) { p.Seq = 0 }},
		{"wrong-generation", "ignored", func(p *Post) { p.Gen = 2 }},
		{"wrong-worker", "ignored", func(p *Post) { p.Worker = "other@01K9J000000000000000000002" }},
		{"wrong-lease", "bad", func(p *Post) { p.Lease = pointer("J-2") }},
		{"unresolved-body", "held", func(p *Post) { body(t, p, map[string]string{"ref": "bodies/01K9J000000000000000000001.json"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := side(t, "result", 4)
			tc.mutate(&p)
			s := Fold(append(base(t), p))
			if v := verdict(s, p.ID); v.Kind != tc.kind {
				t.Fatalf("verdict = %+v, want %s", v, tc.kind)
			}
			if len(s.Jobs["J-1"].Results) != 0 {
				t.Fatal("uncounted result changed Job")
			}
		})
	}
	t.Run("stale-reference-still-seen", func(t *testing.T) {
		p := sample(t, "post", 4)
		p.Stale = true
		r := side(t, "result", 5)
		r.Re = &p.ID
		s := Fold(append(base(t), p, r))
		if verdict(s, r.ID).Kind != "counted" {
			t.Fatal("stale target should still satisfy presence dependency")
		}
	})
	t.Run("references-arrive-later", func(t *testing.T) {
		p := sample(t, "post", 4)
		r := side(t, "result", 5)
		r.Re = &p.ID
		before := append(base(t), r)
		if verdict(Fold(before), r.ID).Kind != "held" || verdict(Fold(append(before, p)), r.ID).Kind != "counted" {
			t.Fatal("held result did not release when reference arrived")
		}
	})
}

func TestControlTenures(t *testing.T) {
	posts := base(t)
	old := move(t, 4, Stale)
	newer := move(t, 5, Accepted)
	newer.Fence = 3
	newer.Worker = "next@01K9J000000000000000000002"
	newer.At = "2000-01-01T00:00:00.000Z"
	posts = append(posts, newer, old)
	if got := Fold(posts).Jobs["J-1"].Status; got != Accepted {
		t.Fatalf("control tenure order: %s", got)
	}
	newer.Stale = true
	if got := Fold(append(base(t), old, newer)).Jobs["J-1"].Status; got != Stale {
		t.Fatalf("stale control counted: %s", got)
	}
	newer.Stale, newer.Gen, newer.Fence = false, 1, 1
	if got := Fold(append(base(t), old, newer)).Jobs["J-1"].Status; got != Accepted {
		t.Fatalf("generation must precede fence: %s", got)
	}
}

func TestQuestionAnswerSelection(t *testing.T) {
	q := sample(t, "question", 4)
	q.Job = pointer("J-1")
	changeBody(t, &q, func(v map[string]any) { v["who"] = "guest-one" })
	a := sample(t, "answer", 5)
	a.Author, a.Tier, a.Re = Author{ID: "guest-one", Role: "guest"}, PersonGuest, &q.ID
	b := sample(t, "answer", 6)
	b.Re, b.Seq = &q.ID, 2
	posts := append(base(t), q, a, b)
	if len(Fold(posts).Jobs["J-1"].OpenQuestions) != 1 {
		t.Fatal("answers cannot choose themselves")
	}
	c := sample(t, "control", 7)
	c.Seq = 7
	body(t, &c, map[string]any{"target": q.ID, "answer": a.ID, "status": "answered"})
	state := Fold(append(posts, c))
	if len(state.Jobs["J-1"].OpenQuestions) != 0 {
		t.Fatalf("selected answer did not close question: %+v", state.Verdicts)
	}
	a.Author.ID = "different-guest"
	state = Fold(append(base(t), q, a, b, c))
	if len(state.Jobs["J-1"].OpenQuestions) != 1 || verdict(state, a.ID).Kind != "ignored" {
		t.Fatal("guest answered another guest's question")
	}
}

func TestResultTargetAndTenureBoundaries(t *testing.T) {
	t.Run("control-targets-result", func(t *testing.T) {
		r := side(t, "result", 4)
		c := move(t, 5, Accepted)
		changeBody(t, &c, func(v map[string]any) { v["target"] = r.ID })
		state := Fold(append(base(t), r, c))
		if state.Jobs["J-1"].Status != Accepted {
			t.Fatalf("result target did not resolve: %+v", state.Verdicts)
		}
	})
	t.Run("exit-counts-once", func(t *testing.T) {
		x := side(t, "worker-exit", 4)
		body(t, &x, map[string]string{"reason": "loop"})
		y := x
		y.ID, y.Seq = fmt.Sprintf("%026d", 5), 5
		state := Fold(append(base(t), x, y))
		if state.Jobs["J-1"].ReviewRounds != 1 || verdict(state, y.ID).Kind != "ignored" {
			t.Fatal("second exit consumed another round")
		}
	})
	t.Run("control-without-task-lease", func(t *testing.T) {
		c := move(t, 4, Stale)
		c.Lease, c.Fence = nil, 0
		state := Fold(append(base(t), c))
		if state.Jobs["J-1"].Status != Running || verdict(state, c.ID).Kind != "bad" {
			t.Fatal("unleased control acted")
		}
	})
	t.Run("mixed-tasks-refused", func(t *testing.T) {
		posts := base(t)
		posts[1].Task = fmt.Sprintf("%026d", 99)
		if state := Fold(posts); len(state.Jobs) != 0 {
			t.Fatal("Job IDs from different tasks must not be combined")
		}
	})
	t.Run("bad-record-diagnostics-stable", func(t *testing.T) {
		p := side(t, "result", 4)
		p.Seq, p.Gen, p.TypeV = -1, -1, -1
		first := Fold(append(base(t), p))
		for range 10 {
			if next := Fold(append(base(t), p)); !reflect.DeepEqual(first, next) {
				t.Fatal("invalid-record verdict depends on map traversal")
			}
		}
	})
}
