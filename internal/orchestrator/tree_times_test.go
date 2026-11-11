package orchestrator

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// timesRow reads the Tree row of id the way `grid job tree --json` prints it.
func timesRow(t *testing.T, f *fixture, id string) map[string]any {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range TreeAt(snap, time.Now()) {
		if r.ID != id {
			continue
		}
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(b, &row); err != nil {
			t.Fatal(err)
		}
		return row
	}
	t.Fatalf("no row %s", id)
	return nil
}

// recordTimes returns, for one Job, the At of its post, and of each assign, ack
// and worker-exit record in order, read from the Feed and not from the clock.
func recordTimes(t *testing.T, f *fixture, id string) (post string, assign, ack, exit []string) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	at := func(s string) string {
		p, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t.Fatal(err)
		}
		return contract.FormatTime(p)
	}
	for _, event := range snap.State.Jobs[id].History {
		r := snap.Records[event]
		if _, ok := r.Body["assign"].(map[string]any); ok {
			assign = append(assign, at(r.At))
			continue
		}
		switch r.Type {
		case "job":
			post = at(r.At)
		case "ack":
			ack = append(ack, at(r.At))
		case "worker-exit":
			exit = append(exit, at(r.At))
		}
	}
	return
}

func attemptsOf(t *testing.T, row map[string]any) []map[string]any {
	t.Helper()
	list, _ := row["attempts"].([]any)
	out := []map[string]any{}
	for _, a := range list {
		out = append(out, a.(map[string]any))
	}
	return out
}

func seconds(t *testing.T, from, to string) float64 {
	t.Helper()
	a, err1 := time.Parse(time.RFC3339Nano, from)
	b, err2 := time.Parse(time.RFC3339Nano, to)
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	return b.Sub(a).Seconds()
}

// Row 1 and 2: J-1 posted, assigned and acked; the row carries the ready time,
// the attempt's assign and ack times, and the pickup latency between them.
func TestTreeCarriesPickupTimes(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	post, assign, ack, _ := recordTimes(t, f, "J-1")
	if len(assign) != 1 || len(ack) != 1 {
		t.Fatalf("assign %v ack %v", assign, ack)
	}
	row := timesRow(t, f, "J-1")
	a := attemptsOf(t, row)
	if len(a) != 1 || a[0]["assigned_at"] != assign[0] || a[0]["acked_at"] != ack[0] {
		t.Fatalf("want assigned_at %s acked_at %s, got row %v", assign[0], ack[0], row)
	}
	if _, has := a[0]["exited_at"]; has {
		t.Fatalf("no worker-exit yet, exited_at must be absent: %v", a[0])
	}
	if row["ready_at"] != post {
		t.Fatalf("no after: ready_at is the post time %s, got %v", post, row["ready_at"])
	}
	if got, _ := row["pickup_seconds"].(float64); row["pickup_seconds"] == nil || got != seconds(t, post, ack[0]) {
		t.Fatalf("pickup_seconds want %v, got %v", seconds(t, post, ack[0]), row["pickup_seconds"])
	}
	f.exit(t, "done")
	_, _, _, exit := recordTimes(t, f, "J-1")
	if got := attemptsOf(t, timesRow(t, f, "J-1"))[0]["exited_at"]; len(exit) != 1 || got != exit[0] {
		t.Fatalf("want exited_at %v, got %v", exit, got)
	}
}

// Row 3: a Job after J-1 is ready from the record that closed J-1.
func TestTreeReadyAtIsTheAfterClose(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.tick(t)
	if r := timesRow(t, f, "J-2"); r["ready_at"] != nil || r["pickup_seconds"] != nil {
		t.Fatalf("J-2 waits on J-1 and is not ready: %v", r)
	}
	time.Sleep(20 * time.Millisecond)
	if err := f.s.control(f.state(t), contract.Accepted, "accepted", ""); err != nil {
		t.Fatal(err)
	}
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	h := snap.State.Jobs["J-1"].History
	closed, _ := time.Parse(time.RFC3339Nano, snap.Records[h[len(h)-1]].At)
	post2, _, _, _ := recordTimes(t, f, "J-2")
	row := timesRow(t, f, "J-2")
	if row["ready_at"] != contract.FormatTime(closed) || row["ready_at"] == post2 {
		t.Fatalf("want ready_at %s (not post %s), got %v", contract.FormatTime(closed), post2, row["ready_at"])
	}
	// J-2 is assigned to the worker and acked: latency counts from the close.
	f.tick(t)
	snap, _ = Load(f.s.Feed)
	j := snap.State.Jobs["J-2"]
	f.worker.Lease, f.worker.Fence, f.worker.Gen = "J-2", j.Assign.Fence, j.Generation
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "ack", Job: "J-2", Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	_, _, ack, _ := recordTimes(t, f, "J-2")
	row = timesRow(t, f, "J-2")
	if got, _ := row["pickup_seconds"].(float64); row["pickup_seconds"] == nil || got != seconds(t, contract.FormatTime(closed), ack[0]) {
		t.Fatalf("pickup_seconds want %v from the close, got %v", seconds(t, contract.FormatTime(closed), ack[0]), row["pickup_seconds"])
	}
}

// Row 4: a retried Job keeps each attempt's own times; pickup_seconds is the
// latest attempt's ack against the original ready time.
func TestTreeRetriedJobTimes(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	f.exit(t, "crash")
	f.now = f.now.Add(time.Minute)
	f.tick(t)
	f.ack(t)
	post, assign, ack, exit := recordTimes(t, f, "J-1")
	if len(assign) != 2 || len(ack) != 2 || len(exit) != 1 {
		t.Fatalf("assign %v ack %v exit %v", assign, ack, exit)
	}
	row := timesRow(t, f, "J-1")
	a := attemptsOf(t, row)
	if len(a) != 2 || a[0]["assigned_at"] != assign[0] || a[0]["acked_at"] != ack[0] || a[0]["exited_at"] != exit[0] || a[1]["assigned_at"] != assign[1] || a[1]["acked_at"] != ack[1] {
		t.Fatalf("attempts %v", a)
	}
	if _, has := a[1]["exited_at"]; has {
		t.Fatalf("attempt 2 has not exited: %v", a[1])
	}
	if got, _ := row["pickup_seconds"].(float64); row["ready_at"] != post || got != seconds(t, post, ack[1]) {
		t.Fatalf("want ready_at %s pickup %v, got %v %v", post, seconds(t, post, ack[1]), row["ready_at"], row["pickup_seconds"])
	}
}

// Row 5: a person's Job has a ready time and no pickup; a Job never acked has
// none either.
func TestTreePersonAndUnackedJobsHaveNoPickup(t *testing.T) {
	f := setup(t)
	person := jobBody(t, "J-4")
	person["role"] = "Person"
	person["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	f.post(t, feed.Input{Type: "job", Body: person, Job: "J-4"}, f.s.Writer)
	f.job(t, "J-1", nil)
	f.tick(t) // J-1 assigned, not acked
	p := timesRow(t, f, "J-4")
	post4, _, _, _ := recordTimes(t, f, "J-4")
	if p["ready_at"] != post4 || p["pickup_seconds"] != nil || len(attemptsOf(t, p)) != 0 {
		t.Fatalf("person Job: want ready_at %s and no pickup, got %v", post4, p)
	}
	row := timesRow(t, f, "J-1")
	a := attemptsOf(t, row)
	if row["ready_at"] == nil || row["pickup_seconds"] != nil || len(a) != 1 || a[0]["assigned_at"] == nil || a[0]["acked_at"] != nil {
		t.Fatalf("assigned and not acked: %v", row)
	}
}
