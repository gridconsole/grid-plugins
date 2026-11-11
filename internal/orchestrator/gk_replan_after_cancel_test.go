package orchestrator

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// gkRig posts J-1 (no after) and one Job per entry of afters (J-2, J-3, ...),
// ticks so J-1 is assigned, acks it and posts the worker-exit cancelled. It does
// not tick after the exit.
func gkRig(t *testing.T, afters ...[]any) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	for i, after := range afters {
		f.job(t, fmt.Sprintf("J-%d", i+2), after)
	}
	f.tick(t)
	f.ack(t)
	f.exit(t, "cancelled")
	return f
}

// gkReplans returns the job items that name a Job they were discovered from,
// with the body the Feed holds, and every Job id on the Feed.
func gkReplans(t *testing.T, f *fixture) (replans map[string]map[string]any, all []string) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	replans = map[string]map[string]any{}
	for _, it := range snap.Items {
		all = append(all, it.ID)
		if it.DiscoveredFrom != nil {
			replans[it.ID] = snap.Bodies[it.ID]
		}
	}
	return replans, all
}

// gkStrings calls fn on every string inside a decoded JSON value.
func gkStrings(v any, fn func(string)) {
	switch x := v.(type) {
	case string:
		fn(x)
	case []any:
		for _, e := range x {
			gkStrings(e, fn)
		}
	case map[string]any:
		for _, e := range x {
			gkStrings(e, fn)
		}
	}
}

func gkStatus(t *testing.T, f *fixture, id string) contract.Status {
	t.Helper()
	st, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Jobs[id] == nil {
		t.Fatalf("%s is not on the Feed", id)
	}
	return st.Jobs[id].Status
}

func TestGkReplanAfterCancel(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	f.tick(t)
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if len(replans) != 1 || replans["J-3"] == nil {
		t.Fatalf("want one re-plan Job J-3, got %v", replans)
	}
	body := replans["J-3"]
	if body["role"] != "orchestrator" || body["discovered_from"] != "J-1" {
		t.Fatalf("role %v, discovered_from %v", body["role"], body["discovered_from"])
	}
	if !reflect.DeepEqual(body["skills"], []any{"plan-jobs"}) {
		t.Fatalf("skills = %v", body["skills"])
	}
	if links, ok := body["links"].([]any); !ok || len(links) != 0 {
		t.Fatalf("links = %v, want empty", body["links"])
	}
	if _, has := body["review"]; has {
		t.Fatalf("body has a review key: %v", body["review"])
	}
	if v, _ := body["verify"].(map[string]any); v["path"] != "task:task.md" {
		t.Fatalf("verify = %v", body["verify"])
	}
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	if a1["id"] != "A1" || !strings.Contains(text, "J-2") || !strings.Contains(text, "waiting for J-1, which was cancelled") {
		t.Fatalf("A1 = %v", a1)
	}
	gkStrings(body, func(s string) {
		if strings.HasPrefix(s, "job:") {
			t.Errorf("body holds a job: pointer %q", s)
		}
	})
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the body on the Feed is not a job-record: %v", err)
	}
	// What the loop posts is what ReplanJobBody builds.
	want, err := json.Marshal(ReplanJobBody("J-3", "J-1", "", []string{"J-2"}))
	if err != nil {
		t.Fatal(err)
	}
	var wantBody map[string]any
	if err := json.Unmarshal(want, &wantBody); err != nil {
		t.Fatal(err)
	}
	for k, v := range wantBody {
		if !reflect.DeepEqual(body[k], v) {
			t.Errorf("field %s = %v, want %v", k, body[k], v)
		}
	}
}

func TestGkReplanOnce(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	for i := 0; i < 4; i++ {
		f.tick(t)
	}
	replans, all := gkReplans(t, f)
	if len(replans) != 1 || replans["J-3"] == nil || replans["J-3"]["discovered_from"] != "J-1" {
		t.Fatalf("want one re-plan Job J-3 from J-1, got %v (Jobs %v)", replans, all)
	}
	if got := gkStatus(t, f, "J-2"); got == contract.Cancelled {
		t.Fatalf("J-2 is %s: the loop cancelled it", got)
	}
}

func TestGkReplanAfterRestart(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	// The tick that would have written the control is lost: the worker-exit is
	// on the Feed and the control is posted by hand, with no tick in between.
	exited := f.state(t).Exited
	if exited == "" {
		t.Fatal("J-1 has no worker-exit")
	}
	f.post(t, feed.Input{Type: "control", Re: exited, Body: map[string]any{"target": "J-1", "reason": "worker cancelled", "status": contract.Cancelled, "close_reason": "worker cancelled"}}, f.s.Writer)
	if replans, _ := gkReplans(t, f); len(replans) != 0 {
		t.Fatalf("a re-plan Job exists before any tick: %v", replans)
	}
	restarted := f.s // a new process reads the same Feed with a copy of the Store
	if _, err := restarted.Tick(); err != nil {
		t.Fatal(err)
	}
	if replans, all := gkReplans(t, f); len(replans) != 1 || replans["J-3"] == nil {
		t.Fatalf("after one tick want one re-plan Job J-3, got %v (Jobs %v)", replans, all)
	}
	if _, err := restarted.Tick(); err != nil {
		t.Fatal(err)
	}
	if replans, all := gkReplans(t, f); len(replans) != 1 {
		t.Fatalf("after the second tick want still one re-plan Job, got %v (Jobs %v)", replans, all)
	}
}

func TestGkReplanNotForControlCancel(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "reason": "owner cancelled", "status": contract.Cancelled, "close_reason": "owner cancelled"}}, f.s.Writer)
	f.tick(t)
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if got := f.state(t).ExitReason; got != "" {
		t.Fatalf("J-1 has exit reason %q, want none: nothing exited", got)
	}
	if replans, all := gkReplans(t, f); len(replans) != 0 {
		t.Fatalf("a control-only cancel got a re-plan Job: %v (Jobs %v)", replans, all)
	}
}

func TestGkReplanNoWaiter(t *testing.T) {
	f := gkRig(t)
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if replans, all := gkReplans(t, f); len(replans) != 0 || len(all) != 1 {
		t.Fatalf("no Job waits, want no new Job: %v (Jobs %v)", replans, all)
	}
}

func TestGkReplanFailedEdge(t *testing.T) {
	f := gkRig(t, []any{map[string]any{"job": "J-1", "kind": "failed", "why": "fallback"}})
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if replans, all := gkReplans(t, f); len(replans) != 0 || len(all) != 2 {
		t.Fatalf("a failed edge is satisfied by the cancel, want no new Job: %v (Jobs %v)", replans, all)
	}
}

// Two waiters (a bare id and the explicit done kind) are named; a Job that
// waits through a failed edge, and one that waits on another Job, are not.
func TestGkReplanNamesEveryDoneWaiter(t *testing.T) {
	f := gkRig(t,
		[]any{"J-1"},
		[]any{map[string]any{"job": "J-1", "kind": "done", "why": "needs its files"}},
		[]any{map[string]any{"job": "J-1", "kind": "failed", "why": "fallback"}},
		[]any{"J-2"},
	)
	f.tick(t)
	replans, all := gkReplans(t, f)
	if len(replans) != 1 || replans["J-6"] == nil {
		t.Fatalf("want one re-plan Job J-6, got %v (Jobs %v)", replans, all)
	}
	if title := replans["J-6"]["title"]; title != "Re-plan after J-1 was cancelled: J-2, J-3 wait on it" {
		t.Fatalf("title = %v", title)
	}
}
