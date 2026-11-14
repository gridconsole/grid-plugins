package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// jwKhDeadline bounds a wait that the fixed code never reaches its end of: it is
// the failure path, never the proof.
const jwKhDeadline = 30 * time.Second

// jwKhVerdict is the verdict the Feed reader gives the record id, "" when the
// Feed has no such record (copied from iuRkVerdict).
func jwKhVerdict(t *testing.T, store feed.Store, id string) string {
	t.Helper()
	r, err := store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range r.Records {
		if e.ID == id {
			return e.Verdict
		}
	}
	return ""
}

// jwKhJobRecords lists the job records of the Feed as "<job id> <role> <verdict>",
// in Feed order. The verdict is the fold's (counted, held, ignored, ...): the
// reader alone calls a record counted that the fold then ignores as a duplicate id.
func jwKhJobRecords(t *testing.T, store feed.Store) []string {
	t.Helper()
	r, err := store.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	folded := map[string]string{}
	for _, v := range snap.State.Verdicts {
		folded[v.ID] = v.Kind
	}
	var out []string
	for _, e := range r.Records {
		if e.Type != "job" {
			continue
		}
		verdict := folded[e.ID]
		if verdict == "" {
			verdict = e.Verdict
		}
		out = append(out, fmt.Sprintf("%v %v %s", e.Body["id"], e.Body["role"], verdict))
	}
	return out
}

// jwKhIDs is the Job id of each of jobs.
func jwKhIDs(jobs []job.Job) []string {
	var ids []string
	for _, j := range jobs {
		ids = append(ids, j.ID)
	}
	return ids
}

// jwKhDeliverHolding holds <task>/feed/.orchestrator.lock on its own descriptor
// and runs the deliver that posts the keeper Job in a goroutine. Deliver takes no
// lock before the id pick (postCuratorJob's Load takes none; the delivery write
// holds the task and plan locks only), so the first wait on .orchestrator.lock is
// the one in WriteUnless, which follows the pick at deliver_keeper_job.go:125.
// When the deliver has queued on that lock (the fslock.OnBlockForTest event),
// between runs while the lock is still held; then the lock is let go. It returns
// the exit code of the deliver.
func jwKhDeliverHolding(t *testing.T, h *harness, dir, id string, store feed.Store, between func()) int {
	t.Helper()
	lockPath := filepath.Join(store.Dir(), ".orchestrator.lock")
	lock, err := fslock.OpenNoFollow(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	locked := true
	unlock := func() {
		if locked {
			locked = false
			fslock.Unlock(lock)
		}
	}
	defer unlock()

	// The hook only compares the path and sends without blocking.
	queued := make(chan struct{}, 1)
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if path != lockPath {
			return
		}
		select {
		case queued <- struct{}{}:
		default:
		}
	}))
	done := make(chan int, 1)
	go func() { done <- h.deliverRun(dir, "--outcome", "x", "--task", id) }()
	select {
	case code := <-done:
		t.Fatalf("deliver exited %d without waiting on .orchestrator.lock: %s", code, h.errb)
	case <-queued:
	case <-time.After(jwKhDeadline):
		t.Fatal("timed out waiting for deliver to queue on .orchestrator.lock")
	}
	between()
	unlock()
	select {
	case code := <-done:
		return code
	case <-time.After(jwKhDeadline):
		t.Fatal("timed out waiting for deliver to finish after the lock was let go")
	}
	return -1
}

