package contract

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
)

const (
	pbStHomeA = "01K9J00000000000000000000A"
	pbStHomeB = "01K9J00000000000000000000B"
)

// pbStB is p as a board post written on machine m: no worker lease or fence, its by
// <m>:grid.
func pbStB(p Post, m string) BoardPost {
	p.Lease, p.Fence, p.Worker = nil, 0, m+":grid"
	return BoardPost{Post: p, By: m + ":grid"}
}

// pbStJob is J-1's record, with seq 1, naming home A.
func pbStJob(t *testing.T) BoardPost {
	t.Helper()
	j := side(t, "job", 1)
	changeBody(t, &j, func(v map[string]any) { v["home"] = pbStHomeA })
	return pbStB(j, pbStHomeA)
}

// pbStAssign is the orchestrator's assign of J-1's attempt to worker@A, from machine
// m at gen, with id and seq n.
func pbStAssign(t *testing.T, n, attempt int, gen int64, m string) BoardPost {
	t.Helper()
	a := side(t, "control", n)
	a.Author = Author{ID: "orchestrator", Role: "cli"}
	a.Gen = gen
	changeBody(t, &a, func(v map[string]any) {
		as := v["assign"].(map[string]any)
		as["attempt"], as["to"] = attempt, "worker@"+pbStHomeA
	})
	return pbStB(a, m)
}

// pbStAck is worker's ack of the assign, written on A.
func pbStAck(t *testing.T, n, attempt int, assign BoardPost) BoardPost {
	t.Helper()
	k := side(t, "ack", n)
	k.Job, k.Attempt, k.After = pointer("J-1"), pointer(attempt), []string{assign.ID}
	k.Gen = assign.Gen
	return pbStB(k, pbStHomeA)
}

func pbStMove(t *testing.T, n int, gen int64, to string) BoardPost {
	t.Helper()
	m := side(t, "task-move", n)
	m.Gen = gen
	changeBody(t, &m, func(v map[string]any) { v["from"], v["to"] = pbStHomeA, to })
	return pbStB(m, pbStHomeA)
}

func pbStOrders(posts []BoardPost) [][]BoardPost {
	reversed := slices.Clone(posts)
	slices.Reverse(reversed)
	rotated := append(slices.Clone(posts[len(posts)/2:]), posts[:len(posts)/2]...)
	return [][]BoardPost{posts, reversed, rotated}
}

func pbStVerdict(t *testing.T, s State, p BoardPost, kind, reason string) {
	t.Helper()
	if v := verdict(s, p.ID); v.Kind != kind || reason != "" && v.Reason != reason {
		t.Fatalf("%s %s folds %s %q, want %s %q", p.Type, p.ID, v.Kind, v.Reason, kind, reason)
	}
}

// V9: a fold on a migrated home sorts (gen, seq, id). Each pair of status controls
// here sorts the other way by today's (gen, fence, worker, seq).
func TestPbStFoldSortsGenSeqID(t *testing.T) {
	j, a := pbStJob(t), pbStAssign(t, 2, 1, 0, pbStHomeA)
	moveAt := func(n int, seq int64, fence int64, worker string, status Status) BoardPost {
		p := pbStB(move(t, n, status), pbStHomeB)
		p.Seq, p.Fence, p.Worker, p.Lease = seq, fence, worker, pointer("task")
		p.Legacy = "counted"
		return p
	}
	cases := []struct {
		name  string
		posts []BoardPost
		want  Status
	}{
		{"seq before fence", []BoardPost{j, a, moveAt(4, 4, 9, "a@x", Accepted), moveAt(5, 5, 1, "a@x", Cancelled)}, Cancelled},
		{"seq before worker", []BoardPost{j, a, moveAt(4, 4, 1, "z@x", Cancelled), moveAt(5, 5, 1, "a@x", Accepted)}, Accepted},
		{"gen before seq", []BoardPost{j, a, func() BoardPost { p := moveAt(4, 9, 1, "a@x", Accepted); return p }(), func() BoardPost { p := moveAt(5, 2, 1, "a@x", Cancelled); p.Gen = 1; return p }()}, Cancelled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, in := range pbStOrders(c.posts) {
				s, _ := FoldBoard(in)
				if got := s.Jobs["J-1"].Status; got != c.want {
					t.Fatalf("status %s, want %s", got, c.want)
				}
			}
		})
	}
}

