package jobgraph

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

func mustGraph(t *testing.T, items []Item) *Graph {
	t.Helper()
	g, err := New(items)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestGraphValidation(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"bare", `[{"id":"J-1"},{"id":"J-2","after":["J-1","gate:Q-7"]}]`, ""},
		{"typed", `[{"id":"J-1"},{"id":"J-2","after":[{"job":"J-1","kind":"failed","why":"Fallback on failure"}]}]`, ""},
		{"missing", `[{"id":"J-2","after":["J-1"]}]`, "does not exist"},
		{"duplicate", `[{"id":"J-1"},{"id":"J-1"}]`, "duplicate"},
		{"id", `[{"id":"J-1.bad"}]`, "invalid Job id"},
		{"kind", `[{"id":"J-1","after":[{"job":"J-1","kind":"maybe","why":"A useful explanation"}]}]`, "unknown after kind"},
		{"why missing", `[{"id":"J-1","after":[{"job":"J-1"}]}]`, "why"},
		{"why unhelpful", `[{"id":"J-1","after":[{"job":"J-1","why":"see also"}]}]`, "why"},
		{"gate kind", `[{"id":"J-1","after":[{"job":"gate:Q-7","kind":"done","why":"Wait for approval"}]}]`, "gates have no kind"},
		{"bad gate", `[{"id":"J-1","after":["gate:"]}]`, "invalid gate"},
		{"null", `[{"id":"J-1","after":[null]}]`, "why"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var items []Item
			err := json.Unmarshal([]byte(tc.input), &items)
			if err == nil {
				_, err = New(items)
			}
			if tc.want == "" && err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestBlocksAndImmutableInput(t *testing.T) {
	items := []Item{{ID: "J-2", After: []Edge{{Job: "J-1", Why: "Needs the base result"}, {Job: "gate:Q-7"}}}, {ID: "J-1"}}
	g := mustGraph(t, items)
	items[0].After[0].Job = "J-2"
	want := []Block{{From: "J-1", To: "J-2", Kind: Done, Why: "Needs the base result"}, {From: "gate:Q-7", To: "J-2"}}
	if got := g.Blocks(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	if len(g.Cycle()) != 0 {
		t.Fatal("caller mutation changed graph")
	}
}

func TestEdgeJSONRoundTrip(t *testing.T) {
	input := `["J-1","gate:Q-7",{"job":"J-2","kind":"children-done","why":"Wait for the split Jobs"}]`
	var edges, again []Edge
	if err := json.Unmarshal([]byte(input), &edges); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(edges)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &again); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(edges, again) {
		t.Fatalf("round trip changed edges: %s", encoded)
	}
}

func TestCyclePaths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		items []Item
		want  []string
	}{
		{"empty", nil, []string{}},
		{"self", []Item{{ID: "J-1", After: []Edge{{Job: "J-1"}}}}, []string{"J-1", "J-1"}},
		{"disconnected", []Item{{ID: "J-0"}, {ID: "J-2", After: []Edge{{Job: "J-1", Kind: Failed}}}, {ID: "J-1", After: []Edge{{Job: "J-2"}}}}, []string{"J-1", "J-2", "J-1"}},
		{"split deadlock", []Item{{ID: "J-1"}, {ID: "J-1.1", After: []Edge{{Job: "J-2"}}}, {ID: "J-2", After: []Edge{{Job: "J-1", Kind: ChildrenDone}}}}, []string{"J-1.1", "J-2", "J-1.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := mustGraph(t, tc.items)
			if got := g.Cycle(); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			_, err := g.Ready(Snapshot{})
			var cycleErr *CycleError
			if len(tc.want) > 0 && (!errors.As(err, &cycleErr) || !reflect.DeepEqual(cycleErr.Path, tc.want)) {
				t.Fatalf("Ready: %v", err)
			}
		})
	}
}

// A separate Kahn topological sort is the oracle, not a second DFS.
func TestCycleRandomGraphs(t *testing.T) {
	rng := rand.New(rand.NewSource(2501))
	for trial := 0; trial < 1000; trial++ {
		n := 1 + rng.Intn(30)
		items := make([]Item, n)
		degree := make([]int, n)
		adj := make([][]int, n)
		for i := range items {
			items[i].ID = fmt.Sprintf("J-%d", i)
		}
		for i := range items {
			for j := range items {
				if rng.Intn(100) < 6 && (trial%2 == 0 || i < j) {
					items[i].After = append(items[i].After, Edge{Job: items[j].ID})
					degree[j]++
					adj[i] = append(adj[i], j)
				}
			}
		}
		queue := []int{}
		for i, d := range degree {
			if d == 0 {
				queue = append(queue, i)
			}
		}
		for k := 0; k < len(queue); k++ {
			for _, j := range adj[queue[k]] {
				degree[j]--
				if degree[j] == 0 {
					queue = append(queue, j)
				}
			}
		}
		g := mustGraph(t, items)
		cycle := g.Cycle()
		if (len(cycle) > 0) != (len(queue) != n) {
			t.Fatalf("trial %d: cycle %v, removed %d/%d", trial, cycle, len(queue), n)
		}
		if len(cycle) > 0 {
			if cycle[0] != cycle[len(cycle)-1] {
				t.Fatal("cycle is not closed")
			}
			for k := 1; k < len(cycle); k++ {
				found := false
				for _, e := range g.items[cycle[k-1]].After {
					if e.Job == cycle[k] {
						found = true
					}
				}
				if !found {
					t.Fatalf("invented cycle edge: %v", cycle)
				}
			}
		}
		rng.Shuffle(n, func(i, j int) { items[i], items[j] = items[j], items[i] })
		for i := range items {
			rng.Shuffle(len(items[i].After), func(a, b int) { items[i].After[a], items[i].After[b] = items[i].After[b], items[i].After[a] })
		}
		if got := mustGraph(t, items).Cycle(); !reflect.DeepEqual(cycle, got) {
			t.Fatalf("unstable cycle: %v vs %v", cycle, got)
		}
	}
}

