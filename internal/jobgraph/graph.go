// Package jobgraph evaluates planned Job dependencies without reading files,
// posting records, executing commands, or changing the caller's plan or state.
package jobgraph

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

type EdgeKind string

const (
	Done         EdgeKind = "done"
	Failed       EdgeKind = "failed"
	ChildrenDone EdgeKind = "children-done"
)

// Edge is an after dependency. Empty Kind means done, except for gates.
// Bare ids in JSON expand to this same shape with an empty Why.
type Edge struct {
	Job  string   `json:"job"`
	Kind EdgeKind `json:"kind,omitempty"`
	Why  string   `json:"why,omitempty"`
}

// MarshalJSON preserves the bare-id shorthand when there is no explanation.
func (e Edge) MarshalJSON() ([]byte, error) {
	if e.Kind == "" && e.Why == "" {
		return json.Marshal(e.Job)
	}
	type plain Edge
	return json.Marshal(plain(e))
}

func (e *Edge) UnmarshalJSON(data []byte) error {
	var id string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &id); err != nil {
			return err
		}
		*e = Edge{Job: id}
		return nil
	}
	type plain Edge
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	// Object edges must explain the dependency; bare ids are the shorthand.
	if err := validWhy(value.Why, value.Job); err != nil {
		return err
	}
	*e = Edge(value)
	return nil
}

func validWhy(why, target string) error {
	n := utf8.RuneCountInString(strings.TrimSpace(why))
	s := strings.ToLower(strings.TrimSpace(why))
	if n < 8 || n > 160 || s == "see also" || s == "related" || s == strings.ToLower(target) {
		return fmt.Errorf("after %s: why must explain the link in 8 to 160 characters", target)
	}
	return nil
}

type Verify struct {
	Command  string `json:"command,omitempty"`
	MinTests int    `json:"min_tests,omitempty"`
	Kind     string `json:"kind,omitempty"` // file-exists or reviewer-check for non-code
}

// Item is a plan projection, not a Feed or task record. Kind defaults to code;
// non-code, research and scout opt into non-code verification. Rename and
// generated describe low-risk code work. ReviewReason records an explicit
// exception to the floor; authorization of its author belongs to the caller.
type Item struct {
	ID           string   `json:"id"`
	After        []Edge   `json:"after,omitempty"`
	Review       string   `json:"review,omitempty"`
	Risk         []string `json:"risk,omitempty"`
	Write        []string `json:"write,omitempty"`
	Verify       Verify   `json:"verify"`
	Kind         string   `json:"kind,omitempty"`
	Visible      string   `json:"visible,omitempty"`
	Irreversible bool     `json:"irreversible,omitempty"`
	ReviewReason string   `json:"review_reason,omitempty"`
}

type Graph struct {
	ids      []string
	items    map[string]Item
	children map[string][]string
}

var jobID = regexp.MustCompile(`^J-[0-9]+(?:\.[0-9]+)*$`)
var gateID = regexp.MustCompile(`^gate:Q-[0-9]+$`)

// New validates ids, targets and edge kinds. Gate existence is checked by the
// caller that owns the Feed; a missing gate state remains a wait. A cycle is
// returned by Cycle or Ready, allowing callers to inspect an invalid graph.
func New(items []Item) (*Graph, error) {
	g := &Graph{items: make(map[string]Item), children: make(map[string][]string)}
	for _, item := range items {
		if !jobID.MatchString(item.ID) {
			return nil, fmt.Errorf("invalid Job id %q", item.ID)
		}
		if _, ok := g.items[item.ID]; ok {
			return nil, fmt.Errorf("duplicate Job %s", item.ID)
		}
		item.After = append([]Edge(nil), item.After...)
		item.Risk = append([]string(nil), item.Risk...)
		item.Write = append([]string(nil), item.Write...)
		g.items[item.ID] = item
		g.ids = append(g.ids, item.ID)
		if i := strings.LastIndexByte(item.ID, '.'); i >= 0 {
			parent := item.ID[:i]
			g.children[parent] = append(g.children[parent], item.ID)
		}
	}
	sort.Strings(g.ids)
	for parent := range g.children {
		sort.Strings(g.children[parent])
	}
	for _, id := range g.ids {
		for _, e := range g.items[id].After {
			if strings.HasPrefix(e.Job, "gate:") {
				if !gateID.MatchString(e.Job) || e.Kind != "" {
					return nil, fmt.Errorf("%s: invalid gate edge %q (gates have no kind)", id, e.Job)
				}
			} else {
				if _, ok := g.items[e.Job]; !ok {
					return nil, fmt.Errorf("%s: after target %q does not exist", id, e.Job)
				}
				switch e.Kind {
				case "", Done, Failed, ChildrenDone:
				default:
					return nil, fmt.Errorf("%s: unknown after kind %q", id, e.Kind)
				}
			}
			if e.Why != "" {
				if err := validWhy(e.Why, e.Job); err != nil {
					return nil, fmt.Errorf("%s: %w", id, err)
				}
			}
		}
	}
	return g, nil
}

// Block is the inverse of an after edge. Gates remain explicit blockers.
type Block struct {
	From string   `json:"from"`
	To   string   `json:"to"`
	Kind EdgeKind `json:"kind,omitempty"`
	Why  string   `json:"why,omitempty"`
}

