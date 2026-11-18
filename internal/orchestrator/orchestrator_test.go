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
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/question"
	"github.com/gridconsole/grid-plugins/internal/session"
)

const taskID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const machine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

// ifLwDeadline bounds the wait for a lock-wait event; it proves nothing and its
// failure names what was awaited.
const ifLwDeadline = 30 * time.Second

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
		result       bool
		want         contract.Status
		attempt      int
	}{
		{"crash", "crash", false, contract.Assigned, 2}, {"missing reason", "", false, contract.Assigned, 2}, {"loop", "loop", false, contract.Assigned, 2}, {"usage_limit", "usage_limit", false, contract.Parked, 1}, {"cancelled", "cancelled", false, contract.Cancelled, 1}, {"done", "done", true, contract.Submitted, 1},
		// An empty finish is retried as a new attempt, not left Submitted (empty_finish_test.go).
		{"done with no result", "done", false, contract.Assigned, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.job(t, "J-1", nil)
			out := f.tick(t)
			if out.Assigned != 1 {
				t.Fatal(out)
			}
			f.ack(t)
			if tc.result {
				postResult(t, f)
			}
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
			f.s.AfterWake = tc.name == "alive after sleep"
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

// TestDeadAgentWaitsForItsLiveWorkerExit: the worker holds the task and supervises J-1. Its
// agent's process (the .live pid) has ended, but the worker (the task lease pid) is alive and
// will post the worker-exit. The tick must not retry the attempt first, or that exit is stale.
func TestDeadAgentWaitsForItsLiveWorkerExit(t *testing.T) {
	for _, tc := range []struct {
		name            string
		supervisorAlive bool
		want            int
	}{{"worker alive", true, 1}, {"worker gone", false, 2}} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			f.s.Assignee = "" // the task holder assigns to itself, as worker next does
			f.worker.Worker, f.worker.Author = f.s.Writer.Worker, feed.Author{ID: f.s.Writer.Author.ID, Role: "worker"}
			f.job(t, "J-1", nil)
			f.tick(t)
			f.ack(t)
			j := f.state(t)
			live := session.Live{Lease: j.ID, Holder: f.worker.Worker, Fence: uint64(f.worker.Fence), Attempt: j.Attempt, PID: 123, PIDStart: "example", Until: f.now.Add(time.Minute)}
			dir := filepath.Join(f.s.Feed.Dir(), ".live")
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := writeJSON(filepath.Join(dir, fmt.Sprintf("%s.%d.json", live.Holder, live.Fence)), live); err != nil {
				t.Fatal(err)
			}
			f.s.Probe = func(l session.Live) session.ProcessState {
				if l.PID == os.Getpid() && tc.supervisorAlive {
					return session.Alive
				}
				return session.Dead
			}
			f.tick(t)
			if got := f.state(t); got.Attempt != tc.want {
				t.Fatalf("attempt %d (status %s), want %d", got.Attempt, got.Status, tc.want)
			}
			if !tc.supervisorAlive {
				return
			}
			postResult(t, f)
			f.exit(t, "done") // the worker's exit is not stale
			f.tick(t)
			if got := f.state(t); got.Status != contract.Submitted || got.Attempt != 1 {
				t.Fatalf("after the worker-exit: %s attempt %d, want submitted attempt 1", got.Status, got.Attempt)
			}
		})
	}
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

