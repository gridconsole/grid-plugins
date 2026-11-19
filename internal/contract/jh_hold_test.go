package contract

import (
	"encoding/json"
	"reflect"
	"testing"
)

// jhPermissionBlocked is J-1's first attempt with a row waiting on a permission:
// the attempt posts blocked, then its result with the waiting row naming that
// blocked post, then exits. It returns the posts and the blocked post.
func jhPermissionBlocked(t *testing.T) ([]Post, Post) {
	t.Helper()
	blocked := plBd2Loop(t, "blocked", 8, 1)
	result := jhResult(t, 6,
		map[string]any{"id": "A1", "state": "met"},
		map[string]any{"id": "A3", "state": "waiting", "waits_on": map[string]any{"kind": WaitPermission, "ref": blocked.ID}},
	)
	return append(arJob(t), result, blocked, plBd2Exit(t, 9, 1, "done")), blocked
}

// A result row that waits keeps its waits_on in the fold; a row that does not wait
// has none, and a later mark of the row as met or not met drops it.
func TestHoldWaitingRowKeepsWaitsOn(t *testing.T) {
	result := jhResult(t, 6,
		map[string]any{"id": "A1", "state": "met"},
		map[string]any{"id": "A2", "state": "waiting", "waits_on": map[string]any{"kind": WaitLanded, "ref": "J-6"}},
	)
	rows := Fold(append(arJob(t), result)).Jobs["J-1"].Acceptance
	if rows[0].WaitsOn != nil {
		t.Fatalf("a met row carries waits_on %+v", rows[0].WaitsOn)
	}
	if want := (&Wait{Kind: WaitLanded, Ref: "J-6"}); rows[1].State != RowWaiting || !reflect.DeepEqual(rows[1].WaitsOn, want) {
		t.Fatalf("waiting row = %+v, want state waiting waits_on %+v", rows[1], want)
	}
	for _, state := range []string{"met", "not-met"} {
		marked := Fold(append(arJob(t), result, arMark(t, 7, "A2", state, "")))
		if row := marked.Jobs["J-1"].Acceptance[1]; row.WaitsOn != nil {
			t.Fatalf("row marked %s still carries waits_on %+v", state, row.WaitsOn)
		}
	}
}

