package orchestrator

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// guPcWithdraw withdraws the Job id with a control of status cancelled, the way
// `grid job cancel` does: nothing exited, so the Job has no worker-exit.
func guPcWithdraw(t *testing.T, f *fixture, id string) {
	t.Helper()
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": id, "status": "cancelled", "reason": "withdrawn", "close_reason": "withdrawn"}}, f.s.Writer)
}

// guPcPerson posts the person Job id, as personFixture does for J-1.
func guPcPerson(t *testing.T, f *fixture, id string, after []any) {
	t.Helper()
	body := jobBody(t, id)
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	if after != nil {
		body["after"] = after
	}
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

// guPcWantReplan checks the re-plan Job id the Feed holds for the withdrawn Job
// cancelled while the Jobs in waiting hold a done edge to it: role orchestrator,
// discovered_from the withdrawn Job, the title and A1 text that say it was
// cancelled, valid by contract.Validate, and equal to what ReplanJobBody builds.
func guPcWantReplan(t *testing.T, f *fixture, id, cancelled string, waiting []string) {
	t.Helper()
	replans, all := gkReplans(t, f)
	body := replans[id]
	if body == nil {
		t.Fatalf("want a re-plan Job %s, got %v (Jobs %v)", id, replans, all)
	}
	if body["role"] != "orchestrator" || body["discovered_from"] != cancelled {
		t.Fatalf("role %v, discovered_from %v", body["role"], body["discovered_from"])
	}
	if want := "Re-plan after " + cancelled + " was cancelled: " + strings.Join(waiting, ", ") + " wait on it"; body["title"] != want {
		t.Fatalf("title = %v, want %q", body["title"], want)
	}
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	for _, w := range waiting {
		if want := w + " (waiting for " + cancelled + ", which was cancelled)"; !strings.Contains(text, want) {
			t.Fatalf("A1 = %v, want it to hold %q", a1, want)
		}
	}
	if strings.Contains(text, "Stopped because") {
		t.Fatalf("A1 = %v: a person Job has no worker-exit, so no cause", a1)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the body on the Feed is not a job-record: %v", err)
	}
	// What the pass posts is what ReplanJobBody builds, with no cause.
	want, err := json.Marshal(ReplanJobBody(id, cancelled, "", waiting))
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

func TestGuPcPersonWithdrawn(t *testing.T) {
	f := personFixture(t)
	guPcWithdraw(t, f, "J-1")
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if got := f.state(t).ExitReason; got != "" {
		t.Fatalf("J-1 has exit reason %q, want none: a person Job has no worker-exit", got)
	}
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3", "J-4"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if len(replans) != 1 {
		t.Fatalf("want one Job with a discovered_from, got %v", replans)
	}
	guPcWantReplan(t, f, "J-4", "J-1", []string{"J-2"})
}

func TestGuPcOnce(t *testing.T) {
	f := personFixture(t)
	guPcWithdraw(t, f, "J-1")
	for i := 0; i < 4; i++ {
		f.tick(t)
	}
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3", "J-4"}; !reflect.DeepEqual(all, want) || len(replans) != 1 {
		t.Fatalf("Jobs on the Feed = %v, discovered_from Jobs %v: want one re-plan Job J-4", all, replans)
	}
	guPcWantReplan(t, f, "J-4", "J-1", []string{"J-2"})
	if got := gkStatus(t, f, "J-2"); got == contract.Cancelled {
		t.Fatalf("J-2 is %s: the pass cancels nothing", got)
	}
}

