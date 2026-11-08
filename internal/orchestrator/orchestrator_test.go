package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/question"
	"github.com/gridconsole/grid-plugins/internal/session"
)

const taskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const machine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

type fixture struct {
	s      Store
	now    time.Time
	worker feed.Writer
}

func setup(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{now: time.Now().UTC()}
	f.s = Store{Feed: feed.Store{TaskDir: t.TempDir(), TaskID: taskID}, Writer: feed.Writer{Author: feed.Author{ID: "lead", Role: "lead"}, Worker: "lead@" + machine, Lease: "task"}, Owner: "owner", Now: func() time.Time { return f.now }, Assignee: "worker@" + machine}
	grant, err := (lease.TaskStore{Feed: f.s.Feed}).TakeTask(f.s.Writer, os.Getpid(), contract.FormatTime(f.now))
	if err != nil {
		t.Fatal(err)
	}
	f.s.Writer.Fence = grant.Fence
	if _, err := f.s.Feed.Handover(feed.Input{Body: map[string]any{}}, f.s.Writer); err != nil {
		t.Fatal(err)
	}
	f.worker = feed.Writer{Author: feed.Author{ID: "worker", Role: "worker"}, Worker: f.s.Assignee, Lease: "J-1"}
	return f
}
func jobBody(t *testing.T, id string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = id
	return body
}
func (f *fixture) job(t *testing.T, id string, after []any) {
	t.Helper()
	body := jobBody(t, id)
	if after != nil {
		body["after"] = after
	}
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}
func (f *fixture) post(t *testing.T, in feed.Input, w feed.Writer) feed.Record {
	t.Helper()
	r, err := f.s.Feed.Post(in, w)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func (f *fixture) tick(t *testing.T) Output {
	t.Helper()
	out, err := f.s.Tick()
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func (f *fixture) state(t *testing.T) *contract.Job {
	t.Helper()
	s, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	return s.Jobs["J-1"]
}
func (f *fixture) ack(t *testing.T) {
	t.Helper()
	j := f.state(t)
	f.worker.Fence = j.Assign.Fence
	f.worker.Gen = j.Generation
	_, err := Write(f.s.Feed, f.worker, feed.Input{Type: "ack", Job: j.ID, Attempt: j.Attempt, Body: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
}
func (f *fixture) exit(t *testing.T, reason string) {
	t.Helper()
	j := f.state(t)
	body := map[string]any{}
	if reason != "" {
		body["reason"] = reason
	}
	_, err := Write(f.s.Feed, f.worker, feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: body})
	if err != nil {
		t.Fatal(err)
	}
}
func resultBody() map[string]any {
	return map[string]any{"outcome": "finished", "changed": []string{}, "evidence": []string{}, "acceptance": []any{}, "verified": nil, "must_fix": []any{}, "found": []string{}, "proposed_jobs": []any{}, "decisions_needed": []string{}, "knowledge": []string{}}
}

func TestMoves(t *testing.T) {
	for _, tc := range []struct {
		name, reason string
		want         contract.Status
		attempt      int
	}{
		{"crash", "crash", contract.Assigned, 2}, {"missing reason", "", contract.Assigned, 2}, {"loop", "loop", contract.Assigned, 2}, {"usage_limit", "usage_limit", contract.Parked, 1}, {"cancelled", "cancelled", contract.Cancelled, 1}, {"done", "done", contract.Submitted, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			out := f.tick(t)
			if out.Assigned != 1 {
				t.Fatal(out)
			}
			f.ack(t)
			f.exit(t, tc.reason)
			f.tick(t)
			j := f.state(t)
			if j.Status != tc.want || j.Attempt != tc.attempt {
				t.Fatalf("%+v", j)
			}
		})
	}
}
func TestLateResultRefused(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	old := f.worker
	f.exit(t, "crash")
	f.tick(t)
	before, _ := f.s.Feed.Read(feed.Filter{})
	_, err := Write(f.s.Feed, old, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()})
	if !errors.Is(err, feed.ErrStaleLease) {
		t.Fatal(err)
	}
	after, _ := f.s.Feed.Read(feed.Filter{})
	if len(before.Records) != len(after.Records) {
		t.Fatal("late result wrote a record")
	}
}
func TestUsageLimitReoffer(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	fence := f.worker.Fence
	f.exit(t, "usage_limit")
	f.tick(t)
	if f.tick(t).Assigned != 0 {
		t.Fatal("ignored capacity hold")
	}
	f.s.UsageReady = func(string) bool { return true }
	if f.tick(t).Assigned != 1 {
		t.Fatal("not reoffered")
	}
	j := f.state(t)
	if j.Attempt != 1 || j.Assign.Fence <= fence {
		t.Fatalf("%+v", j)
	}
}
func TestCrashLoopPark(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	incidents := 0
	f.s.Incident = func(_ string, exits []string) error {
		incidents++
		if len(exits) != 3 {
			t.Fatal(exits)
		}
		return nil
	}
	for range 3 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	j := f.state(t)
	if j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatalf("%+v", j)
	}
	f.tick(t)
	state, _ := f.s.Feed.State()
	if len(state.Questions) != 1 || incidents != 1 {
		t.Fatalf("questions %d incidents %d", len(state.Questions), incidents)
	}
	for id := range state.Questions {
		f.post(t, feed.Input{Type: "answer", Re: id, Body: map[string]any{"answer": "Retry"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
	}
	if f.tick(t).Assigned != 1 {
		t.Fatal("owner answer did not reset park")
	}
}
func TestProgressResetsCrashLoop(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for n := 0; n < 2; n++ {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, "research.md"), []byte("new evidence"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.ack(t)
	f.exit(t, "crash")
	f.tick(t)
	j := f.state(t)
	if j.Status == contract.Parked {
		t.Fatalf("progress did not reset crashes: %+v", j)
	}
	if j.Status != contract.Rejected {
		t.Fatal("max_rounds not enforced", j.Status)
	}
}
func TestLiveness(t *testing.T) {
	for _, tc := range []struct {
		name    string
		probe   session.ProcessState
		expired bool
		want    int
	}{
		{"unknown", session.Unknown, true, 1}, {"alive after sleep", session.Alive, true, 1}, {"dead", session.Dead, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			f.tick(t)
			f.ack(t)
			j := f.state(t)
			until := f.now.Add(time.Minute)
			if tc.expired {
				until = f.now.Add(-time.Hour)
			}
			live := session.Live{Lease: j.ID, Holder: f.worker.Worker, Fence: uint64(f.worker.Fence), Attempt: j.Attempt, PID: 123, PIDStart: "example", Until: until}
			dir := filepath.Join(f.s.Feed.Dir(), ".live")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := writeJSON(filepath.Join(dir, fmt.Sprintf("%s.%d.json", live.Holder, live.Fence)), live); err != nil {
				t.Fatal(err)
			}
			f.s.Probe = func(session.Live) session.ProcessState { return tc.probe }
			before, _ := f.s.Feed.Read(feed.Filter{})
			f.tick(t)
			after, _ := f.s.Feed.Read(feed.Filter{})
			if f.state(t).Attempt != tc.want {
				t.Fatal(f.state(t))
			}
			if tc.probe == session.Unknown && len(before.Records) != len(after.Records) {
				t.Fatal("unknown liveness posted to Feed")
			}
		})
	}
	t.Run("no ack expires", func(t *testing.T) {
		f := setup(t)
		f.job(t, "J-1", nil)
		f.tick(t)
		f.now = f.now.Add(11 * time.Minute)
		f.tick(t)
		if f.state(t).Attempt != 2 {
			t.Fatal(f.state(t))
		}
	})
}
func TestReadySpawnAgree(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.job(t, "J-3", []any{"gate:Q-1"})
	f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Wait?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "timer", "at": contract.FormatTime(f.now.Add(time.Hour))}}}, f.s.Writer)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ready.Ready, []string{"J-1"}) {
		t.Fatal(ready)
	}
	if out := f.tick(t); out.Assigned != len(ready.Ready) {
		t.Fatal(out, ready)
	}
	j := f.state(t)
	if err := f.s.control(j, contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(2 * time.Hour)
	snap, err = Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	ready, err = Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ready.Ready, []string{"J-2", "J-3"}) {
		t.Fatal(ready)
	}
	if out := f.tick(t); out.Assigned != len(ready.Ready) {
		t.Fatal(out, ready)
	}
}
func TestWakeIdempotency(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	event, _ := feed.NewID()
	in := feed.Input{Type: "result", Job: "J-1", Attempt: 1, Wake: event + "/1", Body: resultBody()}
	first, err := Write(f.s.Feed, f.worker, in)
	if err != nil {
		t.Fatal(err)
	}
	in.Wake = event + "/2"
	second, err := Write(f.s.Feed, f.worker, in)
	if !errors.Is(err, ErrDuplicateWake) || second.ID != first.ID {
		t.Fatal(second, err)
	}
	out := f.tick(t)
	if out.Wake == nil || out.Wake.Event != first.ID {
		t.Fatal(out)
	}
	again := f.tick(t).Wake
	if !reflect.DeepEqual(out.Wake, again) {
		t.Fatal("unacknowledged wake lost after pass save", out.Wake, again)
	}
	key := fmt.Sprintf("%s/%d", again.Event, again.N)
	f.post(t, feed.Input{Type: "post", Wake: key, Body: map[string]any{"ref": "task", "kind": "message", "text": "judgement recorded"}}, f.s.Writer)
	if f.tick(t).Wake != nil {
		t.Fatal("acknowledged wake delivered twice")
	}
	next := f.post(t, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()}, f.worker)
	later := f.tick(t).Wake
	if later == nil || later.Event != next.ID || later.N != again.N+1 {
		t.Fatal("wake number reused", later)
	}
}
func TestWakeRateSummary(t *testing.T) {
	f := setup(t)
	snap := Snapshot{State: contract.State{Jobs: map[string]*contract.Job{}, Wakes: []string{}}, Records: map[string]feed.Record{}}
	m := memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{}}
	for n := 0; n < 25; n++ {
		id := fmt.Sprintf("event-%02d", n)
		snap.State.Wakes = append(snap.State.Wakes, id)
		snap.Records[id] = feed.Record{Body: map[string]any{"job": "J-1"}}
		out := f.s.wake(snap, &m)
		if n < 20 && out == nil {
			t.Fatal(n)
		}
		if n >= 20 && out != nil {
			t.Fatal("wake cap exceeded")
		}
		if out != nil {
			key := fmt.Sprintf("%s/%d", out.Event, out.N)
			snap.Records[fmt.Sprint("ack-", n)] = feed.Record{Wake: &key}
		}
	}
	f.s.wake(snap, &m)
	if len(m.Pending) != 5 {
		t.Fatal(len(m.Pending))
	}
	f.now = f.now.Add(time.Hour)
	out := f.s.wake(snap, &m)
	if out == nil || len(out.Events) != 5 {
		t.Fatal("queue was not summarized", out)
	}
	if again := f.s.wake(snap, &m); !reflect.DeepEqual(again, out) {
		t.Fatal("summary lost before acknowledgement", again)
	}
	key := fmt.Sprintf("%s/%d", out.Event, out.N)
	snap.Records["summary-ack"] = feed.Record{Wake: &key}
	if again := f.s.wake(snap, &m); again != nil || len(m.Pending) != 0 {
		t.Fatal("summary acknowledgement left events pending", again, m.Pending)
	}
}
func TestWaitCancellation(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.s.Next(ctx, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestProposedJobsAndCheckpoints(t *testing.T) {
	for _, tc := range []struct {
		name, gap string
		children  int
	}{{"same acceptance", "A1", 1}, {"scope change", "new scope", 0}} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, "plan.md"), []byte("# Plan\n\n- [ ] 1 Build example feature\n  jobs: []\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			f.tick(t)
			f.ack(t)
			body := resultBody()
			body["proposed_jobs"] = []contract.ProposedJob{{Title: "Rework acceptance", Role: "builder", Why: "Finish the original acceptance row", Gap: tc.gap, DiscoveredFrom: "J-1"}}
			if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: body}); err != nil {
				t.Fatal(err)
			}
			out := f.tick(t)
			if out.Wake == nil {
				t.Fatal("result did not wake lead")
			}
			state, err := f.s.Feed.State()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Jobs) != 1+tc.children {
				t.Fatalf("%+v", state.Jobs)
			}
			f.tick(t)
			state, err = f.s.Feed.State()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Jobs) != 1+tc.children {
				t.Fatal("duplicate proposal")
			}
			if tc.children == 1 {
				child := state.Jobs["J-1.1"]
				if child.DiscoveredFrom == nil || *child.DiscoveredFrom != "J-1" {
					t.Fatal(child)
				}
			}
		})
	}
	t.Run("checkpoint survives retry", func(t *testing.T) {
		f := setup(t)
		f.job(t, "J-1", nil)
		f.tick(t)
		f.ack(t)
		_, err := Write(f.s.Feed, f.worker, feed.Input{Type: "checkpoint", Job: "J-1", Attempt: 1, Body: map[string]any{"job": "J-1", "step": "tests", "commit": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}})
		if err != nil {
			t.Fatal(err)
		}
		f.exit(t, "crash")
		f.tick(t)
		if j := f.state(t); j.Checkpoint == nil || j.Checkpoint.Step != "tests" {
			t.Fatal(j)
		}
	})
}