// F5: two posts with one seq tie by id; the higher id is folded last.
func TestPbStFoldSameSeqTiesByID(t *testing.T) {
	j, a := pbStJob(t), pbStAssign(t, 2, 1, 0, pbStHomeA)
	lo := pbStB(move(t, 7, Accepted), pbStHomeB)
	hi := pbStB(move(t, 8, Rejected), pbStHomeB)
	lo.Seq, hi.Seq, lo.Legacy, hi.Legacy = 5, 5, "counted", "counted"
	for _, in := range pbStOrders([]BoardPost{j, a, hi, lo}) {
		s, _ := FoldBoard(in)
		if got := s.Jobs["J-1"].Status; got != Rejected {
			t.Fatalf("status %s, want rejected (the higher id last)", got)
		}
	}
}

// V9: a control counts only from the home of its gen; a non-home control, a person's
// as well, is a request: counted, and it moves nothing.
func TestPbStFoldControlCountsOnlyFromHome(t *testing.T) {
	j := pbStJob(t)
	fromB := pbStAssign(t, 2, 1, 0, pbStHomeB)
	person := pbStAssign(t, 3, 1, 0, pbStHomeB)
	person.Author = Author{ID: "lead", Role: "lead"}
	s, loop := FoldBoard([]BoardPost{j, fromB, person})
	pbStVerdict(t, s, fromB, "counted", "")
	pbStVerdict(t, s, person, "counted", "")
	if job := s.Jobs["J-1"]; job.Status != Posted || job.Attempt != 0 {
		t.Fatalf("a request moved the Job: %+v", job)
	}
	if loop.Home != pbStHomeA {
		t.Fatalf("home %q", loop.Home)
	}
	fromA := pbStAssign(t, 4, 1, 0, pbStHomeA)
	ack := pbStAck(t, 5, 1, fromA)
	for _, in := range pbStOrders([]BoardPost{j, fromB, person, fromA, ack}) {
		s, _ = FoldBoard(in)
		if job := s.Jobs["J-1"]; job.Status != Running || job.AssignPost != fromA.ID {
			t.Fatalf("home control did not count: %+v", job)
		}
		pbStVerdict(t, s, ack, "counted", "")
	}
	// An assign must raise the attempt: a second one of attempt 1 from A is ignored.
	again := pbStAssign(t, 6, 1, 0, pbStHomeA)
	s, _ = FoldBoard([]BoardPost{j, fromA, ack, again})
	pbStVerdict(t, s, again, "ignored", "attempt already granted, regressed, or Job closed")
}

