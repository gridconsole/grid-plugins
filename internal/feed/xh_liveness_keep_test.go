package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// xhAssigned posts Job id, assigns it to a worker lease and acks it, so the Job
// reads running with an assign_post.
func xhAssigned(t *testing.T, s Store, task Writer, id string) Writer {
	t.Helper()
	foldJob(t, s, task, id)
	worker := Writer{Author: Author{ID: "worker", Role: "worker"}, Worker: "worker@" + testMachine, Lease: id, Fence: 2}
	n := 1
	testLease(t, s, worker, &n)
	assign := mustPost(t, s, Input{Type: "control", Body: map[string]any{"assign": map[string]any{"job": id, "attempt": 1, "to": worker.Worker, "fence": 2, "until": "2000-01-01T00:00:00.000Z"}}}, task)
	mustPost(t, s, Input{Type: "ack", Job: id, Attempt: 1, After: []string{assign.ID}, Body: map[string]any{}}, worker)
	return worker
}

// xhStateJobs decodes the Jobs of state.json as written, liveness included.
func xhStateJobs(t *testing.T, s Store) (map[string]map[string]any, []byte) {
	t.Helper()
	var view struct {
		Jobs map[string]map[string]any `json:"jobs"`
	}
	raw := readFoldFile(t, s, "state.json", &view)
	return view.Jobs, raw
}

func xhWantLiveness(t *testing.T, s Store, id, want string) {
	t.Helper()
	jobs, raw := xhStateJobs(t, s)
	got, ok := jobs[id]["liveness"]
	if want == "" {
		if ok {
			t.Fatalf("%s kept liveness %v in state.json: %s", id, got, raw)
		}
		return
	}
	if got != want {
		t.Fatalf("%s liveness %v, want %q in state.json: %s", id, got, want, raw)
	}
}

// Row 3: a save with no overlay (SaveState, the path every feed state call and
// post refold takes) keeps the liveness the tick wrote for a Job that is unchanged.
func TestXhSaveStateKeepsLiveness(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	xhAssigned(t, s, task, "J-1")
	xhAssigned(t, s, task, "J-2")

	if _, err := s.SaveStateLiveness(map[string]string{"J-2": "alive"}); err != nil {
		t.Fatal(err)
	}
	xhWantLiveness(t, s, "J-2", "alive")

	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	jobs, raw := xhStateJobs(t, s)
	if jobs["J-2"]["liveness"] != "alive" || !strings.Contains(string(raw), `"liveness":"alive"`) {
		t.Fatalf("J-2 lost its liveness after SaveState: %s", raw)
	}
	if _, ok := jobs["J-1"]["liveness"]; ok {
		t.Fatalf("J-1 got a liveness value it was never given: %s", raw)
	}
	if strings.Count(string(raw), `"liveness"`) != 1 {
		t.Fatalf("liveness must appear once in state.json: %s", raw)
	}
	// A second save, and a post that does not change the Job, keep it too.
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	xhWantLiveness(t, s, "J-2", "alive")
}

// The tick passes its own map every pass. An empty map from the tick says no
// holder is observed, so it replaces what was there as it always did.
func TestXhEmptyOverlayStillReplaces(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	xhAssigned(t, s, task, "J-2")
	if _, err := s.SaveStateLiveness(map[string]string{"J-2": "alive"}); err != nil {
		t.Fatal(err)
	}
	for name, empty := range map[string]map[string]string{"nil": nil, "empty": {}} {
		if _, err := s.SaveStateLiveness(map[string]string{"J-2": "alive"}); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveStateLiveness(empty); err != nil {
			t.Fatal(err)
		}
		if _, raw := xhStateJobs(t, s); strings.Contains(string(raw), "liveness") {
			t.Fatalf("%s overlay kept liveness: %s", name, raw)
		}
	}
	// A new non-empty overlay replaces the value.
	if _, err := s.SaveStateLiveness(map[string]string{"J-2": "offline"}); err != nil {
		t.Fatal(err)
	}
	xhWantLiveness(t, s, "J-2", "offline")
}

// Row 4: a Job whose status changed since the overlay was written loses it. The
// rest keep theirs, and neither State() nor needs-you.json carries any of it.
func TestXhLivenessDropsChangedJob(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	worker := xhAssigned(t, s, task, "J-2")
	xhAssigned(t, s, task, "J-1")

	if _, err := s.SaveStateLiveness(map[string]string{"J-1": "alive", "J-2": "alive"}); err != nil {
		t.Fatal(err)
	}
	before, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	if before.Jobs["J-2"].Status != "running" {
		t.Fatalf("J-2 starts %q, want running", before.Jobs["J-2"].Status)
	}

	// A result alone leaves the Job running (contract fold: a result appends to
	// the Job's results and moves no status), so its liveness stays.
	mustPost(t, s, Input{Type: "result", Job: "J-2", Attempt: 1, Body: contractBody(t, "result", map[string]any{"outcome": "Done"})}, worker)
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	xhWantLiveness(t, s, "J-2", "alive")

	// The control that accepts it changes the status, so the old liveness goes.
	mustPost(t, s, Input{Type: "control", Body: map[string]any{"target": "J-2", "status": "accepted", "reason": "Reviewed"}}, task)
	state, err := s.SaveState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Jobs["J-2"].Status != "accepted" {
		t.Fatalf("J-2 status %q, want accepted", state.Jobs["J-2"].Status)
	}
	xhWantLiveness(t, s, "J-2", "")
	xhWantLiveness(t, s, "J-1", "alive")

	plain, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	if mustJSON(t, plain) != mustJSON(t, state) || strings.Contains(mustJSON(t, state), "liveness") {
		t.Fatalf("overlay changed the fold:\n%s\n%s", mustJSON(t, plain), mustJSON(t, state))
	}
	var needs []string
	readFoldFile(t, s, "needs-you.json", &needs)
	if !reflect.DeepEqual(needs, plain.NeedsYou) {
		t.Fatalf("needs-you.json %v, State().NeedsYou %v", needs, plain.NeedsYou)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir(), "needs-you.json"))
	if err != nil || strings.Contains(string(data), "liveness") {
		t.Fatalf("needs-you.json carries liveness: %q %v", data, err)
	}
}

// A Job whose assign_post differs from the one the overlay was written for is a
// new assignment: the old liveness describes another holder and is dropped.
func TestXhLivenessDropsNewAssignPost(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	xhAssigned(t, s, task, "J-1")
	xhAssigned(t, s, task, "J-2")
	if _, err := s.SaveStateLiveness(map[string]string{"J-1": "alive", "J-2": "alive"}); err != nil {
		t.Fatal(err)
	}
	var view map[string]any
	readFoldFile(t, s, "state.json", &view)
	view["jobs"].(map[string]any)["J-2"].(map[string]any)["assign_post"] = "an-earlier-assign"
	data, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "state.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	xhWantLiveness(t, s, "J-2", "")
	xhWantLiveness(t, s, "J-1", "alive")
}

// An old state.json that is missing or does not decode keeps nothing and is no error.
func TestXhSaveStateIgnoresUnusableOldState(t *testing.T) {
	s := testStore(t)
	task := taskWriter(t, s)
	xhAssigned(t, s, task, "J-2")
	if _, err := s.SaveState(); err != nil {
		t.Fatalf("no state.json yet: %v", err)
	}
	for name, content := range map[string]string{"garbage": "not json", "no jobs": `{"jobs":null}`, "jobs not an object": `{"jobs":[1]}`} {
		if err := os.WriteFile(filepath.Join(s.Dir(), "state.json"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.SaveState(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		xhWantLiveness(t, s, "J-2", "")
	}
}