func TestReadyCanonicalOrder(t *testing.T) {
	f := setup(t)
	for _, id := range []string{"J-3", "J-2", "J-1"} {
		body := jobBody(t, id)
		body["created_at"] = "2000-01-01T00:00:00.000Z"
		if id == "J-3" {
			body["created_at"] = "1999-01-01T00:00:00.000Z"
		}
		f.post(t, feed.Input{Type: "job", Job: id, Body: body}, f.s.Writer)
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ready.Ready, []string{"J-3", "J-1", "J-2"}) {
		t.Fatal(ready)
	}
}

func TestGateKinds(t *testing.T) {
	for _, kind := range []string{"person", "timer", "ci", "job"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", []any{"gate:Q-1"})
			gate := map[string]any{"id": "Q-1", "kind": kind}
			switch kind {
			case "timer":
				gate["at"] = contract.FormatTime(f.now.Add(time.Minute))
			case "ci":
				gate["check"] = "tests"
			case "job":
				f.job(t, "J-2", nil)
				gate["job"], gate["state"] = "J-2", "accepted"
			}
			q := f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "owner", "gate": gate}}, f.s.Writer)
			f.tick(t)
			if f.state(t).Assign != nil {
				t.Fatal("gate opened early")
			}
			switch kind {
			case "timer":
				f.now = f.now.Add(2 * time.Minute)
			case "job":
				state, _ := f.s.Feed.State()
				if err := f.s.control(state.Jobs["J-2"], contract.Accepted, "check passed", ""); err != nil {
					t.Fatal(err)
				}
			default:
				f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Proceed"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
			}
			if f.tick(t).Assigned != 1 {
				t.Fatal("gate did not open")
			}
		})
	}
}

