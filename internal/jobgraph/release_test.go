package jobgraph

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func verified(pass string) json.RawMessage {
	return json.RawMessage(`{"level":"L1","pass":` + pass + `,"by":"sonnet","job":"J-9","evidence":["go test ./..."]}`)
}

func pair(t *testing.T, kind EdgeKind) *Graph {
	t.Helper()
	return mustGraph(t, []job.Job{{ID: "J-1"}, {ID: "J-2", After: []Edge{{Job: "J-1", Kind: kind}}}})
}

// A Job that waits on a submitted Job is ready once that Job's verify passes.
func TestSubmittedWithPassingVerifyReleases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  EdgeKind
		first contract.Job
		ready bool
	}{
		{"bare, verify true", "", contract.Job{Status: contract.Submitted, Verified: verified("true")}, true},
		{"done, verify pass", Done, contract.Job{Status: contract.Submitted, Verified: verified(`"pass"`)}, true},
		{"submitted, no verify yet", Done, contract.Job{Status: contract.Submitted}, false},
		{"submitted, verify null", Done, contract.Job{Status: contract.Submitted, Verified: json.RawMessage(`null`)}, false},
		{"submitted, verify false", Done, contract.Job{Status: contract.Submitted, Verified: verified("false")}, false},
		{"submitted, verify unsure", Done, contract.Job{Status: contract.Submitted, Verified: verified(`"unsure"`)}, false},
		{"running with an old verify", Done, contract.Job{Status: contract.Running, Verified: verified("true")}, false},
		{"accepted without verify", Done, contract.Job{Status: contract.Accepted}, true},
		{"failed edge ignores a passing submit", Failed, contract.Job{Status: contract.Submitted, Verified: verified("true")}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := pair(t, tc.kind).Ready(Snapshot{Jobs: map[string]contract.Job{"J-1": tc.first}})
			if err != nil {
				t.Fatal(err)
			}
			if (len(got.Ready) == 1) != tc.ready || tc.ready && got.Ready[0] != "J-2" {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

// A dependent that has not started goes back to waiting when the Job it waited
// on is rejected, whether it had been released by a submit or not.
func TestRejectedBeforeStartWaitsAgain(t *testing.T) {
	g := pair(t, "")
	passing := Snapshot{Jobs: map[string]contract.Job{"J-1": {Status: contract.Submitted, Verified: verified("true")}}}
	if got, _ := g.Ready(passing); !reflect.DeepEqual(got.Ready, []string{"J-2"}) {
		t.Fatalf("not released while submitted: %+v", got)
	}
	for _, status := range []contract.Status{"", contract.Posted, contract.Ready} {
		rejected := Snapshot{Jobs: map[string]contract.Job{"J-1": {Status: contract.Rejected, Verified: verified("true")}, "J-2": {Status: status}}}
		got, err := g.Ready(rejected)
		if err != nil {
			t.Fatal(err)
		}
		want := []Wait{{Job: "J-2", Target: "J-1", Kind: Done, Reason: "waiting for J-1, which was rejected"}}
		if len(got.Ready) != 0 || !reflect.DeepEqual(got.Waiting, want) || len(got.Notes) != 0 {
			t.Fatalf("status %q: got %+v", status, got)
		}
	}
}

// A dependent that has started is never held or killed by a later rejection:
// the graph leaves it alone and reports a note for its Feed.
func TestRejectedAfterStartNotesNotKills(t *testing.T) {
	g := pair(t, "")
	for _, started := range []contract.Status{contract.Assigned, contract.Running, contract.Submitted, contract.Parked} {
		s := Snapshot{Jobs: map[string]contract.Job{"J-1": {Status: contract.Rejected}, "J-2": {Status: started}}}
		got, err := g.Ready(s)
		if err != nil {
			t.Fatal(err)
		}
		want := []Note{{Job: "J-2", Target: "J-1", Reason: "J-1 was rejected after J-2 started; J-2 carries on"}}
		if len(got.Ready) != 0 || len(got.Waiting) != 0 || !reflect.DeepEqual(got.Notes, want) {
			t.Fatalf("started %q: got %+v", started, got)
		}
	}
	for _, tc := range []struct {
		name string
		jobs map[string]contract.Job
	}{
		{"target still submitted and passing", map[string]contract.Job{"J-1": {Status: contract.Submitted, Verified: verified("true")}, "J-2": {Status: contract.Running}}},
		{"target accepted", map[string]contract.Job{"J-1": {Status: contract.Accepted}, "J-2": {Status: contract.Running}}},
		{"dependent already finished", map[string]contract.Job{"J-1": {Status: contract.Rejected}, "J-2": {Status: contract.Accepted}}},
		{"dependent not started", map[string]contract.Job{"J-1": {Status: contract.Rejected}, "J-2": {Status: contract.Posted}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := g.Ready(Snapshot{Jobs: tc.jobs})
			if err != nil || len(got.Notes) != 0 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}
