package orchestrator

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/judge"
)

// jaOldPerson hands each open person Job of the old kind ("Decide how <J> continues") to an agent:
// the Judge's own plan posts the decision Job and cancels the person Job, so the owner never has to
// run grid judge person. A pass over a task with none posts nothing, and a repeat finds the decision
// Job already on the Feed. It reads no wake, so it does not depend on the old one being answered.
func (s Store) jaOldPerson(snap Snapshot) error {
	for {
		plan, ok := judge.Migrate(judge.State{Task: s.Feed.TaskID, Jobs: snap.State.Jobs, Bodies: snap.Bodies, Records: snap.Records, IDs: snap.JobIDs})
		if !ok {
			return nil
		}
		for _, p := range plan.Posts {
			var err error
			if p.Type == "control" {
				err = s.control(snap.State.Jobs[p.Job], contract.Cancelled, stringValue(p.Body["reason"]), "")
			} else {
				_, err = s.Feed.Post(feed.Input{Type: p.Type, Job: p.Job, Body: p.Body}, s.Writer)
			}
			if err != nil {
				return err
			}
		}
		var err error
		if snap, err = Load(s.Feed); err != nil {
			return err
		}
	}
}