func edgeKind(e Edge) EdgeKind {
	if e.Kind == "" && !strings.HasPrefix(e.Job, "gate:") {
		return Done
	}
	return e.Kind
}

func (g *Graph) Blocks() []Block {
	out := []Block{}
	for _, id := range g.ids {
		for _, e := range g.items[id].After {
			out = append(out, Block{From: e.Job, To: id, Kind: edgeKind(e), Why: e.Why})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.To != b.To {
			return a.To < b.To
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Why < b.Why
	})
	return out
}

// Cycle returns the first cycle as a closed path, or an empty slice. Traversal
// is lexical and stable across input permutations. children-done also depends
// on the target's immediate children, so a split cannot hide a deadlock.
func (g *Graph) Cycle() []string {
	color := make(map[string]uint8)
	stack := []string{}
	positions := make(map[string]int)
	var cycle []string
	var visit func(string) bool
	visit = func(id string) bool {
		color[id] = 1
		positions[id] = len(stack)
		stack = append(stack, id)
		deps := []string{}
		for _, e := range g.items[id].After {
			if strings.HasPrefix(e.Job, "gate:") {
				continue
			}
			deps = append(deps, e.Job)
			if e.Kind == ChildrenDone {
				deps = append(deps, g.children[e.Job]...)
			}
		}
		sort.Strings(deps)
		for _, next := range deps {
			if color[next] == 1 {
				cycle = append(append([]string{}, stack[positions[next]:]...), next)
				return true
			}
			if color[next] == 0 && visit(next) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = 2
		return false
	}
	for _, id := range g.ids {
		if color[id] == 0 && visit(id) {
			return cycle
		}
	}
	return []string{}
}

type CycleError struct{ Path []string }

func (e *CycleError) Error() string { return "Job cycle: " + strings.Join(e.Path, " -> ") }

// Snapshot uses contract.Job for run records. Exhausted is explicit: an absent
// retry decision must not release a failed edge. OpenGates keys are gate:Q-id.
// Optional times enable the canonical order (ready since, created at, id).
// Without timestamps, Ready returns lexical id order, not a scheduling history.
type Snapshot struct {
	Jobs       map[string]contract.Job
	Exhausted  map[string]bool
	OpenGates  map[string]bool
	ReadySince map[string]time.Time
	CreatedAt  map[string]time.Time
}

type Wait struct {
	Job    string   `json:"job"`
	Target string   `json:"target"`
	Kind   EdgeKind `json:"kind,omitempty"`
	Reason string   `json:"reason"`
}

type Readiness struct {
	Ready   []string `json:"ready"`
	Waiting []Wait   `json:"waiting"`
}

// Ready lists unstarted posted/ready Jobs whose dependencies are satisfied.
// No children is a wait, not a vacuous success before a split has happened.
// Already assigned, running, parked or terminal Jobs are never offered again.
func (g *Graph) Ready(s Snapshot) (Readiness, error) {
	out := Readiness{Ready: []string{}, Waiting: []Wait{}}
	if cycle := g.Cycle(); len(cycle) > 0 {
		return out, &CycleError{Path: cycle}
	}
	for _, id := range g.ids {
		status := s.Jobs[id].Status
		if status != "" && status != contract.Posted && status != contract.Ready {
			continue
		}
		start := len(out.Waiting)
		for _, e := range g.items[id].After {
			kind := edgeKind(e)
			satisfied := false
			reason := ""
			switch {
			case strings.HasPrefix(e.Job, "gate:"):
				satisfied = s.OpenGates[e.Job]
				reason = "waiting on gate " + strings.TrimPrefix(e.Job, "gate:")
			case kind == Done:
				satisfied = s.Jobs[e.Job].Status == contract.Accepted
				reason = "waiting for " + e.Job + " to be accepted"
			case kind == Failed:
				status := s.Jobs[e.Job].Status
				satisfied = (status == contract.Rejected || status == contract.Cancelled) && s.Exhausted[e.Job]
				reason = "waiting for " + e.Job + " to fail with no attempt left"
			case kind == ChildrenDone:
				children := g.children[e.Job]
				satisfied = len(children) > 0
				for _, child := range children {
					if s.Jobs[child].Status != contract.Accepted {
						satisfied = false
					}
				}
				reason = "waiting for all children of " + e.Job + " to be accepted"
			}
			if !satisfied {
				out.Waiting = append(out.Waiting, Wait{Job: id, Target: e.Job, Kind: kind, Reason: reason})
			}
		}
		if len(out.Waiting) == start {
			out.Ready = append(out.Ready, id)
		}
	}
	sort.Slice(out.Ready, func(i, j int) bool {
		a, b := out.Ready[i], out.Ready[j]
		if !s.ReadySince[a].Equal(s.ReadySince[b]) {
			return s.ReadySince[a].Before(s.ReadySince[b])
		}
		if !s.CreatedAt[a].Equal(s.CreatedAt[b]) {
			return s.CreatedAt[a].Before(s.CreatedAt[b])
		}
		return a < b
	})
	sort.Slice(out.Waiting, func(i, j int) bool {
		a, b := out.Waiting[i], out.Waiting[j]
		if a.Job != b.Job {
			return a.Job < b.Job
		}
		if a.Target != b.Target {
			return a.Target < b.Target
		}
		return a.Kind < b.Kind
	})
	return out, nil
}
