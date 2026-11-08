// Package fspath spells paths the way the filesystem stores them, so two
// spellings of one folder compare equal as plain strings.
package fspath

import (
	"os"
	"path/filepath"
	"strings"
)

// OnDiskCase rewrites each component of an existing, symlink-free absolute path
// to the name its directory lists. EvalSymlinks keeps the case it was given, so
// on a case-folding volume (the macOS default) ~/.GRID would otherwise differ
// from ~/.grid. A component that cannot be listed keeps its given spelling.
func OnDiskCase(p string) string {
	vol := filepath.VolumeName(p)
	rest := strings.TrimLeft(p[len(vol):], string(filepath.Separator))
	cur := vol + string(filepath.Separator)
	if rest == "" {
		return p
	}
	for _, name := range strings.Split(rest, string(filepath.Separator)) {
		cur = filepath.Join(cur, storedName(cur, name))
	}
	return cur
}

// storedName returns the entry of dir that name refers to, as dir lists it.
func storedName(dir, name string) string {
	if plain(name) {
		// On a case-sensitive volume the swapped spelling is a different, usually
		// missing, entry; skip listing a directory that may hold thousands.
		swapped := swapCase(name)
		if swapped == name {
			return name
		}
		if _, err := os.Lstat(filepath.Join(dir, swapped)); err != nil {
			return name
		}
	}
	d, err := os.Open(dir)
	if err != nil {
		return name
	}
	defer d.Close()
	want, err := os.Lstat(filepath.Join(dir, name))
	if err != nil {
		return name
	}
	// The path exists, so the listing holds this exact spelling or the entry the
	// volume folds it to (case, or Unicode form). Stop at the first entry that is
	// provably the same file rather than reading a large directory to the end.
	for {
		names, err := d.Readdirnames(256)
		for _, n := range names {
			if n == name {
				return name
			}
			if plain(name) && !strings.EqualFold(n, name) {
				continue
			}
			if info, err := os.Lstat(filepath.Join(dir, n)); err == nil && os.SameFile(want, info) {
				return n
			}
		}
		if err != nil {
			return name
		}
	}
}

func plain(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func swapCase(s string) string {
	b := []byte(s)
	for i, ch := range b {
		switch {
		case 'a' <= ch && ch <= 'z':
			b[i] = ch - 'a' + 'A'
		case 'A' <= ch && ch <= 'Z':
			b[i] = ch - 'A' + 'a'
		}
	}
	return string(b)
}
