package orchestrator

import (
	"errors"
	"fmt"
	"os"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// taCancelEndGrants ends the grant of every Cancelled Job whose lease file still
// names a holder. A cancel is a control record; nothing else read its status to
// end lease/<job>.json, so write.go refused every post of the Job's child with
// ErrStaleLease while the app worker, which stops a child only when WithLease
// reports the lease not matched, let it run on. TaskStore.EndJob writes the
// takeover line for `<holder>.<fence>.jsonl` and clears the holder, so the worker
// sees the lease end and stops the child.
//
// It runs inside the tick, which holds the task lease and the orchestrator lock,
// the same place assign calls GrantJob. A Job with no lease file (never granted,
// a person Job) or no holder (already ended) is left alone, and EndJob is
// idempotent, so a repeat tick or a crash between the takeover line and the
// rewrite changes nothing the second time. Accepted and Rejected Jobs are not
// touched here, and nothing is refused, held or counted: it ends a grant and
// posts nothing.
func (s Store) taCancelEndGrants(snap Snapshot) error {
	ts := lease.TaskStore{Feed: s.Feed, Now: s.Now}
	for _, id := range ids(snap) {
		if j := snap.State.Jobs[id]; j == nil || j.Status != contract.Cancelled {
			continue
		}
		t, err := ts.Read(id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read the grant of cancelled Job %s: %w", id, err)
		}
		if t.Holder == nil {
			continue
		}
		if err := ts.EndJob(s.Writer, id); err != nil {
			return fmt.Errorf("end the grant of cancelled Job %s: %w", id, err)
		}
	}
	return nil
}
