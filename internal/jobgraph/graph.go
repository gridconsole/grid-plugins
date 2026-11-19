// Package jobgraph evaluates planned Job dependencies without reading files,
// posting records, executing commands, or changing the caller's plan or state.
package jobgraph

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

type EdgeKind = string

const (
	Done         = "done"
	Failed       = "failed"
	ChildrenDone = "children-done"
	// Landed is met only by a counted landed post for the target (Snapshot.Landed);
	// an accept is not a landing.
	Landed = "landed"
)

type Edge = job.Dependency

type Graph struct {
	ids      []string
	items    map[string]job.Job
	children map[string][]string
}

var jobID = regexp.MustCompile(`^J-[0-9]+(?:\.[0-9]+)*$`)
var gateID = regexp.MustCompile(`^gate:Q-[0-9]+$`)

// New validates ids, targets and edge kinds. Gate existence is checked by the
// caller that owns the Feed; a missing gate state remains a wait. A cycle is
// returned by Cycle or Ready, allowing callers to inspect an invalid graph.
func New(items []job.Job) (*Graph, error) {
	g := &Graph{items: make(map[string]job.Job), children: make(map[string][]string)}
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
	}
	sort.Strings(g.ids)
	g.link()
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
				case "", Done, Failed, ChildrenDone, Landed:
				default:
					return nil, fmt.Errorf("%s: unknown after kind %q", id, e.Kind)
				}
			}
			if e.Why != "" {
				if err := job.ValidDependencyWhy(e.Why, e.Job); err != nil {
					return nil, fmt.Errorf("%s: %w", id, err)
				}
			}
		}
	}
	return g, nil
}

// link fills children from g.ids: each Job is a child of the Job its id is
// numbered under, and a Job the Judge numbers under the first build of a rework
// is also a child of the Job it judges (judged).
func (g *Graph) link() {
	for _, id := range g.ids {
		if i := strings.LastIndexByte(id, '.'); i >= 0 {
			g.children[id[:i]] = append(g.children[id[:i]], id)
		}
	}
	for _, id := range g.ids {
		if parent := g.judged(id); parent != "" {
			g.children[parent] = append(g.children[parent], id)
		}
	}
	for parent := range g.children {
		sort.Strings(g.children[parent])
	}
}

// judged is the Job that id judges when id is not numbered under it, or "". A
// review or check judges the Job it is discovered from (app judge.go startReview);
// a person or decision Job judges the Job it is discovered from, or the Job its
// review judges, and judge.Person numbers it under the first build (person.go
// childID(rootOf)). Reworks and other Jobs are not linked.
func (g *Graph) judged(id string) string {
	it := g.items[id]
	if it.DiscoveredFrom == nil {
		return ""
	}
	from := *it.DiscoveredFrom
	if strings.HasPrefix(id, from+".") {
		return ""
	}
	switch {
	case isReview(it):
	case it.IsPerson() || isDecision(it):
		if r, ok := g.items[from]; ok && isReview(r) && r.DiscoveredFrom != nil {
			from = *r.DiscoveredFrom
		}
	default:
		return ""
	}
	if _, ok := g.items[from]; !ok || from == id || strings.HasPrefix(id, from+".") {
		return ""
	}
	return from
}

func isReview(it job.Job) bool {
	role := contract.NormalizeRole(it.Role)
	return role == "reviewer" || role == "checker"
}

