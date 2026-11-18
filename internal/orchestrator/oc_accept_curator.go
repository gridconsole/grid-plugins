package orchestrator

import (
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Continuous curation: the brain needs no attention, so proposals must not wait
// for deliver. After a pass in which a non-curator Job of an open task has been
// accepted, and no curator Job is open or newer than that acceptance, the pass
// asks ClosedCurator for a curator Job, as a closed task does. A curator Job
// that is accepted never earns another: only a newer non-curator acceptance
// does. This is coordination, not a limit: nothing caps or delays it.

// ocAcceptedAfterCurator reports whether a non-curator Job was accepted after the
// newest curator Job was posted (or there is no curator Job at all), and whether
// a curator Job is open.
func ocAcceptedAfterCurator(snap Snapshot) (earned, open bool) {
	curator := usCuratorJobs(snap)
	var newest time.Time // when the newest curator Job was posted
	for id := range curator {
		if j := snap.State.Jobs[id]; j != nil && j.Status != contract.Accepted && j.Status != contract.Cancelled && j.Status != contract.Rejected {
			open = true
		}
		if at := snap.Created[id]; at.After(newest) {
			newest = at
		}
	}
	for id, j := range snap.State.Jobs {
		if curator[id] || j.Status != contract.Accepted || len(j.History) == 0 {
			continue
		}
		rec, ok := snap.Records[j.History[len(j.History)-1]]
		if !ok {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, rec.At)
		if err == nil && (newest.IsZero() || at.After(newest)) {
			earned = true
		}
	}
	return earned, open
}

// ocPostCurator posts the curator Job of an open task after an acceptance, under
// the task lock and lease the pass holds. It reads the Feed again inside the lock,
// so two passes cannot post two. An error is one Diagnostics line.
func (s *Stream) ocPostCurator(dir, taskID string, st Store) {
	if s.ClosedCurator == nil {
		return
	}
	snap, err := Load(st.Feed)
	if err != nil {
		return
	}
	if earned, open := ocAcceptedAfterCurator(snap); !earned || open {
		return
	}
	err = st.locked(func() error {
		snap, err := Load(st.Feed)
		if err != nil {
			return err
		}
		if earned, open := ocAcceptedAfterCurator(snap); !earned || open {
			return nil
		}
		return st.usPostCurator(snap, dir, taskID, s.ClosedCurator)
	})
	if err != nil {
		s.diag("grid worker next: task %s: curator Job after an accepted Job: %s\n", taskID, oneLine(err))
	}
}