// V9 (F10): after a task-move, a refold still counts the old home's controls for the
// old gen, and the new home's for the new gen.
func TestPbStFoldTaskMoveKeepsOldGen(t *testing.T) {
	j := pbStJob(t)
	a1 := pbStAssign(t, 2, 1, 0, pbStHomeA)
	k1 := pbStAck(t, 3, 1, a1)
	mv := pbStMove(t, 4, 1, pbStHomeB)
	lateA := pbStAssign(t, 5, 2, 1, pbStHomeA) // the old home, after the move
	a2 := pbStAssign(t, 6, 2, 1, pbStHomeB)
	oldGenLate := pbStAssign(t, 7, 3, 0, pbStHomeA) // the old home's gen 0, seen late
	posts := []BoardPost{j, a1, k1, mv, lateA, a2, oldGenLate}
	for _, in := range pbStOrders(posts) {
		s, loop := FoldBoard(in)
		if loop.Home != pbStHomeB {
			t.Fatalf("home %q, want B", loop.Home)
		}
		job := s.Jobs["J-1"]
		// gen 0 sorts first: a1, then attempt 3 from A at gen 0; at gen 1 a2 (attempt
		// 2) is below 3 and is ignored, lateA is a request.
		if job.AssignPost != oldGenLate.ID || job.Attempt != 3 {
			t.Fatalf("Job %+v", job)
		}
		if !slices.Contains(job.History, a1.ID) {
			t.Fatal("the old home's first assign is not in the history")
		}
		pbStVerdict(t, s, lateA, "counted", "")
		pbStVerdict(t, s, a2, "ignored", "attempt already granted, regressed, or Job closed")
	}
	// Without the late gen-0 assign the new home's assign of gen 1 counts.
	s, _ := FoldBoard(posts[:6])
	if job := s.Jobs["J-1"]; job.AssignPost != a2.ID || job.Attempt != 2 {
		t.Fatalf("new home's assign did not count: %+v", job)
	}
}

// V9 (decision point 4): the fold reports the current home from the posts. A move by
// someone not of the team names no home.
func TestPbStFoldReportsHomeFromPosts(t *testing.T) {
	j := pbStJob(t)
	_, loop := FoldBoard([]BoardPost{j})
	if loop.Home != pbStHomeA {
		t.Fatalf("home %q, want A", loop.Home)
	}
	mv := pbStMove(t, 2, 1, pbStHomeB)
	_, loop = FoldBoard([]BoardPost{j, mv})
	if loop.Home != pbStHomeB {
		t.Fatalf("home %q, want B", loop.Home)
	}
	guest := pbStMove(t, 3, 2, "01K9J00000000000000000000C")
	guest.Author = Author{ID: "g", Role: "guest"}
	_, loop = FoldBoard([]BoardPost{j, mv, guest})
	if loop.Home != pbStHomeB {
		t.Fatalf("a guest's move changed the home to %q", loop.Home)
	}
	if _, plain := FoldLoop([]Post{j.Post}); plain.Home != "" {
		t.Fatalf("a JSONL fold has a home %q", plain.Home)
	}
}

// V9: a duplicate claim folds ignored, the first by (gen, seq, id) counting.
func TestPbStFoldDuplicateClaimIgnored(t *testing.T) {
	j, a := pbStJob(t), pbStAssign(t, 2, 1, 0, pbStHomeA)
	started := func(n int, m, firing string) BoardPost {
		p := side(t, "started", n)
		p.Job, p.Attempt = pointer("J-1"), pointer(1)
		if firing != "" {
			changeBody(t, &p, func(v map[string]any) { v["firing"] = firing })
		}
		return pbStB(p, m)
	}
	first, second := started(3, pbStHomeA, ""), started(4, pbStHomeB, "")
	for _, in := range pbStOrders([]BoardPost{j, a, second, first}) {
		s, _ := FoldBoard(in)
		pbStVerdict(t, s, first, "counted", "")
		pbStVerdict(t, s, second, "ignored", "duplicate claim")
		if s.Jobs["J-1"].Status != Running {
			t.Fatalf("status %s", s.Jobs["J-1"].Status)
		}
	}
	f1, f2 := started(5, pbStHomeA, "2026-01-02T03:04:05.000Z"), started(6, pbStHomeA, "2026-01-02T04:04:05.000Z")
	s, _ := FoldBoard([]BoardPost{j, a, f1, f2})
	pbStVerdict(t, s, f1, "counted", "")
	pbStVerdict(t, s, f2, "counted", "")
}