// isDecision is a decision Job the Judge posts for an agent (judge.decisionBody).
func isDecision(it job.Job) bool {
	return contract.NormalizeRole(it.Role) == "orchestrator" && strings.HasPrefix(it.Title, decidePrefix)
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
// Landed holds the Jobs that have a counted landed post (contract.FoldLoop's
// Loop.Landed); a landed edge on a Job not in it waits.
type Snapshot struct {
	Jobs       map[string]contract.Job
	Exhausted  map[string]bool
	OpenGates  map[string]bool
	ReadySince map[string]time.Time
	CreatedAt  map[string]time.Time
	Landed     map[string]bool
}

type Wait struct {
	Job    string   `json:"job"`
	Target string   `json:"target"`
	Kind   EdgeKind `json:"kind,omitempty"`
	Reason string   `json:"reason"`
}

// Note is something a started Job's Feed should hear: a Job it waited on was
// rejected after it started. It is information only; nothing is held or killed.
type Note struct {
	Job    string `json:"job"`
	Target string `json:"target"`
	Reason string `json:"reason"`
}

type Readiness struct {
	Ready   []string `json:"ready"`
	Waiting []Wait   `json:"waiting"`
	Notes   []Note   `json:"notes,omitempty"`
}

// VerifyPassed reports whether the Job's latest result carries a passing
// verify: a verified object whose pass is true or "pass". An absent or null
// verified, false, and "unsure" are not a pass.
func VerifyPassed(j contract.Job) bool {
	var v struct {
		Pass any `json:"pass"`
	}
	if len(j.Verified) == 0 || json.Unmarshal(j.Verified, &v) != nil {
		return false
	}
	switch p := v.Pass.(type) {
	case bool:
		return p
	case string:
		return p == "pass"
	}
	return false
}

// Released reports whether a Job releases the Jobs that wait on it with a done
// edge: it is accepted, or it is submitted with its verify passing. Rejected,
// cancelled, running and a submit whose verify has not passed do not release.
func Released(j contract.Job) bool {
	return j.Status == contract.Accepted || j.Status == contract.Submitted && VerifyPassed(j)
}

// DoneReason is why a Job with a done edge on target still waits, given the
// target's status. A rejected or cancelled target will not be accepted by
// itself, so the reason says what happened to it instead of promising an accept.
func DoneReason(target string, status contract.Status) string {
	switch status {
	case contract.Rejected:
		return "waiting for " + target + ", which was rejected"
	case contract.Cancelled:
		return "waiting for " + target + ", which was cancelled"
	}
	return "waiting for " + target + " to be accepted"
}

// childrenDone reports whether a children-done edge on target is met, and why it
// waits when it is not. Every immediate child of target is accepted, and there
// is one: no children is a wait, not a vacuous success before a split. A target
// that is still open (a parked split parent, say) is not asked for more. A target
// closed without its work does not meet the edge, however its children ended: a
// review child is accepted even when its verdict rejected the work (end review
// 0612Z M1). A cancelled target waits unless another Job continues it; a rejected
// one, or a cancelled one so continued, meets the edge through those Jobs (reworkDone). Cycle does not follow reworks: they are
// posted after the plan, when the review rejects.
//
// An open target meets the edge on its children alone (end review 0812Z S5): a
// parked split parent has no other way to release it. For a build with a review
// child this relies on the order the app's Judge uses, build closed first and the
// review accepted after; a review accepted while the build is still submitted
// would make the edge met on an unaccepted build. s5_children_done_open_target_test.go
// pins it. An open target with a child numbered under another Job (a rework's
// review, person or decision Job) waits for its own accept.
func (g *Graph) childrenDone(s Snapshot, target string) (bool, string) {
	status := s.Jobs[target].Status
	if !closed(status) {
		for _, child := range g.children[target] {
			if !strings.HasPrefix(child, target+".") {
				return false, DoneReason(target, status)
			}
		}
	}
	if !g.childrenAccepted(s, target) || len(g.holding(s, target)) == 0 {
		return false, "waiting for all children of " + target + " to be accepted"
	}
	switch status {
	case contract.Cancelled:
		// A cancelled target that another Job continues was replaced by it, as a cancelled rework is in reworkDone.
		if len(g.reworksOf(target, map[string]bool{target: true})) == 0 {
			return false, "waiting for " + target + " to be accepted, but it was cancelled"
		}
		return g.reworkDone(s, target, map[string]bool{target: true})
	case contract.Rejected:
		return g.reworkDone(s, target, map[string]bool{target: true})
	}
	return true, ""
}

// reworkDone reports whether the rejected Job id is made good by its reworks. A
// rework continues id and is not one of id's own children (a proposed child
// continues its parent too); the app's Judge posts one when a review rejects,
// and numbers its review under the first build. Every rework counts, not any:
// each is accepted with its own children accepted, or was rejected and is made
// good by its own reworks in turn. seen ends a loop of continues.
func (g *Graph) reworkDone(s Snapshot, id string, seen map[string]bool) (bool, string) {
	reworks := g.reworksOf(id, seen)
	if len(reworks) == 0 {
		return false, "waiting for a Job that reworks " + id + ", which was rejected"
	}
	for _, r := range reworks {
		seen[r] = true
	}
	for _, r := range reworks {
		if !g.childrenAccepted(s, r) {
			return false, "waiting for all children of " + r + " to be accepted"
		}
		switch s.Jobs[r].Status {
		case contract.Accepted:
		case contract.Rejected:
			if ok, reason := g.reworkDone(s, r, seen); !ok {
				return false, reason
			}
		case contract.Cancelled:
			// A cancelled rework that another Job continues was replaced by it, which makes it good in turn.
			if len(g.reworksOf(r, seen)) == 0 {
				return false, "waiting for " + r + ", which reworks " + id + ", to be accepted, but it was cancelled"
			}
			if ok, reason := g.reworkDone(s, r, seen); !ok {
				return false, reason
			}
		default:
			return false, "waiting for " + r + ", which reworks " + id + ", to be accepted"
		}
	}
	return true, ""
}

// reworksOf are the Jobs that continue id and are not its own children, leaving
// out those in seen.
func (g *Graph) reworksOf(id string, seen map[string]bool) []string {
	reworks := []string{}
	for _, r := range g.ids {
		if c := g.items[r].Continues; c != nil && *c == id && !seen[r] && !strings.HasPrefix(r, id+".") {
			reworks = append(reworks, r)
		}
	}
	return reworks
}

// childrenAccepted reports whether every immediate child of id that holds an
// edge is accepted; a Job with no children has none waiting.
func (g *Graph) childrenAccepted(s Snapshot, id string) bool {
	for _, child := range g.holding(s, id) {
		if s.Jobs[child].Status != contract.Accepted {
			return false
		}
	}
	return true
}

// holding are the immediate children of id that hold a children-done edge: a
// cancelled person Job holds nothing once a decision Job replaced it and was
// accepted (replaced), and a cancelled review holds nothing once the decision Job
// for its lost verdict was accepted (lostDecided). Any other cancelled person Job,
// such as an approve card, still holds: an approval is the person Job accepted.
func (g *Graph) holding(s Snapshot, id string) []string {
	out := []string{}
	for _, child := range g.children[id] {
		if s.Jobs[child].Status == contract.Cancelled && (g.replaced(s, child) || g.lostDecided(s, child)) {
			continue
		}
		out = append(out, child)
	}
	return out
}

// decidePrefix starts the title of the Judge's old person Job and of the decision
// Job that replaces it ("Decide how <J> continues", judge.Person).
const decidePrefix = "Decide how "

// replaced reports whether p is a person Job of the old kind that judge.Migrate
// replaced with a decision Job, and that decision Job is accepted: a Job for an
// agent, titled as p is and discovered from the same Job (judge.decisionFrom).
func (g *Graph) replaced(s Snapshot, p string) bool {
	it := g.items[p]
	if !it.IsPerson() || it.DiscoveredFrom == nil || !strings.HasPrefix(it.Title, decidePrefix) {
		return false
	}
	for _, d := range g.ids {
		dec := g.items[d]
		if d != p && contract.NormalizeRole(dec.Role) == "orchestrator" && strings.HasPrefix(dec.Title, decidePrefix) &&
			dec.DiscoveredFrom != nil && *dec.DiscoveredFrom == *it.DiscoveredFrom && s.Jobs[d].Status == contract.Accepted {
			return true
		}
	}
	return false
}

// lostDecided reports whether r is a review whose verdict was lost and the Judge's
// decision Job for it is accepted: a decision Job discovered from the reviewed Job
// and titled as judge.lost titles it ("Decide how <J> continues without its review").
func (g *Graph) lostDecided(s Snapshot, r string) bool {
	it := g.items[r]
	if !isReview(it) || it.DiscoveredFrom == nil {
		return false
	}
	b := *it.DiscoveredFrom
	for _, d := range g.ids {
		dec := g.items[d]
		if isDecision(dec) && dec.Title == decidePrefix+b+" continues without its review" &&
			dec.DiscoveredFrom != nil && *dec.DiscoveredFrom == b && s.Jobs[d].Status == contract.Accepted {
			return true
		}
	}
	return false
}

// started is a Job that a worker holds or has handed in: it reads as Started or
// Needs review, or it is parked mid-run.
func started(status contract.Status) bool {
	switch status {
	case contract.Assigned, contract.Running, contract.Submitted, contract.Parked:
		return true
	}
	return false
}

// Ready lists unstarted posted/ready Jobs whose dependencies are satisfied.
// No children is a wait, not a vacuous success before a split has happened.
// A deliver Job also waits for every open person question (questionWaits).
// Already assigned, running, parked or terminal Jobs are never offered again.
func (g *Graph) Ready(s Snapshot) (Readiness, error) {
	out := Readiness{Ready: []string{}, Waiting: []Wait{}}
	if cycle := g.Cycle(); len(cycle) > 0 {
		return out, &CycleError{Path: cycle}
	}
	for _, id := range g.ids {
		status := s.Jobs[id].Status
		if status != "" && status != contract.Posted && status != contract.Ready {
			// A Job that has started is never held or stopped by a later
			// rejection of a Job it waited on; its Feed is told instead.
			if started(status) {
				for _, e := range g.items[id].After {
					if !strings.HasPrefix(e.Job, "gate:") && edgeKind(e) == Done && s.Jobs[e.Job].Status == contract.Rejected {
						out.Notes = append(out.Notes, Note{Job: id, Target: e.Job, Reason: e.Job + " was rejected after " + id + " started; " + id + " carries on"})
					}
				}
			}
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
				satisfied = Released(s.Jobs[e.Job])
				reason = DoneReason(e.Job, s.Jobs[e.Job].Status)
			case kind == Failed:
				status := s.Jobs[e.Job].Status
				satisfied = (status == contract.Rejected || status == contract.Cancelled) && s.Exhausted[e.Job]
				reason = "waiting for " + e.Job + " to fail with no attempt left"
			case kind == ChildrenDone:
				satisfied, reason = g.childrenDone(s, e.Job)
			case kind == Landed:
				satisfied = s.Landed[e.Job]
				reason = "waiting for " + e.Job + " to land"
			}
			if !satisfied {
				out.Waiting = append(out.Waiting, Wait{Job: id, Target: e.Job, Kind: kind, Reason: reason})
			}
		}
		if IsDeliver(g.items[id]) {
			out.Waiting = append(out.Waiting, g.questionWaits(s, id)...)
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
