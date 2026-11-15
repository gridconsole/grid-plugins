package orchestrator

import (
	"errors"
	"fmt"

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

// guestAnswerer reports whether w is a guest writing without a lease: the one
// writer outside the team that may answer a person Job, and only the Job that
// names it (guestAddressed). PostPerson stays with PersonWriter.
func guestAnswerer(w feed.Writer) bool {
	return w.Lease == "" && w.Fence == 0 && contract.NormalizeRole(w.Author.Role) == "guest"
}

// guestAddressed reports whether Job id is a person Job whose intent.owner is w.
func guestAddressed(snap Snapshot, w feed.Writer, id string) bool {
	for _, it := range snap.Items {
		if it.ID == id && it.IsPerson() {
			return contract.GuestAddressee(w.Author.Role, w.Author.ID, w.Lease != "" || w.Fence != 0, it.Role, it.PersonName(), it.Kind)
		}
	}
	return false
}

var errPersonOnly = &feed.InvalidRecordError{Err: errors.New("only a person of the team posts a Job without the task lease")}

// PostPerson writes a person's Job post without a lease, at fence zero like an
// answer to a question, so a held task lease does not stop it. The post is a
// plain `job` record: the fold counts it and the holder's next pass assigns it.
// It takes the lock a tick takes, so the after-targets are checked against the
// Feed that pass will read.
func PostPerson(store feed.Store, w feed.Writer, in feed.Input) (record feed.Record, err error) {
	record, _, err = PostPersonUnless(store, w, in, nil)
	return record, err
}

// PostPersonUnless is PostPerson with a check made under .orchestrator.lock, in
// the same pass as the write, as WriteUnless makes it for a leased write: when
// done returns true nothing is written, the record is empty and the bool is
// true. A check made before the lock could be overtaken by a write from another
// process, so a caller that must not write twice asks here. A nil done never
// stops the post.
//
// A person's cancel request (a `control` with status cancelled) goes the same
// way: the holder's next pass writes the leased cancel (personCancels). It is
// refused under the lock when the task has no such Job or the Job is already
// closed, with a plain error, so the CLI exits 1.
func PostPersonUnless(store feed.Store, w feed.Writer, in feed.Input, done func() bool) (record feed.Record, skipped bool, err error) {
	cancel := in.Type == "control" && stringValue(in.Body["status"]) == string(contract.Cancelled)
	if !PersonWriter(w) || in.Type != "job" && !cancel {
		return record, false, errPersonOnly
	}
	err = (Store{Feed: store, Writer: w}).exclusive(func() error {
		if done != nil && done() {
			skipped = true
			return nil
		}
		snap, e := Load(store)
		if e != nil {
			return e
		}
		if cancel {
			target := stringValue(in.Body["target"])
			j := snap.State.Jobs[target]
			if j == nil {
				return fmt.Errorf("grid job cancel: no Job %s in this task", target)
			}
			if j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
				return fmt.Errorf("grid job cancel: %s is already %s", target, j.Status)
			}
		} else if e := checkAfterTargets(snap, in); e != nil {
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