// An agent Job cancelled by a control alone was decided by someone: gk's rule
// stands, and the pass of this lane adds nothing for it, run by a tick or alone.
func TestGuPcAgentControlCancelStillNothing(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	guPcWithdraw(t, f, "J-1")
	f.tick(t)
	f.tick(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.s.guReplanPersonCancelled(snap); err != nil {
		t.Fatal(err)
	}
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if replans, all := gkReplans(t, f); len(replans) != 0 || !reflect.DeepEqual(all, []string{"J-1", "J-2"}) {
		t.Fatalf("a control-only cancel of an agent Job got a Job: %v (Jobs %v)", replans, all)
	}
}

// A person Job that is answered is Accepted, not withdrawn: the Jobs behind it
// are released and no re-plan Job is posted.
func TestGuPcAnsweredNothing(t *testing.T) {
	f := personFixture(t)
	f.tick(t)
	if _, err := AnswerPerson(f.s.Feed, f.s.Writer, "J-1", "Call it Lark.", f.now); err != nil {
		t.Fatal(err)
	}
	f.tick(t)
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s, want accepted", got)
	}
	if replans, all := gkReplans(t, f); len(replans) != 0 || !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3"}) {
		t.Fatalf("an answered person Job got a Job: %v (Jobs %v)", replans, all)
	}
}

// Nothing waits on the withdrawn person Job: no re-plan Job. Nor when the only
// waiter is cancelled first.
func TestGuPcNoWaiter(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(t *testing.T, f *fixture)
	}{
		{"waiter cancelled in the same tick", func(t *testing.T, f *fixture) {
			f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-2", "status": "cancelled", "reason": "r", "close_reason": "r"}}, f.s.Writer)
		}},
		{"waiter cancelled in an earlier tick", func(t *testing.T, f *fixture) {
			f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-2", "status": "cancelled", "reason": "r", "close_reason": "r"}}, f.s.Writer)
			f.tick(t)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := personFixture(t)
			tc.run(t, f)
			guPcWithdraw(t, f, "J-1")
			f.tick(t)
			f.tick(t)
			if replans, all := gkReplans(t, f); len(replans) != 0 || !reflect.DeepEqual(all, []string{"J-1", "J-2", "J-3"}) {
				t.Fatalf("want no Job: %v (Jobs %v)", replans, all)
			}
		})
	}
}

// One tick posts the person Job for a dead re-plan Job (guReplanAskOwner, J-6) and
// the re-plan Job for a withdrawn person Job (J-7): the second post picks its id on
// a snapshot that holds the first, or the fold drops it without an error.
func TestGuPcSameTickAsOwnerJob(t *testing.T) {
	f := guDpRig(t)
	guPcPerson(t, f, "J-4", nil)
	f.job(t, "J-5", []any{"J-4"})
	guPcWithdraw(t, f, "J-4")
	guDpExit(t, f, "J-3", "cancelled")
	f.tick(t)
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3", "J-4", "J-5", "J-6", "J-7"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if replans["J-6"] == nil || replans["J-6"]["role"] != "Person" || replans["J-6"]["discovered_from"] != "J-3" {
		t.Fatalf("want the person Job J-6 from J-3, got %v", replans)
	}
	guPcWantReplan(t, f, "J-7", "J-4", []string{"J-5"})
}

// J-1 is withdrawn and the pass posts the re-plan Job J-4; J-4 is cancelled by a
// control: the chain ends at one person Job for the owner, with no second re-plan
// Job for J-1.
func TestGuPcChainEndsAtOwner(t *testing.T) {
	f := personFixture(t)
	guPcWithdraw(t, f, "J-1")
	f.tick(t)
	guPcWantReplan(t, f, "J-4", "J-1", []string{"J-2"})
	guDpClose(t, f, "J-4", contract.Cancelled)
	for i := 0; i < 4; i++ {
		f.tick(t)
	}
	replans, all := gkReplans(t, f)
	if want := []string{"J-1", "J-2", "J-3", "J-4", "J-5"}; !reflect.DeepEqual(all, want) {
		t.Fatalf("Jobs on the Feed = %v, want %v", all, want)
	}
	if replans["J-5"] == nil || replans["J-5"]["role"] != "Person" || replans["J-5"]["discovered_from"] != "J-4" || len(replans) != 2 {
		t.Fatalf("want the person Job J-5 from J-4 and the re-plan Job J-4, got %v", replans)
	}
}