// V9: legacy verdicts are honoured: counted is authority as it was on the JSONL feed,
// stale stays stale, and any other is folded again.
func TestPbStFoldLegacyVerdicts(t *testing.T) {
	j := pbStJob(t)
	a := pbStAssign(t, 2, 1, 0, pbStHomeB) // not the home
	a.Author = Author{ID: "lead", Role: "lead"}
	a.Legacy = "counted"
	k := pbStAck(t, 3, 1, a)
	k.By = pbStHomeB + ":other" // not the assign's to
	k.Legacy = "counted"
	st := pbStB(side(t, "result", 4), pbStHomeA)
	st.Job, st.Attempt, st.Legacy = pointer("J-1"), pointer(1), "stale"
	re := pbStAssign(t, 5, 2, 0, pbStHomeA)
	re.Legacy = "ignored"
	s, _ := FoldBoard([]BoardPost{j, a, k, st})
	if job := s.Jobs["J-1"]; job.AssignPost != a.ID || job.Status != Running {
		t.Fatalf("legacy counted lost its authority: %+v", job)
	}
	pbStVerdict(t, s, k, "counted", "")
	pbStVerdict(t, s, st, "stale", "legacy verdict stale")
	s, _ = FoldBoard([]BoardPost{j, a, k, st, re})
	pbStVerdict(t, s, re, "counted", "")
	if job := s.Jobs["J-1"]; job.AssignPost != re.ID || job.Attempt != 2 {
		t.Fatalf("legacy ignored was not folded again: %+v", job)
	}
}

// pbStBenchPosts is a board of 6,000 posts: 200 Jobs, each with its record, an
// assign, an ack, 26 checkpoints and a result.
func pbStBenchPosts(b *testing.B) []BoardPost {
	t := &testing.T{}
	var posts []BoardPost
	n := 0
	next := func() int { n++; return n }
	for i := 1; i <= 200; i++ {
		job := fmt.Sprintf("J-%d", i)
		jr := side(t, "job", next())
		changeBody(t, &jr, func(v map[string]any) { v["id"] = job; v["home"] = pbStHomeA })
		jr.Job = pointer(job)
		posts = append(posts, pbStB(jr, pbStHomeA))
		a := pbStAssign(t, next(), 1, 0, pbStHomeA)
		changeBody(t, &a.Post, func(v map[string]any) { v["assign"].(map[string]any)["job"] = job })
		posts = append(posts, a)
		k := pbStAck(t, next(), 1, a)
		k.Job = pointer(job)
		posts = append(posts, k)
		for c := 0; c < 26; c++ {
			p := pbStB(side(t, "checkpoint", next()), pbStHomeA)
			p.Job, p.Attempt = pointer(job), pointer(1)
			posts = append(posts, p)
		}
		r := pbStB(side(t, "result", next()), pbStHomeA)
		r.Job, r.Attempt = pointer(job), pointer(1)
		posts = append(posts, r)
	}
	if len(posts) != 6000 {
		b.Fatalf("%d posts", len(posts))
	}
	return posts
}

// V15: the fold at 6,000 posts. A measurement, recorded in the lane's proof table.
func BenchmarkPbStFoldBoard6000(b *testing.B) {
	posts := pbStBenchPosts(b)
	s, _ := FoldBoard(posts)
	if len(s.Jobs) != 200 || s.Jobs["J-200"].Status != Running {
		data, _ := json.Marshal(s.Jobs["J-200"])
		b.Fatalf("bench board folds wrong: %d Jobs, %s", len(s.Jobs), data)
	}
	b.ResetTimer()
	for range b.N {
		FoldBoard(posts)
	}
}

// The same 6,000 posts as a JSONL feed would hold them, for comparison.
func BenchmarkPbStFoldJSONL6000(b *testing.B) {
	board := pbStBenchPosts(b)
	posts := make([]Post, len(board))
	for i, p := range board {
		p.Post.Worker, p.Post.Lease, p.Post.Fence = "worker@"+pbStHomeA, pointer("task"), 1
		if p.Type != "control" && p.Type != "job" && p.Job != nil {
			p.Post.Lease, p.Post.Fence = p.Job, 2
		}
		posts[i] = p.Post
	}
	b.ResetTimer()
	for range b.N {
		FoldLoop(posts)
	}
}
