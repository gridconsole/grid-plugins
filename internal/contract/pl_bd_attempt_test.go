package contract

import (
	"math/rand"
	"reflect"
	"testing"
)

// plBdRetried is J-1 with attempt 1 assigned and acked (fence 2), then attempt 2
// assigned and acked (fence 3).
func plBdRetried(t *testing.T) []Post {
	t.Helper()
	a, k := retry(t, 20, 2, 3)
	return append(base(t), a, k)
}

// plBdOld returns attempt 1's result, checkpoint and done exit, with ids from n,
// each changed by mutate.
func plBdOld(t *testing.T, n int, mutate func(*Post)) []Post {
	t.Helper()
	var out []Post
	for i, typ := range []string{"result", "checkpoint", "worker-exit"} {
		p := side(t, typ, n+i)
		if typ == "worker-exit" {
			body(t, &p, map[string]any{"reason": "done"})
		}
		mutate(&p)
		out = append(out, p)
	}
	return out
}

// plBdSide is a Job-side post of attempt 2's tenure (fence 3).
func plBdSide(t *testing.T, typ string, id int) Post {
	t.Helper()
	p := side(t, typ, id)
	p.Attempt, p.Fence = pointer(2), 3
	return p
}

func plBdShuffle(posts []Post) []Post {
	out := append([]Post(nil), posts...)
	rand.New(rand.NewSource(7)).Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	for i := range out {
		out[i].At = "2099-01-01T00:00:00.000Z"
	}
	return out
}

// Rows 1 and 4 (V9, F8): a Job-side post of an attempt that a newer assign
// superseded folds stale with reason "older attempt" and changes nothing, in any
// input order.
func TestPlBdOlderAttemptFoldsStale(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Post)
	}{
		// The reader marked it stale: its tenure file ended (feed/read.go).
		{"marked stale by the reader", func(p *Post) { p.Stale = true }},
		// Not attempt 1's tenure: no counted assign consumed it.
		{"another worker", func(p *Post) { p.Worker = "other@01K9J000000000000000000002" }},
		{"another fence", func(p *Post) { p.Fence = 9 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			without := plBdRetried(t)
			want := Fold(without).Jobs["J-1"]
			if want == nil || want.Status != Running || want.Attempt != 2 {
				t.Fatalf("attempt 2 did not start: %+v", want)
			}
			old := plBdOld(t, 10, tc.mutate)
			with := append(append([]Post(nil), without...), old...)
			for _, in := range [][]Post{with, plBdShuffle(with)} {
				state := Fold(in)
				for _, p := range old {
					if v := verdict(state, p.ID); v.Kind != "stale" || v.Reason != "older attempt" {
						t.Errorf("%s of attempt 1 folds %s %q, want stale %q", p.Type, v.Kind, v.Reason, "older attempt")
					}
				}
				if got := state.Jobs["J-1"]; !reflect.DeepEqual(got, want) {
					t.Errorf("older attempt changed the Job:\n got %+v\nwant %+v", got, want)
				}
			}
		})
	}
}

// What the attempt-stale rule leaves as it is.
func TestPlBdNotOlderAttempt(t *testing.T) {
	t.Run("attempt 1 checkpoint counted under its own tenure survives the retry", func(t *testing.T) {
		c := side(t, "checkpoint", 4)
		a, k := retry(t, 20, 2, 3)
		posts := append(base(t), c, a, k)
		for _, in := range [][]Post{posts, plBdShuffle(posts)} {
			state := Fold(in)
			if v := verdict(state, c.ID); v.Kind != "counted" || state.Jobs["J-1"].Checkpoint == nil || state.Jobs["J-1"].Attempt != 2 {
				t.Fatalf("counted checkpoint of attempt 1: %+v, Job %+v", v, state.Jobs["J-1"])
			}
		}
	})
	t.Run("current attempt matching no assign stays ignored", func(t *testing.T) {
		r := plBdSide(t, "result", 30)
		r.Fence = 99
		state := Fold(append(plBdRetried(t), r))
		if v := verdict(state, r.ID); v.Kind != "ignored" || v.Reason != "no matching counted assign for this tenure" {
			t.Fatalf("attempt 2 result off its tenure: %+v", v)
		}
	})
	t.Run("post after its attempt's exit stays ignored", func(t *testing.T) {
		x := side(t, "worker-exit", 4)
		body(t, &x, map[string]any{"reason": "crash"})
		r := side(t, "result", 5)
		a, k := retry(t, 20, 2, 3)
		state := Fold(append(base(t), x, r, a, k))
		if v := verdict(state, r.ID); v.Kind != "ignored" || v.Reason != "attempt already exited" {
			t.Fatalf("attempt 1 result after its exit: %+v", v)
		}
	})
	t.Run("a stale control keeps its reason", func(t *testing.T) {
		c := move(t, 30, Accepted)
		c.Stale = true
		state := Fold(append(plBdRetried(t), c))
		if v := verdict(state, c.ID); v.Kind != "stale" || v.Reason != "past tenure end offset" {
			t.Fatalf("stale control: %+v", v)
		}
	})
}

// Row 3 at the fold: an exit done with a result of that attempt folds Submitted,
// "awaiting judgement", with no control post. A result of an older attempt does
// not count, and an exit done with no result is left to the tick.
func TestPlBdDoneWithResultFoldsSubmitted(t *testing.T) {
	done := func(t *testing.T, p Post) Post {
		body(t, &p, map[string]any{"reason": "done"})
		return p
	}
	crash := side(t, "worker-exit", 5)
	body(t, &crash, map[string]any{"reason": "crash"})
	a, k := retry(t, 20, 2, 3)
	for _, tc := range []struct {
		name   string
		posts  func(*testing.T) []Post
		status Status
		reason string
	}{
		{"result then done", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 4), done(t, side(t, "worker-exit", 5)))
		}, Submitted, "awaiting judgement"},
		{"done with no result", func(t *testing.T) []Post {
			return append(base(t), done(t, side(t, "worker-exit", 4)))
		}, Running, ""},
		{"attempt 2 done with only attempt 1's result", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 4), crash, a, k, done(t, plBdSide(t, "worker-exit", 30)))
		}, Running, ""},
		{"attempt 2 result then done", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 4), crash, a, k, plBdSide(t, "result", 30), done(t, plBdSide(t, "worker-exit", 31)))
		}, Submitted, "awaiting judgement"},
		{"a later accept still wins", func(t *testing.T) []Post {
			return append(base(t), side(t, "result", 4), done(t, side(t, "worker-exit", 5)), move(t, 6, Accepted))
		}, Accepted, "The task holder made this decision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := tc.posts(t)
			for _, in := range [][]Post{posts, plBdShuffle(posts)} {
				j := Fold(in).Jobs["J-1"]
				if j == nil || j.Status != tc.status || j.Reason != tc.reason {
					t.Fatalf("Job = %+v, want %s %q", j, tc.status, tc.reason)
				}
			}
		})
	}
}
