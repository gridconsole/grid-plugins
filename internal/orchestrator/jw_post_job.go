package orchestrator

import "github.com/gridconsole/grid-plugins/internal/feed"

// PostJobLocked writes a `job` record from `grid feed post --type job` or the MCP
// feed_post tool under .orchestrator.lock, the lock `grid job post`, PostPerson
// and a holder pass take, so a pass that reads State under it never sees a Job
// appear halfway through.
//
// It keeps the order the package comment states and picks the way AnswerPerson
// does. A writer with a lease goes in through Store.locked: the lease file first
// (shared), then the pass lock. A writer with none goes in through exclusive, the
// pass lock alone. Both callers take that choice from the writer they build: the
// CLI from --lease or GRID_LEASE, the MCP tool from GRID_LEASE. The Job-lease
// writers the two front ends refuse (refuseJobSide) never get here.
//
// It is not PostPerson: it checks no after-target, so a seed that is held behind
// a record not yet on the Feed is accepted as it was, and it writes the record
// as given, with no id picked. It only holds the lock around store.Post.
func PostJobLocked(store feed.Store, in feed.Input, w feed.Writer) (record feed.Record, err error) {
	s := Store{Feed: store, Writer: w}
	lock := s.exclusive
	if w.Lease != "" {
		lock = s.locked
	}
	err = lock(func() error {
		var e error
		record, e = store.Post(in, w)
		return e
	})
	return record, err
}
