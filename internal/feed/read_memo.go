package feed

import (
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// A read of a Feed folder that has not changed is answered from the last read.
//
// Every read decodes the whole reader cache and sorts and validates every record,
// and the stream's pass over an open task makes about a dozen of them, so an idle
// pass over a Feed of hundreds of files cost seconds of CPU. What a read answers
// depends on the Feed folder alone: the name, size and mod time of every
// feed/*.jsonl, of every entry under feed/lease/ (the takeover log among them) and
// of every file under feed/bodies/ (a record that waits for its body). The key of
// that listing is taken before the read, so a record that lands after it shows in
// the next key and is read then. feed/.cache/ is not in the key, as a read writes
// there itself. A per-post board has its own reader and is never kept.
//
// Nothing here is clock-based: a fold that takes the clock (a question that is
// due) still folds the kept records again at the time of its own call.

type readKey struct {
	sum [sha256.Size]byte
	ok  bool
}

type readMemoEntry struct {
	key readKey
	out ReadResult
}

var (
	readMemoMu sync.Mutex
	readMemo   = map[string]readMemoEntry{} // Feed folder to its last read
)

// readKey digests the Feed folder listing; it answers no key when a file cannot be
// listed or has gone by the time it is looked at, and the Feed is then read as before.
func (s Store) readKey() readKey {
	if s.Board != nil || BoardPresent(s.TaskDir) {
		return readKey{}
	}
	dir := s.Dir()
	h := sha256.New()
	logs, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return readKey{}
	}
	sort.Strings(logs)
	for _, p := range logs {
		info, err := os.Stat(p) // the reader opens it, so a link is followed
		if err != nil {
			return readKey{}
		}
		readKeyAdd(h, "feed", filepath.Base(p), info)
	}
	if info, err := os.Stat(filepath.Join(dir, ".cache", "reader.json")); err == nil {
		readKeyAdd(h, "cache", "reader.json", info)
	}
	for _, sub := range []string{"lease", "bodies"} {
		entries, err := os.ReadDir(filepath.Join(dir, sub)) // sorted by name
		if err != nil && !os.IsNotExist(err) {
			return readKey{}
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				return readKey{}
			}
			readKeyAdd(h, sub, e.Name(), info)
		}
	}
	var k readKey
	h.Sum(k.sum[:0])
	k.ok = true
	return k
}

func readKeyAdd(h hash.Hash, kind, name string, info os.FileInfo) {
	fmt.Fprintf(h, "%s\x00%s\x00%d\x00%d\x00", kind, name, info.Size(), info.ModTime().UnixNano())
}

// readMemoGet answers a copy of the kept read of dir when its key is key.
func readMemoGet(dir string, key readKey) (ReadResult, bool) {
	if !key.ok {
		return ReadResult{}, false
	}
	readMemoMu.Lock()
	e, ok := readMemo[dir]
	readMemoMu.Unlock()
	if !ok || e.key != key {
		return ReadResult{}, false
	}
	return copyRead(e.out), true
}

func readMemoPut(dir string, key readKey, out ReadResult) {
	readMemoMu.Lock()
	defer readMemoMu.Unlock()
	if !key.ok {
		delete(readMemo, dir)
		return
	}
	readMemo[dir] = readMemoEntry{key, copyRead(out)}
}

func readMemoDrop(dir string) {
	readMemoMu.Lock()
	delete(readMemo, dir)
	readMemoMu.Unlock()
}

// copyRead copies the record list and the bad counts, which a caller may change.
func copyRead(r ReadResult) ReadResult {
	out := r
	out.Records = append([]Entry(nil), r.Records...)
	out.Bad = make(map[string]int, len(r.Bad))
	for k, v := range r.Bad {
		out.Bad[k] = v
	}
	return out
}