func TestReadyEdges(t *testing.T) {
	for _, tc := range []struct {
		name             string
		kind             EdgeKind
		status           contract.Status
		exhausted, ready bool
	}{
		{"bare accepted", "", contract.Accepted, false, true},
		{"done accepted", Done, contract.Accepted, false, true},
		{"submitted is not done", Done, contract.Submitted, false, false},
		{"rejected is not done", Done, contract.Rejected, true, false},
		{"failed rejected", Failed, contract.Rejected, true, true},
		{"failed cancelled", Failed, contract.Cancelled, true, true},
		{"cancelled with a retry left", Failed, contract.Cancelled, false, false},
		{"retry left", Failed, contract.Rejected, false, false},
		{"accepted is not failed", Failed, contract.Accepted, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := mustGraph(t, []Item{{ID: "J-1"}, {ID: "J-2", After: []Edge{{Job: "J-1", Kind: tc.kind}}}})
			got, err := g.Ready(Snapshot{Jobs: map[string]contract.Job{"J-1": {Status: tc.status}}, Exhausted: map[string]bool{"J-1": tc.exhausted}})
			if err != nil {
				t.Fatal(err)
			}
			if (len(got.Ready) == 1) != tc.ready || tc.ready && got.Ready[0] != "J-2" {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestReadyChildrenAndGates(t *testing.T) {
	g := mustGraph(t, []Item{{ID: "J-14"}, {ID: "J-14.1"}, {ID: "J-14.2"}, {ID: "J-140.1"}, {ID: "J-20", After: []Edge{{Job: "J-14", Kind: ChildrenDone}, {Job: "gate:Q-7"}}}})
	s := Snapshot{Jobs: map[string]contract.Job{"J-14": {Status: contract.Parked}, "J-14.1": {Status: contract.Accepted}, "J-14.2": {Status: contract.Running}, "J-140.1": {Status: contract.Running}}}
	for _, tc := range []struct {
		name                  string
		acceptChild, openGate bool
		waits                 int
	}{
		{"both waiting", false, false, 2}, {"last child", true, false, 1}, {"gate opens", true, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.acceptChild {
				s.Jobs["J-14.2"] = contract.Job{Status: contract.Accepted}
			}
			s.OpenGates = map[string]bool{"gate:Q-7": tc.openGate}
			got, err := g.Ready(s)
			if err != nil || len(got.Waiting) != tc.waits {
				t.Fatalf("got %+v, %v", got, err)
			}
			if tc.waits == 0 && !reflect.DeepEqual(got.Ready, []string{"J-20"}) {
				t.Fatalf("ready %v", got.Ready)
			}
			if tc.waits == 1 && got.Waiting[0].Reason != "waiting on gate Q-7" {
				t.Fatalf("wait %+v", got.Waiting)
			}
		})
	}
	g = mustGraph(t, []Item{{ID: "J-14"}, {ID: "J-20", After: []Edge{{Job: "J-14", Kind: ChildrenDone}}}})
	got, _ := g.Ready(Snapshot{})
	if len(got.Waiting) != 1 {
		t.Fatal("unsplit parent released children-done")
	}
}

func TestReadyOrderAndOwnState(t *testing.T) {
	items := []Item{}
	jobs := map[string]contract.Job{}
	for i, status := range []contract.Status{"", contract.Posted, contract.Ready, contract.Assigned, contract.Running, contract.Submitted, contract.Accepted, contract.Rejected, contract.Cancelled, contract.Stale, contract.Parked} {
		id := fmt.Sprintf("J-%d", i)
		items = append(items, Item{ID: id})
		jobs[id] = contract.Job{Status: status}
	}
	now := time.Unix(100, 0)
	got, err := mustGraph(t, items).Ready(Snapshot{Jobs: jobs, ReadySince: map[string]time.Time{"J-0": now, "J-1": now, "J-2": now.Add(-time.Second)}, CreatedAt: map[string]time.Time{"J-0": now, "J-1": now.Add(-time.Second)}})
	if err != nil || !reflect.DeepEqual(got.Ready, []string{"J-2", "J-1", "J-0"}) {
		t.Fatalf("got %+v, %v", got, err)
	}
}
