package orchestrator

import (
	"errors"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// PersonWriter reports whether w is a person of the team writing without a
// lease: the one writer whose Job post and person-Job answer are taken unleased.
// It reads only the stamped role. Agents are not in it, and the record schemas
// do not admit a worker or agent author for a Job in any case.
func PersonWriter(w feed.Writer) bool {
	return w.Lease == "" && contract.TierFor(w.Author.Role) == contract.PersonTeam
}

var errPersonOnly = &feed.InvalidRecordError{Err: errors.New("only a person of the team posts a Job without the task lease")}

// PostPerson writes a person's Job post without a lease, at fence zero like an
// answer to a question, so a held task lease does not stop it. The post is a
// plain `job` record: the fold counts it and the holder's next pass assigns it.
// It takes the lock a tick takes, so the after-targets are checked against the
// Feed that pass will read.
func PostPerson(store feed.Store, w feed.Writer, in feed.Input) (record feed.Record, err error) {
	if !PersonWriter(w) || in.Type != "job" {
		return record, errPersonOnly
	}
	err = (Store{Feed: store, Writer: w}).exclusive(func() error {
		snap, e := Load(store)
		if e != nil {
			return e
		}
		if e := checkAfterTargets(snap, in); e != nil {
			return e
		}
		if in.ID == "" {
			if in.ID, e = feed.NewID(); e != nil {
				return e
			}
		}
		record, e = store.Post(in, w)
		return e
	})
	return
}