// A held job record (its after target is on no record) planted between the id
// pick and the lock is not in State, but its id is taken: the check under the
// lock sees it, the pick is made again, and the curator Job is J-(n+1) and in State.
func TestJwKhHeldIDPlantedBeforeTheLockIsSkipped(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact", "second-fact")
	missing, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	w, err := orchestratorWriter(h.env, "task", h.env.getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := orchestrator.Load(store)
	if err != nil {
		t.Fatal(err)
	}
	// The id deliver will pick: nothing is on the Feed yet, so J-1.
	n := nextJobID(snap)
	if n != "J-1" {
		t.Fatalf("the id deliver picks is %s, want J-1", n)
	}
	var held feed.Record
	code := jwKhDeliverHolding(t, h, dir, id, store, func() {
		body := curatorJobBody(n, id, []string{"first-fact"})
		body["role"] = "builder"
		held, err = store.Post(feed.Input{Type: "job", Job: n, Body: body, After: []string{missing}}, w)
		if err != nil {
			t.Fatal(err)
		}
		if snap, err := orchestrator.Load(store); err != nil {
			t.Fatal(err)
		} else if v := jwKhVerdict(t, store, held.ID); snap.State.Jobs[n] != nil || v != "held" {
			t.Fatalf("the %s record is not held: job %v, verdict %q", n, snap.State.Jobs[n], v)
		}
	})
	if code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}

	// While the planted record is held: the curator Job is J-2 and in State.
	jobs, snap := curatorJobs(t, store)
	records := jwKhJobRecords(t, store)
	if ids := jwKhIDs(jobs); strings.Join(ids, ",") != "J-2" {
		t.Errorf("held record still held: want one curator Job J-2 in State, got %q; job records (id role verdict): %q; stdout %q", ids, records, h.out.String())
	}
	if !strings.Contains(h.out.String(), "(Job J-2)") {
		t.Errorf("count line does not name J-2: %q", h.out.String())
	}
	if snap.State.Jobs[n] != nil || snap.State.Jobs["J-2"] == nil {
		t.Errorf("held record still held: State holds %s = %v, J-2 = %v", n, snap.State.Jobs[n], snap.State.Jobs["J-2"])
	}
	if v := jwKhVerdict(t, store, held.ID); v != "held" {
		t.Errorf("verdict of the planted %s record = %q, want held", n, v)
	}
	// J-3 is the person Job of the ship step, posted after the curator Job.
	if want := []string{"J-1 builder held", "J-2 curator counted", "J-3 Person counted"}; strings.Join(records, "|") != strings.Join(want, "|") {
		t.Errorf("held record still held: job records = %q, want %q", records, want)
	}

	// The record the planted one waits for arrives. The fold reads the planted
	// record first (first record wins), so a curator record on the same id would
	// now be ignored and the keeper would never run; on J-2 it is not.
	if _, err := store.Post(feed.Input{ID: missing, Type: "handover", Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	jobs, snap = curatorJobs(t, store)
	records = jwKhJobRecords(t, store)
	if ids := jwKhIDs(jobs); strings.Join(ids, ",") != "J-2" {
		t.Errorf("held record released: want one curator Job J-2 in State, got %q; job records (id role verdict): %q", ids, records)
	}
	if j := snap.State.Jobs[n]; j == nil || j.Role != "builder" {
		t.Errorf("held record released: State holds %s = %v, want the planted builder Job", n, j)
	}
	if j := snap.State.Jobs["J-2"]; j == nil || j.Role != "curator" {
		t.Errorf("held record released: State holds J-2 = %v, want the curator Job", j)
	}
	if want := []string{"J-1 builder counted", "J-2 curator counted", "J-3 Person counted"}; strings.Join(records, "|") != strings.Join(want, "|") {
		t.Errorf("held record released: job records = %q, want %q", records, want)
	}
}

// With nothing planted the same route, lock held and let go, gives the curator
// Job J-1 as before: the check under the lock does not turn a free id away.
func TestJwKhNothingPlantedTakesTheFirstID(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "first-fact", "second-fact")
	code := jwKhDeliverHolding(t, h, dir, id, store, func() {})
	if code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	jobs, snap := curatorJobs(t, store)
	records := jwKhJobRecords(t, store)
	if len(jobs) != 1 || jobs[0].ID != "J-1" {
		t.Fatalf("want one curator Job J-1, got %+v; job records (id role verdict): %q; stdout %q", jobs, records, h.out.String())
	}
	if !strings.Contains(h.out.String(), "(Job J-1)") {
		t.Fatalf("count line does not name J-1: %q", h.out.String())
	}
	if snap.State.Jobs["J-1"] == nil {
		t.Fatal("State does not hold J-1")
	}
	// J-2 is the person Job of the ship step, posted after the curator Job.
	if want := []string{"J-1 curator counted", "J-2 Person counted"}; strings.Join(records, "|") != strings.Join(want, "|") {
		t.Fatalf("job records = %q, want %q", records, want)
	}
}
