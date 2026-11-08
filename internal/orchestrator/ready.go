// Package orchestrator makes mechanical task moves without calling a model.
//
// All passes and command writes hold task.json shared before acquiring
// .orchestrator.lock exclusively. Worker lease checks occur inside that order;
// no Job lease is held while waiting for the pass lock. Task grants hold
// task.json exclusively and never wait for .orchestrator.lock. Within a pass,
// Feed posts use wake lock, lease, then tenure file; Job grants use Job lease,
// takeover log, then sequence counter. Question ID reservation uses the
// question store lock inside the pass; question-file paths never take a pass
// or task lease lock. Every lock uses internal/fslock.
package orchestrator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// Snapshot contains one folded Feed and its admitted records. The worker uses
// Ready on this same input to size its spawn demand.
type Snapshot struct {
	// Assignee scopes host capacity; empty computes dependency readiness only.
	Assignee string
	State    contract.State
	Records  map[string]feed.Record
	Items    []job.Job
	Bodies   map[string]map[string]any
	Created  map[string]time.Time
}

func Load(store feed.Store) (Snapshot, error) {
	allowed := store.WithdrawAllowed
	if allowed == nil {
		allowed = feed.QuestionWithdrawReadPolicy(store.TaskDir)
	}
	r, err := store.Read(feed.Filter{})
	if err != nil {
		return Snapshot{}, err
	}
	now := time.Now()
	if store.Now != nil {
		now = store.Now()
	}
	// The person layer keeps a non-person answer from releasing a person gate.
	state, _ := feed.FoldReadAt(r, now, allowed)
	s := Snapshot{State: state, Records: map[string]feed.Record{}, Bodies: map[string]map[string]any{}, Created: map[string]time.Time{}}
	counted := map[string]bool{}
	for _, v := range s.State.Verdicts {
		counted[v.ID] = v.Kind == "counted"
	}
	for _, e := range r.Records {
		if !counted[e.ID] {
			continue
		}
		s.Records[e.ID] = e.Record
		if e.Type == "job" {
			b, _ := json.Marshal(e.Body)
			var item job.Job
			if err := json.Unmarshal(b, &item); err != nil {
				return s, err
			}
			s.Items = append(s.Items, item)
			s.Bodies[item.ID] = e.Body
			created := stringValue(e.Body["created_at"])
			if created == "" {
				created = e.At
			}
			s.Created[item.ID], _ = time.Parse(time.RFC3339Nano, created)
		}
	}
	sort.Slice(s.Items, func(i, j int) bool { return s.Items[i].ID < s.Items[j].ID })
	return s, nil
}

// Ready is the only Orchestrator ready predicate, shared with spawn sizing.
// The graph owns dependency semantics and canonical ordering. Readiness times
// are metadata from the authority's controls, never authority for a transition.
func Ready(s Snapshot, now time.Time) (jobgraph.Readiness, error) {
	g, err := jobgraph.New(s.Items)
	if err != nil {
		return jobgraph.Readiness{}, err
	}
	input := jobgraph.Snapshot{Jobs: map[string]contract.Job{}, Exhausted: map[string]bool{}, OpenGates: map[string]bool{}, ReadySince: map[string]time.Time{}, CreatedAt: s.Created}
	settled := map[string]time.Time{}
	for id, j := range s.State.Jobs {
		input.Jobs[id] = *j
		input.Exhausted[id] = j.Status == contract.Cancelled || j.Status == contract.Rejected
		for _, rid := range j.History {
			r := s.Records[rid]
			if r.Type == "control" && r.Body["status"] == string(j.Status) {
				settled[id], _ = time.Parse(time.RFC3339Nano, r.At)
			}
		}
	}
	gateTimes := map[string]time.Time{}
	gateRanks := map[string]int{}
	for id, q := range s.State.Questions {
		r := s.Records[id]
		gate, _ := r.Body["gate"].(map[string]any)
		gid, _ := gate["id"].(string)
		if gid == "" {
			continue
		}
		key := "gate:" + gid
		rank := gateRank(stringValue(gate["kind"]))
		previous, exists := gateRanks[key]
		if exists && rank < previous {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, stringValue(gate["at"]))
		open := !q.Open
		switch gate["kind"] {
		case "timer":
			open = !at.IsZero() && !now.Before(at)
		case "person":
			open = open || stringValue(gate["default"]) != "" && !at.IsZero() && !now.Before(at)
		case "job":
			j := s.State.Jobs[stringValue(gate["job"])]
			open = j != nil && string(j.Status) == stringValue(gate["state"])
			if j != nil {
				at = settled[j.ID]
			}
		}
		if q.Applied != nil {
			at, _ = time.Parse(time.RFC3339Nano, s.Records[*q.Applied].At)
		}
		// Old Feeds can contain duplicate names. Strictest kind wins; ties
		// require every equally strict question to open. AND and max keep
		// both readiness and its timestamp independent of map iteration.
		if exists && rank == previous {
			open = open && input.OpenGates[key]
			if gateTimes[key].After(at) {
				at = gateTimes[key]
			}
		}
		gateRanks[key] = rank
		input.OpenGates[key] = open
		gateTimes[key] = at
	}
	for _, item := range s.Items {
		readyAt := s.Created[item.ID]
		for _, e := range item.After {
			at := settled[e.Job]
			if strings.HasPrefix(e.Job, "gate:") {
				if _, ok := input.OpenGates[e.Job]; !ok {
					return jobgraph.Readiness{}, fmt.Errorf("%s: gate %s does not exist", item.ID, e.Job)
				}
				at = gateTimes[e.Job]
			} else if e.Kind == jobgraph.ChildrenDone {
				for child, t := range settled {
					if strings.HasPrefix(child, e.Job+".") && t.After(at) {
						at = t
					}
				}
			}
			if at.After(readyAt) {
				readyAt = at
			}
		}
		input.ReadySince[item.ID] = readyAt
	}
	out, err := g.Ready(input)
	if err != nil {
		return out, err
	}
	if s.Assignee != "" {
		for _, j := range s.State.Jobs {
			if j.Status == contract.Parked && j.Reason == "usage_limit" && j.Assign != nil && host(j.Assign.To) == host(s.Assignee) {
				for _, id := range out.Ready {
					out.Waiting = append(out.Waiting, jobgraph.Wait{Job: id, Reason: "usage_limit"})
				}
				out.Ready = []string{}
				break
			}
		}
	}

	if s.Assignee != "" {
		slots := 3
		review := func(j *contract.Job) bool {
			return (contract.NormalizeRole(j.Role) == "reviewer" || contract.NormalizeRole(j.Role) == "checker") && (j.Review == "L2" || j.Review == "L3")
		}
		for _, j := range s.State.Jobs {
			if review(j) && j.Assign != nil && host(j.Assign.To) == host(s.Assignee) && (j.Status == contract.Assigned || j.Status == contract.Running) {
				slots--
			}
		}
		keep := out.Ready[:0]
		for _, id := range out.Ready {
			j := s.State.Jobs[id]
			if review(j) {
				if slots <= 0 {
					out.Waiting = append(out.Waiting, jobgraph.Wait{Job: id, Reason: "review_capacity"})
					continue
				}
				slots--
			}
			keep = append(keep, id)
		}
		out.Ready = keep
	}
	return out, nil
}
func stringValue(v any) string { s, _ := v.(string); return s }
func ids(s Snapshot) []string {
	out := make([]string, 0, len(s.State.Jobs))
	for id := range s.State.Jobs {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func host(worker string) string {
	if i := strings.LastIndexByte(worker, '@'); i >= 0 {
		return worker[i+1:]
	}
	return worker
}