// A Job whose attempt posted blocked and exited, with a result whose row waits on
// the permission, folds Blocked (a hold with no review and no rework) and names
// the wait, which grid feed state shows with its kind and ref.
func TestHoldBlockedJobNamesItsWaits(t *testing.T) {
	posts, blocked := jhPermissionBlocked(t)
	s := Fold(posts)
	j := s.Jobs["J-1"]
	if j.Status != Blocked || j.State != StateStarted || j.StateReason != "blocked" {
		t.Fatalf("Job = %s %q %q, want blocked, Started, blocked", j.Status, j.State, j.StateReason)
	}
	if want := []HeldWait{{Row: "A3", Kind: WaitPermission, Ref: blocked.ID}}; !reflect.DeepEqual(j.WaitsOn, want) {
		t.Fatalf("WaitsOn = %+v, want %+v", j.WaitsOn, want)
	}
	if err := jhValid(t, "cmd-feed-state", s); err != nil {
		t.Fatalf("feed state with a held wait: %v", err)
	}
	data, _ := json.Marshal(s)
	var out struct {
		Jobs map[string]struct {
			Status  string     `json:"status"`
			WaitsOn []HeldWait `json:"waits_on"`
		}
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if got := out.Jobs["J-1"].WaitsOn; len(got) != 1 || got[0].Kind != WaitPermission || got[0].Ref != blocked.ID {
		t.Fatalf("feed state shows waits_on %+v, want kind %s ref %s", got, WaitPermission, blocked.ID)
	}
}

// A blocked Job with no waiting row names no wait, and a new assign of the Job
// clears the waits of the attempt that blocked.
func TestHoldBlockedWaitsEndWithTheAttempt(t *testing.T) {
	plain := Fold(append(base(t), plBd2Loop(t, "blocked", 6, 1), plBd2Exit(t, 7, 1, "done")))
	if j := plain.Jobs["J-1"]; j.Status != Blocked || len(j.WaitsOn) != 0 {
		t.Fatalf("Job = %s waits %+v, want blocked with no waits", j.Status, j.WaitsOn)
	}
	posts, _ := jhPermissionBlocked(t)
	a, k := retry(t, 20, 2, 3)
	j := Fold(append(posts, a, k)).Jobs["J-1"]
	if j.Attempt != 2 || j.Status == Blocked || len(j.WaitsOn) != 0 {
		t.Fatalf("after a new assign Job = attempt %d %s waits %+v, want attempt 2, not blocked, no waits", j.Attempt, j.Status, j.WaitsOn)
	}
}

// The Held status reads as Started with the reason held, as Blocked does, so a
// held Job is open and in progress, not Needs review and not Done; its feed state
// carries the status and the waits with their kind and ref and validates.
func TestHoldHeldReadsStartedAndShowsItsWait(t *testing.T) {
	if state, reason := StateOf(Held); state != StateStarted || reason != "held" {
		t.Fatalf("StateOf(Held) = %q, %q, want Started, held", state, reason)
	}
	s := Fold(arJob(t))
	j := s.Jobs["J-1"]
	j.Status, j.WaitsOn = Held, []HeldWait{{Row: "A2", Kind: WaitLanded, Ref: "J-6"}}
	j.State, j.StateReason = StateOf(j.Status)
	if err := jhValid(t, "cmd-feed-state", s); err != nil {
		t.Fatalf("feed state of a held Job: %v", err)
	}
	data, _ := json.Marshal(j)
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	waits, _ := out["waits_on"].([]any)
	if out["status"] != "held" || out["state"] != StateStarted || out["state_reason"] != "held" || len(waits) != 1 {
		t.Fatalf("held Job JSON = %s", data)
	}
	if w := waits[0].(map[string]any); w["row"] != "A2" || w["kind"] != WaitLanded || w["ref"] != "J-6" || w["met"] != nil {
		t.Fatalf("wait JSON = %v", w)
	}
}

// A control that holds a Job sets its waits; one with no status (the tick's
// reason-only control) keeps them; one that moves the Job to another status
// clears them. A hold is never counted, capped or timed out here.
func TestHoldControlSetsKeepsAndClearsWaits(t *testing.T) {
	waits := []HeldWait{{Row: "A2", Kind: WaitJob, Ref: "J-6"}, {Row: "A3", Kind: WaitQuestion, Ref: "Q-2"}}
	j := &Job{ID: "J-1", Status: Submitted}
	applyHold(j, controlBody{Status: Held, WaitsOn: waits})
	if !reflect.DeepEqual(j.WaitsOn, waits) {
		t.Fatalf("held control left waits %+v, want %+v", j.WaitsOn, waits)
	}
	applyHold(j, controlBody{Reason: "waiting for J-6 to be accepted"})
	if !reflect.DeepEqual(j.WaitsOn, waits) {
		t.Fatalf("a reason-only control changed the waits to %+v", j.WaitsOn)
	}
	// Held again with other waits, any number of times: the latest control wins.
	for i := 0; i < 5; i++ {
		applyHold(j, controlBody{Status: Held, WaitsOn: waits[:1]})
	}
	if !reflect.DeepEqual(j.WaitsOn, waits[:1]) {
		t.Fatalf("a second hold left waits %+v, want %+v", j.WaitsOn, waits[:1])
	}
	applyHold(j, controlBody{Status: Ready, Reason: "hold released"})
	if len(j.WaitsOn) != 0 {
		t.Fatalf("a control to ready left waits %+v", j.WaitsOn)
	}
}
