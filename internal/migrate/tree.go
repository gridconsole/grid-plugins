package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// entry is what the migration knows about one path. Scanning keeps metadata only:
// file bytes are streamed when they are copied or hashed, never held. Data is set
// only for the few small files the migration rewrites or creates, and Src names the
// file a verbatim copy reads from.
type entry struct {
	Mode    fs.FileMode
	Size    int64
	ModTime int64  // unix nanoseconds, regular files only
	Link    string // target of a symlink, which is kept as a link and never followed
	Data    []byte
	Src     string
}
type tree map[string]entry

// snap is a scanned folder: its entries and their sorted paths, so the paths below
// one folder are a contiguous range.
type snap struct {
	m    tree
	keys []string
}

func (s snap) get(rel string) (entry, bool) { e, ok := s.m[rel]; return e, ok }

// below lists the paths under prefix, not prefix itself. "" is the whole tree.
func (s snap) below(prefix string) []string {
	if prefix == "" || prefix == "." {
		out := make([]string, 0, len(s.keys))
		for _, k := range s.keys {
			if k != "." {
				out = append(out, k)
			}
		}
		return out
	}
	lo := sort.SearchStrings(s.keys, prefix+"/")
	hi := sort.SearchStrings(s.keys, prefix+"0") // "0" is the byte after "/"
	return s.keys[lo:hi]
}

func isSpecial(m fs.FileMode) bool {
	return m&(fs.ModeSocket|fs.ModeNamedPipe|fs.ModeDevice|fs.ModeCharDevice|fs.ModeIrregular) != 0
}

// snapshot walks root with lstat: a symlink is recorded with its target and not
// followed, and no file is opened.
func snapshot(root string) (snap, error) {
	out := tree{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		e := entry{Mode: info.Mode()}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			if e.Link, err = os.Readlink(path); err != nil {
				return err
			}
		case info.Mode().IsRegular():
			e.Size, e.ModTime = info.Size(), info.ModTime().UnixNano()
		}
		out[filepath.ToSlash(rel)] = e
		return nil
	})
	if os.IsNotExist(err) && len(out) == 0 {
		return snap{m: out}, nil
	}
	if err != nil {
		return snap{}, err
	}
	return snap{m: out, keys: sortedKeys(out)}, nil
}

// sameMeta is the "has it changed" test: type, permissions, size, mtime and link
// target. Directory mtimes are left out; a changed listing shows in the key set.
func sameMeta(a, b entry) bool {
	if a.Mode != b.Mode || a.Link != b.Link {
		return false
	}
	return !a.Mode.IsRegular() || (a.Size == b.Size && a.ModTime == b.ModTime)
}

