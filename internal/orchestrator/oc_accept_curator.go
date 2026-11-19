package orchestrator

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Continuous curation: the brain needs no attention, so proposals must not wait
// for deliver. After a pass in which a non-curator Job of an open task has been
// accepted, and no curator Job is open or newer than that acceptance, the pass
// asks ClosedCurator for a curator Job, as a closed task does. A curator Job
// that is accepted never earns another: only a newer non-curator acceptance
// does. This is coordination, not a limit: nothing caps or delays it.
//
// An answer of nothing waiting is kept, per task folder, with a key of what the
// answer depends on: the newest acceptance that earned the ask, and the name, size
// and mod time of every proposal file in the folders ClosedCurator reads. The next
// pass takes that key before the task lock and asks nothing while it is the same,
// so an idle task costs a listing and not a read of every proposal on every pass,
// and an unreadable proposal is named once per change. The key is taken before the
// ask, never after, so a proposal that lands during the ask shows in the next key.
// Nothing is clock-based: no timer, count or cap.

// ocAcceptedAfterCurator answers the id of the newest record that accepted a
// non-curator Job after the newest curator Job was posted (or there is no curator
// Job at all), "" when none did, and whether a curator Job is open.
func ocAcceptedAfterCurator(snap Snapshot) (earned string, open bool) {
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
	var earnedAt time.Time
	for id, j := range snap.State.Jobs {
		if curator[id] || j.Status != contract.Accepted || len(j.History) == 0 {
			continue
		}
		recID := j.History[len(j.History)-1]
		rec, ok := snap.Records[recID]
		if !ok {
			continue
		}
		at, err := time.Parse(time.RFC3339Nano, rec.At)
		if err != nil || !newest.IsZero() && !at.After(newest) {
			continue
		}
		// The jobs are a map: the newest, and the larger id on a tie, so the answer is one.
		if earned == "" || at.After(earnedAt) || at.Equal(earnedAt) && recID > earned {
			earned, earnedAt = recID, at
		}
	}
	return earned, open
}

// ocCuratorKey digests the acceptance that earned the ask and the proposal files in
// the task folder's brain-proposals and in each folder ProposalDirs names. A missing
// folder is a state of its own. It answers no key when a folder cannot be listed or a
// file has gone by the time it is looked at: the ask is then made as before.
func (s *Stream) ocCuratorKey(dir, earned string) ohSqKey {
	h := sha256.New()
	fmt.Fprintf(h, "earned\x00%s\x00", earned)
	folders := []string{filepath.Join(dir, "brain-proposals")}
	if s.ProposalDirs != nil {
		folders = append(folders, s.ProposalDirs(dir)...)
	}
	for _, folder := range folders {
		if !ocProposalsKey(h, folder) {
			return ohSqKey{}
		}
	}
	var k ohSqKey
	h.Sum(k.sum[:0])
	k.ok = true
	return k
}

// ocProposalsKey adds the listing of one proposals folder, the .md files the proposal
// reader opens, to h.
func ocProposalsKey(h hash.Hash, folder string) bool {
	entries, err := os.ReadDir(folder) // sorted by name
	if err != nil && !os.IsNotExist(err) {
		return false
	}
	fmt.Fprintf(h, "folder\x00%s\x00", folder)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return false
		}
		ohSqAdd(h, "proposal", e.Name(), info)
	}
	return true
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
	earned, open := ocAcceptedAfterCurator(snap)
	if earned == "" || open {
		return
	}
	key := s.ocCuratorKey(dir, earned)
	if key.ok && s.ocCuratorMemo[dir] == key {
		return
	}
	asked := false
	err = st.locked(func() error {
		snap, err := Load(st.Feed)
		if err != nil {
			return err
		}
		if earned, open := ocAcceptedAfterCurator(snap); earned == "" || open {
			return nil
		}
		if err := st.usPostCurator(snap, dir, taskID, s.ClosedCurator); err != nil {
			return err
		}
		asked = true
		return nil
	})
	if err != nil {
		s.diag("grid worker next: task %s: curator Job after an accepted Job: %s\n", taskID, oneLine(err))
		return
	}
	// Posted, nothing waiting, or a hook error that usPostCurator has already printed once:
	// none is asked again until the key differs. A failed lock or post is asked again.
	if asked && key.ok {
		if s.ocCuratorMemo == nil {
			s.ocCuratorMemo = map[string]ohSqKey{}
		}
		s.ocCuratorMemo[dir] = key
	}
}
