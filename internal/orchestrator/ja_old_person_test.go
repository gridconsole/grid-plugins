package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// oldPersonBody is a person Job of the kind the Judge raised before an agent decided: the owner
// "decides how J-1 continues".
func oldPersonBody(t *testing.T, id, title, from string) map[string]any {
	body := personJobBody(t, id)
	body["title"] = title
	body["discovered_from"] = from
	body["acceptance"] = []any{map[string]any{"id": "A1", "text": "dana decides how J-1 continues", "traces": "owner"}}
	return body
}

// jaDecisions are the orchestrator decision Jobs on the Feed ("Decide how ...").
func jaDecisions(t *testing.T, f *fixture) (out []string) {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for id, b := range snap.Bodies {
		if b["role"] == "orchestrator" && b["discovered_from"] == "J-1" {
			out = append(out, id)
		}
	}
	return out
}

// An open person Job "Decide how J-1 continues ..." is replaced by one agent decision Job on the
// tick, with nobody running a command; the next tick posts nothing, and a workflow point gate
// ("Judge J-1 before ...") is left open.
func TestTickConvertsAnOldDecidePersonJobOnce(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	rev := jobBody(t, "J-1.1")
	rev["role"] = "reviewer"
	rev["discovered_from"] = "J-1"
	f.post(t, feed.Input{Type: "job", Body: rev, Job: "J-1.1"}, f.s.Writer)
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-1.1", "status": "cancelled", "reason": "prepare_failed", "close_reason": "prepare_failed"}}, f.s.Writer)
	f.post(t, feed.Input{Type: "job", Body: oldPersonBody(t, "J-1.2", "Decide how J-1 continues without its review", "J-1"), Job: "J-1.2"}, f.s.Writer)
	f.post(t, feed.Input{Type: "job", Body: oldPersonBody(t, "J-5", "Judge J-1 before ship", "J-1"), Job: "J-5"}, f.s.Writer)

	f.tick(t)
	if got := jaDecisions(t, f); len(got) != 1 {
		t.Fatalf("decision Jobs %v, want exactly one", got)
	}
	if got := jobStatus(t, f, "J-1.2"); got != contract.Cancelled {
		t.Fatalf("the old person Job is %s, want cancelled", got)
	}
	if got := jobStatus(t, f, "J-5"); got == contract.Cancelled || got == contract.Accepted {
		t.Fatalf("the point gate Job is %s, want it left open", got)
	}

	before := recordCount(t, f)
	f.tick(t)
	if got := jaDecisions(t, f); len(got) != 1 {
		t.Fatalf("a second tick left decision Jobs %v, want still one", got)
	}
	if after := recordCount(t, f); after != before {
		t.Fatalf("a second tick wrote %d records, want none", after-before)
	}
}
