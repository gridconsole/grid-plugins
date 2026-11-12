package orchestrator

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A task whose Jobs are all closed is not read again until its Feed folder changes.
//
// Stream.task used to Load every task of the home on every pass, and Load takes
// the exclusive feed/.cache/reader.lock, decodes the whole reader cache and writes
// it back, only to find a task with no open Job and no held lease and drop it. For
// a home of old tasks that is a full read of each of them every second, under the
// lock the hooks take.
//
// The Stream now keeps, per task folder, the key of the Feed folder it saw on the
// pass that dropped the task. A pass that finds the same key drops the task the
// same way without a Load. The key is what the reader and the lease check look at:
// the name, size and mod time of every feed/*.jsonl and of every entry under
// feed/lease/. feed/.cache/ is not in it, as Load writes there itself.
//
// The key is taken before Load, never after. A record or lease that lands after
// the key was taken makes the next key differ, so the next pass reads it; a key
// taken after Load could hide a record Load had not seen. A memo is kept only for a
// pass that saw at least one Job: a task with no Job record is the first-Job path
// (wlFirstJob), which also reads task.md, and task.md is not in the key.
//
// The memo adds no limit, timer or count: every task whose key differs, and every
// task with a pending receipt, is read as before, on every pass.

// ohSqKey is the digest of the Feed folder listing; the zero value is no key.
type ohSqKey struct {
	sum [sha256.Size]byte
	ok  bool
}

// ohSqFeedKey lists the Feed files the pass depends on and digests their names,
// sizes and mod times. It answers no key when a file cannot be listed or has gone
// by the time it is looked at: the task is then read as before.
func ohSqFeedKey(fs feed.Store) ohSqKey {
	dir := fs.Dir()
	h := sha256.New()
	logs, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return ohSqKey{}
	}
	sort.Strings(logs)
	for _, p := range logs {
		info, err := os.Stat(p) // the reader opens it, so a link is followed
		if err != nil {
			return ohSqKey{}
		}
		ohSqAdd(h, "feed", filepath.Base(p), info)
	}
	leases, err := os.ReadDir(filepath.Join(dir, "lease")) // sorted by name
	if err != nil && !os.IsNotExist(err) {
		return ohSqKey{}
	}
	for _, e := range leases {
		info, err := e.Info()
		if err != nil {
			return ohSqKey{}
		}
		ohSqAdd(h, "lease", e.Name(), info)
	}
	var k ohSqKey
	h.Sum(k.sum[:0])
	k.ok = true
	return k
}

func ohSqAdd(h hash.Hash, kind, name string, info os.FileInfo) {
	fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d\x00", kind, name, info.Size(), info.ModTime().UnixNano())
}

// ohSqCheck is the step of Stream.task before Load. It answers the key of the Feed
// folder as it is now and whether the task is quiet: the memo of dir holds the
// same key, and no receipt is pending. A key that differs, no key, or a pending
// receipt clears the memo and answers not quiet.
func (s *Stream) ohSqCheck(dir string, fs feed.Store, pending bool) (key ohSqKey, quiet bool) {
	if pending {
		delete(s.ohSqMemo, dir)
		return ohSqKey{}, false
	}
	key = ohSqFeedKey(fs)
	if prev, ok := s.ohSqMemo[dir]; ok && key.ok && prev == key {
		return key, true
	}
	delete(s.ohSqMemo, dir)
	return key, false
}

// ohSqStore keeps key for dir at the return of Stream.task that dropped the task
// (no open Job, no held lease, no receipt pending), when the pass saw at least one
// Job and the key could be taken.
func (s *Stream) ohSqStore(dir string, key ohSqKey, snap Snapshot) {
	if !key.ok || len(snap.State.Jobs) == 0 {
		return
	}
	if s.ohSqMemo == nil {
		s.ohSqMemo = map[string]ohSqKey{}
	}
	s.ohSqMemo[dir] = key
}