func TestCrashLoopRecoversWithoutMetadata(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 2 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	if err := os.Remove(filepath.Join(f.s.Feed.Dir(), ".orchestrator.json")); err != nil {
		t.Fatal(err)
	}
	f.ack(t)
	f.exit(t, "crash")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatal(j)
	}
}

func TestUsageHoldBlocksHostDemand(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.exit(t, "usage_limit")
	f.tick(t)
	f.job(t, "J-2", nil)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	snap.Assignee = f.s.Assignee
	ready, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready.Ready) != 0 {
		t.Fatal(ready)
	}
	if f.tick(t).Assigned != 0 {
		t.Fatal("held host assigned work")
	}
	f.s.UsageReady = func(string) bool { return true }
	if f.tick(t).Assigned != 2 {
		t.Fatal("capacity did not restore demand")
	}
}

func TestReviewCapacityUsesReady(t *testing.T) {
	f := setup(t)
	for n := 1; n <= 4; n++ {
		id := fmt.Sprintf("J-%d", n)
		body := jobBody(t, id)
		body["role"], body["review"] = "reviewer", "L2"
		f.post(t, feed.Input{Type: "job", Job: id, Body: body}, f.s.Writer)
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	snap.Assignee = f.s.Assignee
	ready, err := Ready(snap, f.now)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready.Ready) != 3 || len(ready.Waiting) != 1 || ready.Waiting[0].Reason != "review_capacity" {
		t.Fatal(ready)
	}
	if f.tick(t).Assigned != 3 {
		t.Fatal("spawn demand disagreed")
	}
	if f.tick(t).Assigned != 0 {
		t.Fatal("capacity exceeded")
	}
}