func equalMeta(a, b snap) bool {
	if len(a.m) != len(b.m) {
		return false
	}
	for k, e := range a.m {
		o, ok := b.m[k]
		if !ok || !sameMeta(e, o) {
			return false
		}
	}
	return true
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashBytes(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// dataEntry is a small file made by the migration itself.
func dataEntry(data []byte, mode fs.FileMode) entry {
	return entry{Mode: mode, Size: int64(len(data)), Data: data}
}

// maxRecord bounds the files the migration reads whole to rewrite them: project and
// task records, session pointers and session records.
const maxRecord = 4 << 20

func readSmall(path string, size int64) ([]byte, error) {
	if size > maxRecord {
		return nil, fmt.Errorf("file is %d bytes; records over %d bytes are not rewritten", size, maxRecord)
	}
	return os.ReadFile(path)
}

// writeEntries makes the entries of get under root, in path order, and returns the
// digest of each as written. Files are streamed from Src through the hash, so one
// read gives both the copy and its checksum. Directory permissions come last, so a
// read-only directory can still be filled.
func writeEntries(root string, keys []string, get func(rel string) entry) (map[string]digest, error) {
	digs := make(map[string]digest, len(keys))
	var dirs []string
	for _, rel := range keys {
		e := get(rel)
		path := filepath.Join(root, filepath.FromSlash(rel))
		d := digest{Mode: uint32(e.Mode), Size: e.Size, Link: e.Link}
		switch {
		case e.Mode.IsDir():
			if err := os.MkdirAll(path, 0700); err != nil {
				return nil, err
			}
			dirs = append(dirs, rel)
		case e.Mode&fs.ModeSymlink != 0:
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
			if err := os.Symlink(e.Link, path); err != nil {
				return nil, err
			}
		case e.Mode.IsRegular() || e.Data != nil:
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
			sum, n, err := copyFile(path, e)
			if err != nil {
				return nil, err
			}
			d.SHA256, d.Size = sum, n
		default:
			return nil, fmt.Errorf("%s: special files cannot be copied", rel)
		}
		digs[rel] = d
	}
	for i := len(dirs) - 1; i >= 0; i-- {
		path := filepath.Join(root, filepath.FromSlash(dirs[i]))
		if err := os.Chmod(path, get(dirs[i]).Mode.Perm()); err != nil {
			return nil, err
		}
		syncDir(path)
	}
	return digs, nil
}

// copyFile writes e to path, synced, and returns the checksum and size of what it
// wrote. A source that changed size or mtime while it was read is an error.
func copyFile(path string, e entry) (sum string, n int64, err error) {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", 0, err
	}
	h := sha256.New()
	w := io.MultiWriter(out, h)
	if e.Data != nil {
		var m int
		m, err = w.Write(e.Data)
		n = int64(m)
	} else {
		n, err = copyFrom(w, e)
	}
	if err == nil {
		err = out.Chmod(e.Mode.Perm())
	}
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && e.ModTime != 0 {
		mt := time.Unix(0, e.ModTime)
		err = os.Chtimes(path, mt, mt)
	}
	if err != nil {
		return "", 0, fmt.Errorf("%s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func copyFrom(w io.Writer, e entry) (int64, error) {
	in, err := os.Open(e.Src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	n, err := io.Copy(w, in)
	if err != nil {
		return n, err
	}
	if info, err := os.Lstat(e.Src); err != nil || info.Size() != e.Size || info.ModTime().UnixNano() != e.ModTime {
		return n, fmt.Errorf("%s changed while it was copied; stop writers and run migrate again", e.Src)
	}
	return n, nil
}

// verifyEntries checks that root holds exactly the entries of want, with the same
// type, permissions, size and link targets, and re-reads every file for its digest.
func verifyEntries(root string, want snap, digs map[string]digest, rehash bool) error {
	got, err := snapshot(root)
	if err != nil {
		return err
	}
	if len(got.m) != len(want.m) {
		return fmt.Errorf("%s holds %d entries, expected %d", root, len(got.m), len(want.m))
	}
	for rel, w := range want.m {
		g, ok := got.m[rel]
		if !ok {
			return fmt.Errorf("%s: missing from %s", rel, root)
		}
		if g.Mode.Type() != w.Mode.Type() || g.Mode.Perm() != w.Mode.Perm() || g.Link != w.Link || g.Size != wantSize(w) {
			return fmt.Errorf("%s: differs from the plan in %s", rel, root)
		}
		if rehash && g.Mode.IsRegular() {
			sum, err := hashFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return err
			}
			if sum != digs[rel].SHA256 {
				return fmt.Errorf("%s: checksum differs in %s", rel, root)
			}
		}
	}
	return nil
}

func wantSize(e entry) int64 {
	if e.Data != nil {
		return int64(len(e.Data))
	}
	return e.Size
}

// syncDir flushes a directory entry to disk. It is best effort: Windows cannot open
// a directory for syncing.
func syncDir(path string) {
	if d, err := os.Open(path); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}

func within(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func under(p, root string) bool { return p == root || strings.HasPrefix(p, root+"/") }
