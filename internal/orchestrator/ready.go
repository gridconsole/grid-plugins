// Package orchestrator makes mechanical task moves without calling a model.
package orchestrator

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// Snapshot contains one folded Feed and its admitted records. The worker uses
// Ready on this same input to size its spawn demand.
type Snapshot struct {
	State   contract.State
	Records map[string]feed.Record
	Items   []jobgraph.Item
	Bodies  map[string]map[string]any
	Created map[string]time.Time
}

func Load(store feed.Store) (Snapshot, error) {
	r, err := store.Read(feed.Filter{})
	if err != nil {
		return Snapshot{}, err
	}
	s := Snapshot{State: feed.FoldRead(r), Records: map[string]feed.Record{}, Bodies: map[string]map[string]any{}, Created: map[string]time.Time{}}
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
			var item jobgraph.Item
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
	for id, q := range s.State.Questions {
		r := s.Records[id]
		gate, _ := r.Body["gate"].(map[string]any)
		gid, _ := gate["id"].(string)
		if gid == "" {
			continue
		}
		key := "gate:" + gid
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
	return g.Ready(input)
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