func TestHeartbeatExpiryAndWake(t *testing.T) {
	for _, wake := range []bool{false, true} {
		t.Run(fmt.Sprint(wake), func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			f.tick(t)
			f.ack(t)
			j := f.state(t)
			live := session.Live{Lease: j.ID, Holder: f.worker.Worker, Fence: uint64(f.worker.Fence), Attempt: j.Attempt, PID: 123, PIDStart: "2000-01-01T00:00:00.000Z", BeatAt: f.now.Add(-11 * time.Minute), Until: f.now.Add(-time.Minute)}
			dir := filepath.Join(f.s.Feed.Dir(), ".live")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := writeJSON(filepath.Join(dir, fmt.Sprintf("%s.%d.json", live.Holder, live.Fence)), live); err != nil {
				t.Fatal(err)
			}
			f.s.AfterWake = wake
			f.tick(t)
			want := 2
			if wake {
				want = 1
			}
			if f.state(t).Attempt != want {
				t.Fatal(f.state(t))
			}
		})
	}
}

func TestParkOnlyResumesForItsAppliedRetry(t *testing.T) {
	for _, kind := range []string{"keep", "chatter", "unrelated", "ambiguous", "file"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			f.tick(t)
			for range 3 {
				f.ack(t)
				f.exit(t, "crash")
				f.tick(t)
			}
			state, err := f.s.Feed.State()
			if err != nil {
				t.Fatal(err)
			}
			var own string
			for id := range state.Questions {
				own = id
			}
			person := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine}
			switch kind {
			case "keep":
				f.post(t, feed.Input{Type: "answer", Re: own, Body: map[string]any{"answer": "Keep parked"}}, person)
			case "chatter":
				f.post(t, feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": "still looking"}}, feed.Writer{Author: f.worker.Author, Worker: f.worker.Worker})
			case "unrelated":
				other := f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Other decision?", "who": "owner", "blocking": false}}, f.s.Writer)
				f.post(t, feed.Input{Type: "answer", Re: other.ID, Body: map[string]any{"answer": "Retry"}}, person)
			case "ambiguous":
				f.post(t, feed.Input{Type: "answer", Re: own, Body: map[string]any{"answer": "Retry"}}, person)
				f.post(t, feed.Input{Type: "answer", Re: own, Body: map[string]any{"answer": "Keep parked"}}, person)
			case "file":
				if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, "research.md"), []byte("new evidence"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if f.tick(t).Assigned != 0 || f.state(t).Status != contract.Parked {
				t.Fatal("unrelated progress or unselected answer released park")
			}
			if err := os.Remove(filepath.Join(f.s.Feed.Dir(), ".orchestrator.json")); err != nil {
				t.Fatal(err)
			}
			if f.tick(t).Assigned != 0 || f.state(t).Status != contract.Parked {
				t.Fatal("metadata recovery released park")
			}
		})
	}
}

