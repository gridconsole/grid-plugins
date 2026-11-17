package postfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// Dir is one task's board: the board/ folder in the brain and the machine's cache
// folder for that task, which holds the board lock, the seq counter and the names
// cache. Rel is the task folder's path from the brain root, in slashes, for links.
// MkdirAll and WriteFile write in the cache folder; nil is 0700 folders and 0600
// files by atomic rename.
type Dir struct {
	Board     string
	Cache     string
	Rel       string
	MkdirAll  func(path string) error
	WriteFile func(path string, data []byte) error
}

// The cache folder's files.
const (
	lockName  = "board.lock"
	seqName   = "board.seq"
	namesName = "board.cache.json"
	tmpName   = ".tmp"
)

// tmpAge is how old a temp file in board/.tmp/ is before the next poster removes it
// (F2): a temp that old belongs to a poster that stopped between write and link.
const tmpAge = time.Hour

// link publishes a temp file under its final name. It is a variable so a test can
// count the links or take them away (F12).
var link = os.Link

// Own is a post's path from the brain root.
func (d Dir) Own(name string) string { return path.Join(d.Rel, "board", name) }

func (d Dir) mkdir(p string) error {
	if d.MkdirAll != nil {
		return d.MkdirAll(p)
	}
	return os.MkdirAll(p, 0o700)
}

func (d Dir) write(p string, data []byte) error {
	if d.WriteFile != nil {
		return d.WriteFile(p, data)
	}
	return atomicfile.Write(p, data, 0o600)
}

// Locked is the board lock held. It orders this machine's own writers; it is not a
// lease, and a person's edit of a post never waits on it.
type Locked struct {
	d Dir
	f *os.File
}

// Lock takes the board lock, waiting until ctx is done.
func (d Dir) Lock(ctx context.Context) (*Locked, error) {
	if err := d.mkdir(d.Cache); err != nil {
		return nil, err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(d.Cache, lockName), os.O_CREATE|os.O_RDWR, 0o660)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusiveContext(ctx, f); err != nil {
		f.Close()
		return nil, err
	}
	return &Locked{d: d, f: f}, nil
}

// Unlock releases the board lock.
func (l *Locked) Unlock() {
	fslock.Unlock(l.f)
	l.f.Close()
}