// The person layer admits only person answers on a person gate; any other gate
// kind keeps the contract's rule, so a ci gate opens on an answer from any role.
func TestGateKinds(t *testing.T) {
	for _, row := range []struct{ kind, role string }{
		{"person", "owner"}, {"timer", ""}, {"ci", "owner"}, {"ci", "worker"}, {"ci", "reviewer"}, {"ci", "agent"}, {"job", ""},
	} {
		kind := row.kind
		t.Run(kind+"/"+row.role, func(t *testing.T) {
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
				f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Proceed"}}, feed.Writer{Author: feed.Author{ID: row.role, Role: row.role}, Worker: row.role + "@" + machine})
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

// The review slots keyed on L2/L3 are gone with the levels and the owner's
// rule of no Grid-made caps (11 Nov q-levels and verdicts v-10): reviewer Jobs
// are ready like any other Job.
func TestReviewerJobsHaveNoCapacityCap(t *testing.T) {
	f := setup(t)
	for n := 1; n <= 4; n++ {
		id := fmt.Sprintf("J-%d", n)
		body := jobBody(t, id)
		body["role"] = "reviewer"
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
	if len(ready.Ready) != 4 || len(ready.Waiting) != 0 {
		t.Fatal(ready)
	}
	if f.tick(t).Assigned != 4 {
		t.Fatal("spawn demand disagreed")
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
	qs := question.Store{TaskDir: f.s.Feed.TaskDir, TaskID: taskID, Writer: feed.Writer{Author: feed.Author{ID: "owner", Role: "member"}, Worker: "owner@" + machine}}
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
	// The question file is a Feed question too now; the crash gate is the Job's.
	var gate any
	for _, r := range records.Records {
		if r.Job != nil && *r.Job == "J-1" {
			gate = r.Body["gate"].(map[string]any)["id"]
		}
	}
	if gate == nil || gate == first.ID {
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

func TestUnknownLivenessEventuallyMarksHostOffline(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.s.Probe = func(session.Live) session.ProcessState { return session.Unknown }
	f.now = f.now.Add(2*session.LeaseDuration - time.Second)
	before, _ := f.s.Feed.Read(feed.Filter{})
	f.tick(t)
	after, _ := f.s.Feed.Read(feed.Filter{})
	if len(before.Records) != len(after.Records) {
		t.Fatal("staled before offline grace")
	}
	f.now = f.now.Add(time.Second)
	out := f.tick(t)
	j := f.state(t)
	if j.Status != contract.Stale || j.Reason != "host_offline" || out.Assigned != 0 {
		t.Fatalf("offline host reused or never staled: %+v", j)
	}
	f.s.Assignee = "worker@01ARZ3NDEKTSV4RRFFQ69G5FAX"
	if f.tick(t).Assigned != 1 || f.state(t).Assign.To != f.s.Assignee {
		t.Fatal("did not retry on another host")
	}
}

func TestNoAckAfterWakeDoesNotExpire(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.s.AfterWake = true
	f.now = f.now.Add(time.Hour)
	f.tick(t)
	if f.state(t).Attempt != 1 {
		t.Fatal("sleep expired unacknowledged assignment")
	}
	f.s.AfterWake = false
	f.tick(t)
	if f.state(t).Attempt != 1 {
		t.Fatal("wake suppression lost before a successful check")
	}
}

func TestAliveProbeDoesNotHideExpiredHeartbeat(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	j := f.state(t)
	dir := filepath.Join(f.s.Feed.Dir(), ".live")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	live := session.Live{Lease: j.ID, Holder: f.worker.Worker, Fence: uint64(f.worker.Fence), Attempt: 1, PID: 123, PIDStart: "example", Until: f.now.Add(-time.Second)}
	if err := writeJSON(filepath.Join(dir, fmt.Sprintf("%s.%d.json", live.Holder, live.Fence)), live); err != nil {
		t.Fatal(err)
	}
	f.s.Probe = func(session.Live) session.ProcessState { return session.Alive }
	f.tick(t)
	if f.state(t).Attempt != 2 {
		t.Fatal("alive process hid expired heartbeat")
	}
}

func TestOfflineGraceStartsAtLastSuccessfulCheck(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	j := f.state(t)
	dir := filepath.Join(f.s.Feed.Dir(), ".live")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(9 * time.Minute)
	live := session.Live{Lease: j.ID, Holder: f.worker.Worker, Fence: uint64(f.worker.Fence), Attempt: 1, PID: 123, PIDStart: "example", Until: f.now.Add(21 * time.Minute)}
	if err := writeJSON(filepath.Join(dir, fmt.Sprintf("%s.%d.json", live.Holder, live.Fence)), live); err != nil {
		t.Fatal(err)
	}
	f.s.Probe = func(session.Live) session.ProcessState { return session.Alive }
	f.tick(t)
	f.s.Probe = func(session.Live) session.ProcessState { return session.Unknown }
	for _, step := range []time.Duration{19 * time.Minute, time.Minute} {
		f.now = f.now.Add(step)
		f.tick(t)
		if f.state(t).Status != contract.Running {
			t.Fatal("staled within grace or before until")
		}
	}
	f.now = f.now.Add(time.Minute)
	f.tick(t)
	if f.state(t).Reason != "host_offline" {
		t.Fatal("last successful check never expired")
	}
}

func TestWorkerWriteWaitsForTaskLeaseBeforePassLock(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	taskPath := filepath.Join(f.s.Feed.Dir(), "lease", "task.json")
	task, err := fslock.OpenNoFollow(taskPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer task.Close()
	if err := fslock.LockExclusive(task); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			fslock.Unlock(task)
		}
	}()
	// The hook says a lock call on task.json is about to wait; it only compares
	// the path and sends without blocking.
	queued := make(chan struct{}, 1)
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if path != taskPath {
			return
		}
		select {
		case queued <- struct{}{}:
		default:
		}
	}))
	done := make(chan error, 1)
	go func() {
		_, err := Write(f.s.Feed, f.worker, feed.Input{Type: "result", Job: "J-1", Attempt: 1, Body: resultBody()})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("worker wrote while task lease was exclusively held: %v", err)
	case <-queued:
	case <-time.After(ifLwDeadline):
		t.Fatal("timed out waiting for the writer to queue on task.json")
	}
	// The queued writer must leave the pass lock free while waiting for task.json.
	pass, err := fslock.OpenNoFollow(filepath.Join(f.s.Feed.Dir(), ".orchestrator.lock"), os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer pass.Close()
	acquired := make(chan error, 1)
	go func() { acquired <- fslock.LockExclusive(pass) }()
	select {
	case err := <-acquired:
		if err != nil {
			t.Fatal(err)
		}
		fslock.Unlock(pass)
	case <-time.After(time.Second):
		fslock.Unlock(task)
		locked = false
		<-acquired
		fslock.Unlock(pass)
		<-done
		t.Fatal("writer took the pass lock before task.json")
	}
	fslock.Unlock(task)
	locked = false
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestAppliedRetrySurvivesMetadataLoss(t *testing.T) {
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
	for id := range state.Questions {
		f.post(t, feed.Input{Type: "answer", Re: id, Body: map[string]any{"answer": "Retry"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
	}
	if f.tick(t).Assigned != 1 {
		t.Fatal("retry not applied")
	}
	if err := os.Remove(filepath.Join(f.s.Feed.Dir(), ".orchestrator.json")); err != nil {
		t.Fatal(err)
	}
	f.ack(t)
	f.exit(t, "crash")
	f.tick(t)
	if f.state(t).Status == contract.Parked {
		t.Fatal("recovery forgot applied retry and restored old crash count")
	}
}

func TestLegacyWakeMigrationPreservesAcknowledgementsAndNumbers(t *testing.T) {
	f := setup(t)
	snap := Snapshot{State: contract.State{Jobs: map[string]*contract.Job{}, Wakes: []string{"event-1", "event-2", "event-3"}}, Records: map[string]feed.Record{}}
	m := memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{"event-1": true, "event-2": true, "event-3": true}}
	for _, event := range snap.State.Wakes {
		snap.Records[event] = feed.Record{Body: map[string]any{"job": "J-1"}}
	}
	for _, event := range []string{"event-1", "event-2"} {
		key := event + "/1"
		snap.Records["ack-"+event] = feed.Record{Wake: &key}
	}
	out := f.s.wake(snap, &m)
	if out == nil || out.Event != "event-3" || out.N <= 3 {
		t.Fatal("migration replayed acknowledged event or reused legacy render number", out)
	}
	if again := f.s.wake(snap, &m); !reflect.DeepEqual(out, again) {
		t.Fatal("migrated pending wake was lost", again)
	}
}

// An agent's answer must not release a person gate on the Orchestrator path,
// nor make every pass post a selection control for it.
func TestAgentAnswerKeepsPersonGateClosed(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", []any{"gate:Q-1"})
	q := f.post(t, feed.Input{Type: "question", Body: map[string]any{"question": "Ship?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": contract.FormatTime(f.now.Add(time.Hour))}}}, f.s.Writer)
	f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Yes"}}, feed.Writer{Author: feed.Author{ID: "agent-1", Role: "agent"}, Worker: "agent-1@" + machine})
	for range 2 {
		if out := f.tick(t); out.Assigned != 0 {
			t.Fatalf("agent answer assigned J-1: %+v", out.Rows)
		}
	}
	if f.state(t).Assign != nil {
		t.Fatal("agent answer opened the person gate")
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if !snap.State.Questions[q.ID].Open || snap.State.Questions[q.ID].Applied != nil {
		t.Fatalf("orchestrator fold closed the gate: %+v", snap.State.Questions[q.ID])
	}
	controls, err := f.s.Feed.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range controls.Records {
		if c.Body["target"] == q.ID && c.Body["status"] == "answered" {
			t.Fatalf("orchestrator selected a non-person answer: %+v", c.Record)
		}
	}
	f.post(t, feed.Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Yes"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
	if f.tick(t).Assigned != 1 {
		t.Fatal("person answer did not open the gate")
	}
}