func TestParkGateReservesQuestionFileIDs(t *testing.T) {
	f := setup(t)
	qs := question.Store{TaskDir: f.s.Feed.TaskDir}
	in := question.Input{Question: "Other decision?", Who: "owner", Options: []string{"Wait"}, Recommended: "Wait", Default: "Wait", Applies: f.now.Add(time.Hour)}
	first, err := qs.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 3 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	records, err := f.s.Feed.Read(feed.Filter{Type: "question"})
	if err != nil {
		t.Fatal(err)
	}
	gate := records.Records[0].Body["gate"].(map[string]any)["id"]
	if gate == first.ID {
		t.Fatal("crash gate reused question-file id", gate)
	}
	next, err := qs.Add(in)
	if err != nil {
		t.Fatal(err)
	}
	if next.ID == gate {
		t.Fatal("question file reused crash gate id", gate)
	}
}

func TestProgressUsesSizeAndMtimeCache(t *testing.T) {
	f := setup(t)
	path := filepath.Join(f.s.Feed.TaskDir, "research.md")
	if err := os.WriteFile(path, []byte("first"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 2 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	if err := os.WriteFile(path, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	f.ack(t)
	f.exit(t, "crash")
	f.tick(t)
	if f.state(t).Status != contract.Parked {
		t.Fatal("unchanged size and mtime reread file content")
	}
}

func TestIdlePassDoesNotRewriteProjections(t *testing.T) {
	f := setup(t)
	f.tick(t)
	paths := []string{"state.json", "needs-you.json", ".orchestrator.json"}
	stamp := time.Unix(1000000000, 0)
	for _, name := range paths {
		path := filepath.Join(f.s.Feed.Dir(), name)
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	f.tick(t)
	for _, name := range paths {
		info, err := os.Stat(filepath.Join(f.s.Feed.Dir(), name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(stamp) {
			t.Errorf("idle pass rewrote %s", name)
		}
	}
}

func TestSpawnRequestRemovedAfterAck(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.tick(t)
	if _, err := os.Stat(filepath.Join(f.s.Feed.TaskDir, "spawned", "J-1.json")); !os.IsNotExist(err) {
		t.Fatal("old spawn request survived ack", err)
	}
}
