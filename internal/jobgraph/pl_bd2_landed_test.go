package jobgraph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// plBd2Post is the valid basic fixture of a Feed type with id n.
func plBd2Post(t *testing.T, typ string, n int) contract.Post {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", typ, "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var p contract.Post
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	p.ID, p.Seq = fmt.Sprintf("%026d", n), int64(n)
	return p
}

// plBd2Graph is J-1, and J-2 waiting for J-1 to land.
func plBd2Graph(t *testing.T) *Graph {
	t.Helper()
	return mustGraph(t, []job.Job{{ID: "J-1"}, {ID: "J-2", After: []Edge{{Job: "J-1", Kind: Landed}}}})
}

func plBd2Snapshot(status contract.Status, landed map[string]bool) Snapshot {
	return Snapshot{Jobs: map[string]contract.Job{"J-1": {ID: "J-1", Status: status}}, Landed: landed}
}

// Row 1 (V27, 4): an after edge of kind landed is Ready only after a landed post
// for its target. An accept is not a landing; a done edge is as it was.
func TestPlBd2LandedEdgeWaitsForLanded(t *testing.T) {
	g := plBd2Graph(t)
	wait := []Wait{{Job: "J-2", Target: "J-1", Kind: Landed, Reason: "waiting for J-1 to land"}}
	for _, tc := range []struct {
		name   string
		status contract.Status
		landed map[string]bool
		ready  bool
	}{
		{"accepted, not landed", contract.Accepted, nil, false},
		{"submitted with no landing", contract.Submitted, map[string]bool{}, false},
		{"another Job landed", contract.Accepted, map[string]bool{"J-3": true}, false},
		{"accepted and landed", contract.Accepted, map[string]bool{"J-1": true}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := g.Ready(plBd2Snapshot(tc.status, tc.landed))
			if err != nil {
				t.Fatal(err)
			}
			if got := len(r.Ready) == 1 && r.Ready[0] == "J-2"; got != tc.ready {
				t.Fatalf("Ready = %v, want J-2 ready %v", r.Ready, tc.ready)
			}
			if !tc.ready && !reflect.DeepEqual(r.Waiting, wait) {
				t.Fatalf("Waiting = %+v, want %+v", r.Waiting, wait)
			}
		})
	}
	t.Run("a done edge needs no landing", func(t *testing.T) {
		g := mustGraph(t, []job.Job{{ID: "J-1"}, {ID: "J-2", After: []Edge{{Job: "J-1", Kind: Done}}}})
		r, err := g.Ready(plBd2Snapshot(contract.Accepted, nil))
		if err != nil || !reflect.DeepEqual(r.Ready, []string{"J-2"}) {
			t.Fatalf("Ready = %v, %v; want J-2", r.Ready, err)
		}
	})
	t.Run("blocks list the landed kind", func(t *testing.T) {
		if got := g.Blocks(); !reflect.DeepEqual(got, []Block{{From: "J-1", To: "J-2", Kind: Landed}}) {
			t.Fatalf("Blocks = %+v", got)
		}
	})
}

// Row 1 end to end: the fold's Loop.Landed from a real landed post is what makes
// the edge Ready.
func TestPlBd2LandedPostMakesEdgeReady(t *testing.T) {
	record := plBd2Post(t, "job", 1)
	landed := plBd2Post(t, "landed", 4)
	landed.Job = &[]string{"J-1"}[0]
	g := plBd2Graph(t)
	for _, tc := range []struct {
		name  string
		posts []contract.Post
		ready bool
	}{
		{"no landed post", []contract.Post{record}, false},
		{"a landed post for J-1", []contract.Post{record, landed}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, loop := contract.FoldLoop(tc.posts)
			in := Snapshot{Jobs: map[string]contract.Job{}, Landed: map[string]bool{}}
			for id, j := range state.Jobs {
				in.Jobs[id] = *j
			}
			for id := range loop.Landed {
				in.Landed[id] = true
			}
			r, err := g.Ready(in)
			if err != nil {
				t.Fatal(err)
			}
			if got := reflect.DeepEqual(r.Ready, []string{"J-1", "J-2"}); got != tc.ready {
				t.Fatalf("Ready = %v, Waiting = %+v; want J-2 ready %v", r.Ready, r.Waiting, tc.ready)
			}
		})
	}
}
