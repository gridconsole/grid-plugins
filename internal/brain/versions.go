package brain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"

	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// versionsTopic is the folder below knowledge/ that holds every published
// entry's history. It is not a topic: scans and indexes skip it, and no topic
// may take its name.
const versionsTopic = "versions"

// Version is one stored copy of a published entry. Hash is the audit hash of
// the stored bytes, so a copy can be compared with a log.md line.
type Version struct {
	N    int    `json:"n"`
	Hash string `json:"hash"`
	Path string `json:"path"` // relative to the Grid home, like Entry.Path
}

var versionFile = regexp.MustCompile(`^[1-9][0-9]*\.md$`)

// auditHash is what log.md stores for published bytes: the first 16 hex digits
// of their SHA-256.
func auditHash(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:8])
}

func versionDir(topic, name string) string {
	return filepath.Join("knowledge", versionsTopic, topic, name)
}

func versionName(n int) string { return strconv.Itoa(n) + ".md" }

// versionNumbers lists the stored numbers, ascending. A missing folder is an
// empty history. Other names (temporary files, notes by hand) are ignored.
func versionNumbers(root *os.Root, dir string) ([]int, error) {
	if err := safefile.NoSymlinks(root, dir); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(root.FS(), filepath.ToSlash(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var numbers []int
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("brain paths cannot be symbolic links")
		}
		if e.IsDir() || !versionFile.MatchString(e.Name()) {
			continue
		}
		if n, err := strconv.Atoi(e.Name()[:len(e.Name())-len(".md")]); err == nil {
			numbers = append(numbers, n)
		}
	}
	sort.Ints(numbers)
	return numbers, nil
}

// recordVersion runs under the level lock, before the entry is published. It
// stores data as one higher than the highest version present, unless that
// version already holds exactly these bytes: a retry after an interrupted
// write then reuses its number instead of adding a duplicate.
func recordVersion(root *os.Root, topic, name string, data []byte) (int, string, error) {
	dir := versionDir(topic, name)
	numbers, err := versionNumbers(root, dir)
	if err != nil {
		return 0, "", err
	}
	next := 1
	if len(numbers) > 0 {
		last := numbers[len(numbers)-1]
		prev, err := safefile.Read(root, filepath.Join(dir, versionName(last)))
		if err != nil {
			return 0, "", err
		}
		if bytes.Equal(prev, data) {
			return last, auditHash(data), nil
		}
		next = last + 1
	}
	if err := atomicWrite(root, filepath.Join(dir, versionName(next)), data); err != nil {
		return 0, "", err
	}
	return next, auditHash(data), nil
}

// Versions lists the stored copies of one published entry, oldest first. An
// entry without history, or a level without a folder, returns an empty list.
func (s Store) Versions(level, topic, name string) ([]Version, error) {
	if !ValidName(topic) || !ValidName(name) {
		return nil, fmt.Errorf("topic and name must be plain names")
	}
	prefix, err := s.prefix(level)
	if err != nil {
		return nil, err
	}
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return []Version{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir := versionDir(topic, name)
	numbers, err := versionNumbers(root, dir)
	if err != nil {
		return nil, err
	}
	result := make([]Version, 0, len(numbers))
	for _, n := range numbers {
		path := filepath.Join(dir, versionName(n))
		b, err := safefile.Read(root, path)
		if err != nil {
			return nil, err
		}
		result = append(result, Version{N: n, Hash: auditHash(b), Path: filepath.ToSlash(filepath.Join(prefix, path))})
	}
	return result, nil
}
