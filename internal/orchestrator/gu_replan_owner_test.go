package orchestrator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// guDpAck is f.ack for the Job id: the worker holds the lease of that Job.
func guDpAck(t *testing.T, f *fixture, id string) {
	t.Helper()
	st, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs[id]
	if j == nil || j.Assign == nil {
		t.Fatalf("%s is not assigned: %+v", id, j)
	}
	w := f.worker
	w.Lease, w.Fence, w.Gen = id, j.Assign.Fence, j.Generation
	if _, err := Write(f.s.Feed, w, feed.Input{Type: "ack", Job: id, Attempt: j.Attempt, Body: map[string]any{}}); err != nil {
		t.Fatal(err)
	}
}

// guDpExit is f.exit for the Job id: the worker holds the lease of that Job.
func guDpExit(t *testing.T, f *fixture, id, reason string) {
	t.Helper()
	st, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	j := st.Jobs[id]
	if j == nil || j.Assign == nil {
		t.Fatalf("%s is not assigned: %+v", id, j)
	}
	w := f.worker
	w.Lease, w.Fence, w.Gen = id, j.Assign.Fence, j.Generation
	body := map[string]any{}
	if reason != "" {
		body["reason"] = reason
	}
	if _, err := Write(f.s.Feed, w, feed.Input{Type: "worker-exit", Job: id, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
}

// guDpClose closes a Job with a control, the way `grid job post --type control`
// does by hand.
func guDpClose(t *testing.T, f *fixture, id string, status contract.Status) {
	t.Helper()
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": id, "status": status, "reason": "r", "close_reason": "r"}}, f.s.Writer)
}

// guDpPeople returns the bodies of the person Jobs on the Feed, by id, and every
// Job id on the Feed.
func guDpPeople(t *testing.T, f *fixture) (people map[string]map[string]any, all []string) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	people = map[string]map[string]any{}
	for _, it := range snap.Items {
		all = append(all, it.ID)
		if it.IsPerson() {
			people[it.ID] = snap.Bodies[it.ID]
		}
	}
	return people, all
}

// guDpRig is gkRig with the re-plan Job J-3 on the Feed and assigned: J-1 is
// cancelled by its worker with J-2 waiting on it, and one tick posts J-3 and
// assigns it (the same tick: Ready is computed after the re-plan pass). It
// returns with J-3 acked.
func guDpRig(t *testing.T) *fixture {
	t.Helper()
	f := gkRig(t, []any{"J-1"})
	f.tick(t)
	if replans, all := gkReplans(t, f); len(replans) != 1 || replans["J-3"] == nil {
		t.Fatalf("want one re-plan Job J-3, got %v (Jobs %v)", replans, all)
	}
	guDpAck(t, f, "J-3")
	return f
}

// guDpWantOwnerJob checks the one person Job J-4 the Feed holds, and that its
// body is what ReplanOwnerJobBody builds.
func guDpWantOwnerJob(t *testing.T, f *fixture, replanStatus, deadStatus contract.Status, dead string) {
	t.Helper()
	people, all := guDpPeople(t, f)
	if len(people) != 1 || people["J-4"] == nil {
		t.Fatalf("want one person Job J-4, got %v (Jobs %v)", people, all)
	}
	body := people["J-4"]
	if body["role"] != "Person" || body["kind"] != "ask" || body["discovered_from"] != "J-3" {
		t.Fatalf("role %v, kind %v, discovered_from %v", body["role"], body["kind"], body["discovered_from"])
	}
	if intent, _ := body["intent"].(map[string]any); intent["owner"] != "owner" {
		t.Fatalf("intent = %v", body["intent"])
	}
	if skills, ok := body["skills"].([]any); !ok || len(skills) != 0 {
		t.Fatalf("skills = %v", body["skills"])
	}
	if want := "Decide how J-2 continue: re-plan Job J-3 of " + dead + " ended " + string(replanStatus); body["title"] != want {
		t.Fatalf("title = %v, want %q", body["title"], want)
	}
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	for _, want := range []string{dead, "J-3", "J-2 (waiting for " + dead + ", which was " + string(deadStatus) + ")"} {
		if a1["id"] != "A1" || !strings.Contains(text, want) {
			t.Fatalf("A1 = %v, want it to hold %q", a1, want)
		}
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the body on the Feed is not a job-record: %v", err)
	}
	// What the pass posts is what ReplanOwnerJobBody builds.
	want, err := json.Marshal(ReplanOwnerJobBody("J-4", "owner", "J-3", dead, replanStatus, deadStatus, []string{"J-2"}))
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

func TestGuDpWorkerCancelledReplan(t *testing.T) {
	f := guDpRig(t)
	guDpExit(t, f, "J-3", "cancelled")
	f.tick(t)
	if got := gkStatus(t, f, "J-3"); got != contract.Cancelled {
		t.Fatalf("J-3 is %s, want cancelled", got)
	}
	guDpWantOwnerJob(t, f, contract.Cancelled, contract.Cancelled, "J-1")
	if _, all := guDpPeople(t, f); !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3", "J-4"}) {
		t.Fatalf("Jobs on the Feed = %v", all)
	}
}

func TestGuDpControlRejectedReplan(t *testing.T) {
	f := gkRig(t, []any{"J-1"})
	f.tick(t)
	guDpClose(t, f, "J-3", contract.Rejected)
	f.tick(t)
	if got := gkStatus(t, f, "J-3"); got != contract.Rejected {
		t.Fatalf("J-3 is %s, want rejected", got)
	}
	guDpWantOwnerJob(t, f, contract.Rejected, contract.Cancelled, "J-1")
}

func TestGuDpOnce(t *testing.T) {
	f := guDpRig(t)
	guDpExit(t, f, "J-3", "cancelled")
	f.tick(t)
	for i := 0; i < 4; i++ {
		f.tick(t)
	}
	guDpWantOwnerJob(t, f, contract.Cancelled, contract.Cancelled, "J-1")
	if _, all := guDpPeople(t, f); !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3", "J-4"}) {
		t.Fatalf("Jobs on the Feed = %v", all)
	}
}

