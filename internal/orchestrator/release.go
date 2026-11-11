package orchestrator

import (
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

// noteStartedDependents posts a note to the Feed of each Job that started and
// then lost a Job it waited on to a rejection. The note is information: the Job
// is not held, stopped or reassigned. The note itself is the durable marker, so
// a later pass, a handover or lost local memory never posts it twice.
func (s Store) noteStartedDependents(snap Snapshot, notes []jobgraph.Note) error {
	said := map[string]bool{}
	for _, r := range snap.Records {
		if r.Type == "post" && r.Job != nil && r.Body["kind"] == "message" {
			said[*r.Job+"\n"+stringValue(r.Body["text"])] = true
		}
	}
	for _, n := range notes {
		if said[n.Job+"\n"+n.Reason] {
			continue
		}
		in := feed.Input{Type: "post", Job: n.Job, Body: map[string]any{"ref": "task", "kind": "message", "text": n.Reason}}
		if _, err := s.Feed.Post(in, s.Writer); err != nil {
			return err
		}
	}
	return nil
}