// names lists the post file names in board/: every name ending in .md that does not
// start with a dot.
func (d Dir) names() ([]string, error) {
	f, err := os.Open(d.Board)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	all, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, n := range all {
		if !strings.HasPrefix(n, ".") && strings.HasSuffix(n, ".md") {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out, nil
}

// nextSeq is one past both the counter and the highest seq among the names, so a post
// that arrived by sync is never numbered over.
func (l *Locked) nextSeq() (int64, error) {
	var top int64
	if b, err := os.ReadFile(filepath.Join(l.d.Cache, seqName)); err == nil {
		top, _ = strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	}
	names, err := l.d.names()
	if err != nil {
		return 0, err
	}
	for _, n := range names {
		if s, ok := NameSeq(n); ok && s > top {
			top = s
		}
	}
	return top + 1, nil
}

// sweep removes temp files older than tmpAge from board/.tmp/ (F2).
func (l *Locked) sweep(now time.Time) {
	dir := filepath.Join(l.d.Board, tmpName)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		info, err := e.Info()
		if err == nil && info.Mode().IsRegular() && now.Sub(info.ModTime()) > tmpAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// Publish writes a post under the next free seq and answers its header and name. It
// writes board/.tmp/<name>.*, then links it to board/<name>; a name that exists
// already takes the next seq. Where the folder has no hard links it creates the final
// name with O_EXCL, writes and syncs it (F12). names gives the file names of the
// posts the new one links to.
func (l *Locked) Publish(h Header, names map[string]string) (Header, string, error) {
	l.sweep(time.Now())
	seq, err := l.nextSeq()
	if err != nil {
		return h, "", err
	}
	tmpDir := filepath.Join(l.d.Board, tmpName)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return h, "", err
	}
	for {
		h.Seq = seq
		name := Name(seq, h.Job, h.Type)
		data, err := l.d.file(h, name, names)
		if err != nil {
			return h, "", err
		}
		done, err := l.place(tmpDir, name, data)
		if err != nil {
			return h, "", err
		}
		if !done {
			seq++
			continue
		}
		_ = atomicfile.SyncDir(l.d.Board)
		if err := l.d.write(filepath.Join(l.d.Cache, seqName), []byte(strconv.FormatInt(seq, 10)+"\n")); err != nil {
			return h, name, err
		}
		return h, name, nil
	}
}

// file is the whole post file: header and rendering.
func (d Dir) file(h Header, name string, names map[string]string) ([]byte, error) {
	head, err := h.Encode()
	if err != nil {
		return nil, err
	}
	return append(head, Render(h, d.Own(name), names)...), nil
}

// place publishes data as board/<name>. It answers false when the name exists.
func (l *Locked) place(tmpDir, name string, data []byte) (bool, error) {
	final := filepath.Join(l.d.Board, name)
	tmp, err := os.CreateTemp(tmpDir, name+".*")
	if err != nil {
		return false, err
	}
	_, werr := tmp.Write(data)
	if werr == nil {
		werr = tmp.Sync()
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	defer os.Remove(tmp.Name())
	if werr != nil {
		return false, werr
	}
	err = link(tmp.Name(), final)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	// No hard links here: create the final name exclusively (F12).
	f, err := os.OpenFile(final, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, werr = f.Write(data)
	if werr == nil {
		werr = f.Sync()
	}
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	return werr == nil, werr
}

// Post is one post file as listed: its name, header and the sha256 of its bytes when
// this machine first read it.
type Post struct {
	Name   string
	Header Header
	Sum    string
}

// Listing is the posts of a board, sorted by name, and the files whose header could
// not be read this pass, with why. A bad file is not cached, so the next pass reads it
// again (F3).
type Listing struct {
	Posts []Post
	Bad   map[string]string
}

type namesCache struct {
	V     int                   `json:"v"`
	Posts map[string]cachedPost `json:"posts"`
}

type cachedPost struct {
	Sum    string `json:"sum"`
	Header string `json:"header"`
}

func (d Dir) loadCache() namesCache {
	c := namesCache{V: 1, Posts: map[string]cachedPost{}}
	b, err := os.ReadFile(filepath.Join(d.Cache, namesName))
	if err != nil {
		return c
	}
	var got namesCache
	if json.Unmarshal(b, &got) != nil || got.V != 1 || got.Posts == nil {
		return c
	}
	return got
}

func (d Dir) saveCache(c namesCache) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := d.mkdir(d.Cache); err != nil {
		return err
	}
	return d.write(filepath.Join(d.Cache, namesName), b)
}

// List reads the board: the names in board/, each known name from the names cache and
// each new name from its file. Posts never change, so a known name is not read again;
// Check is the pass that looks at the files.
func (d Dir) List() (Listing, error) { return d.list(true) }

// Scan is List without writing the names cache: a new name is read from its file and
// not kept, for a reader that must not change the machine folder.
func (d Dir) Scan() (Listing, error) { return d.list(false) }

func (d Dir) list(save bool) (Listing, error) {
	out := Listing{Bad: map[string]string{}}
	names, err := d.names()
	if err != nil {
		return out, err
	}
	c := d.loadCache()
	changed := false
	for _, n := range names {
		if cp, ok := c.Posts[n]; ok {
			if h, _, err := Decode([]byte(cp.Header)); err == nil {
				out.Posts = append(out.Posts, Post{Name: n, Header: h, Sum: cp.Sum})
				continue
			}
		}
		data, err := os.ReadFile(filepath.Join(d.Board, n))
		if err != nil {
			out.Bad[n] = err.Error()
			continue
		}
		h, md, err := Decode(data)
		if err != nil {
			out.Bad[n] = err.Error()
			continue
		}
		sum := sha256.Sum256(data)
		cp := cachedPost{Sum: hex.EncodeToString(sum[:]), Header: string(data[:len(data)-len(md)])}
		if old, ok := c.Posts[n]; ok && old.Sum != "" {
			cp.Sum = old.Sum
		}
		c.Posts[n] = cp
		changed = true
		out.Posts = append(out.Posts, Post{Name: n, Header: h, Sum: cp.Sum})
	}
	if changed && save {
		if err := d.saveCache(c); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Finding is a post that is no longer the file this machine first read (F6).
type Finding struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// Check compares every post this machine has read with the file as it is now. A file
// whose bytes differ is an "edited post", and its header is read again so the fold
// counts the file as it is now; a file that is gone is a "deleted post" and drops out
// of the fold. Neither is refused or undone: the person's edit stays.
func (d Dir) Check() ([]Finding, error) {
	c := d.loadCache()
	var out []Finding
	changed := false
	names := make([]string, 0, len(c.Posts))
	for n := range c.Posts {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		cp := c.Posts[n]
		data, err := os.ReadFile(filepath.Join(d.Board, n))
		if errors.Is(err, fs.ErrNotExist) {
			out = append(out, Finding{Name: n, Kind: "deleted post"})
			continue
		}
		if err != nil {
			return out, err
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) == cp.Sum {
			continue
		}
		out = append(out, Finding{Name: n, Kind: "edited post"})
		_, md, err := Decode(data)
		if err != nil {
			delete(c.Posts, n)
			changed = true
			continue
		}
		if head := string(data[:len(data)-len(md)]); head != cp.Header {
			cp.Header = head
			c.Posts[n] = cp
			changed = true
		}
	}
	if changed {
		return out, d.saveCache(c)
	}
	return out, nil
}

// Renumber gives the posts named in move the next free seqs, in seq order: the same
// id, header and body under a new name and seq (F4). Each moved post and each post in
// unaccepted that links to a moved one is rendered again, so its links name the new
// files. The sync server has accepted none of them, so no other machine has read them.
// It answers the new name of each moved name.
func (l *Locked) Renumber(move, unaccepted []string) (map[string]string, error) {
	listing, err := l.d.List()
	if err != nil {
		return nil, err
	}
	byName := map[string]Header{}
	names := map[string]string{}
	for _, p := range listing.Posts {
		byName[p.Name] = p.Header
		names[p.Header.ID] = p.Name
	}
	sorted := append([]string{}, move...)
	sort.Slice(sorted, func(i, j int) bool {
		a, _ := NameSeq(sorted[i])
		b, _ := NameSeq(sorted[j])
		return a < b
	})
	moved := map[string]string{}
	movedID := map[string]bool{}
	for _, old := range sorted {
		h, ok := byName[old]
		if !ok {
			return moved, fmt.Errorf("renumber: no post %s", old)
		}
		_, name, err := l.Publish(h, names)
		if err != nil {
			return moved, err
		}
		if err := os.Remove(filepath.Join(l.d.Board, old)); err != nil {
			return moved, err
		}
		moved[old] = name
		names[h.ID] = name
		movedID[h.ID] = true
	}
	c := l.d.loadCache()
	for old := range moved {
		delete(c.Posts, old)
	}
	for _, n := range unaccepted {
		h, ok := byName[n]
		if !ok || moved[n] != "" || !links(h, movedID) {
			continue
		}
		data, err := l.d.file(h, n, names)
		if err != nil {
			return moved, err
		}
		if err := atomicfile.Write(filepath.Join(l.d.Board, n), data, 0o600); err != nil {
			return moved, err
		}
		delete(c.Posts, n)
	}
	return moved, l.d.saveCache(c)
}

func links(h Header, ids map[string]bool) bool {
	if ids[h.Re] || ids[h.For] {
		return true
	}
	for _, a := range h.After {
		if ids[a] {
			return true
		}
	}
	return false
}