// No Job waits on the first dead Job any more, or the task has no owner: no
// person Job, and a tick with no owner is no error.
func TestGuDpNoWaiterLeft(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(t *testing.T, f *fixture)
	}{
		{"waiter cancelled first", func(t *testing.T, f *fixture) { guDpClose(t, f, "J-2", contract.Cancelled) }},
		{"no owner", func(t *testing.T, f *fixture) { f.s.Owner = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := guDpRig(t)
			tc.run(t, f)
			guDpExit(t, f, "J-3", "cancelled")
			f.tick(t)
			f.tick(t)
			if got := gkStatus(t, f, "J-3"); got != contract.Cancelled {
				t.Fatalf("J-3 is %s, want cancelled", got)
			}
			if people, all := guDpPeople(t, f); len(people) != 0 || !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3"}) {
				t.Fatalf("want no person Job: %v (Jobs %v)", people, all)
			}
		})
	}
}

// One tick posts a re-plan Job (J-4 is rejected with J-5 waiting) and the person
// Job for the dead re-plan Job J-3: the second post picks its id on a snapshot
// that holds the first, or the fold drops it without an error.
func TestGuDpSameTickAsRejectReplan(t *testing.T) {
	f := guDpRig(t)
	f.job(t, "J-4", nil)
	f.job(t, "J-5", []any{"J-4"})
	guDpClose(t, f, "J-4", contract.Rejected)
	guDpExit(t, f, "J-3", "cancelled")
	f.tick(t)
	people, all := guDpPeople(t, f)
	if want := []string{"J-1", "J-2", "J-3", "J-4", "J-5", "J-6", "J-7"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if people["J-7"] == nil || people["J-7"]["discovered_from"] != "J-3" || len(people) != 1 {
		t.Fatalf("want one person Job J-7 from J-3, got %v", people)
	}
	if replans, _ := gkReplans(t, f); replans["J-6"] == nil || replans["J-6"]["discovered_from"] != "J-4" || replans["J-6"]["role"] != "orchestrator" {
		t.Fatalf("want the re-plan Job J-6 from J-4, got %v", replans)
	}
}

// J-1 is rejected, the base pass posts the re-plan Job J-3, J-3 is cancelled by a
// control: the chain ends at one person Job, with no second re-plan Job for J-1.
func TestGuDpChainEndsAtPerson(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	guDpClose(t, f, "J-1", contract.Rejected)
	f.tick(t)
	if replans, all := gkReplans(t, f); len(replans) != 1 || replans["J-3"] == nil || !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3"}) {
		t.Fatalf("want one re-plan Job J-3, got %v (Jobs %v)", replans, all)
	}
	guDpClose(t, f, "J-3", contract.Cancelled)
	f.tick(t)
	guDpWantOwnerJob(t, f, contract.Cancelled, contract.Rejected, "J-1")
	for i := 0; i < 4; i++ {
		f.tick(t)
	}
	guDpWantOwnerJob(t, f, contract.Cancelled, contract.Rejected, "J-1")
	if replans, all := gkReplans(t, f); !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3", "J-4"}) || len(replans) != 2 {
		t.Fatalf("Jobs on the Feed = %v, discovered_from Jobs %v: want J-1 to J-4 only", all, replans)
	}
}
