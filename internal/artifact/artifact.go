// Package artifact stores task deliverables and local publication records.
package artifact

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// DefaultMaxImportBytes is the largest file Import copies when MaxBytes is zero.
const DefaultMaxImportBytes int64 = 200 << 20

// Record is the sidecar for an imported file or an HTTPS link. File is a bare
// name inside artifacts/. Source of an import is the path below the project
// folder when the file sits inside it, else the base name and a hash of the
// absolute path, so a synced sidecar never carries a personal path. Publishing
// changes only Published; it never uploads anything.
type Record struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	SHA256    string `json:"sha256"`
	Created   string `json:"created"`
	File      string `json:"file,omitempty"`
	Published bool   `json:"published"`
}

// Store is scoped to a task resolved by where. ProjectFolder is the project's
// folder on this machine, "" when unknown. MaxBytes caps one import and defaults
// to DefaultMaxImportBytes. Now can be fixed in tests.
type Store struct {
	TaskPath      string
	ProjectFolder string
	MaxBytes      int64
	Now           func() time.Time
}

var idPattern = regexp.MustCompile(`^A-[0-9a-f]{32}$`)

func (s Store) directory(create bool) (string, error) {
	if s.TaskPath == "" {
		return "", errors.New("a task is required")
	}
	info, err := os.Stat(s.TaskPath)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("task is not a directory")
	}
	dir := filepath.Join(s.TaskPath, "artifacts")
	if create {
		if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return "", err
		}
	}
	info, err = os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("artifacts must be a directory, not a symlink")
	}
	return dir, nil
}

func (s Store) record(source, title, kind string) Record {
	if title == "" {
		title = source
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	return Record{Title: title, Source: source, Kind: kind, Created: now().UTC().Format(time.RFC3339Nano)}
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "A-" + hex.EncodeToString(b[:]), nil
}

// Import copies the bytes of a regular file, preserving its source, and hashes
// the bytes actually copied. Every import has a unique name, even for duplicates.
// The file is inspected on the opened handle, so a source swapped for a pipe or
// device after the call began cannot block it.
func (s Store) Import(source, title string) (Record, error) {
	f, err := openSource(source)
	if err != nil {
		return Record{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return Record{}, err
	}
	if !info.Mode().IsRegular() {
		return Record{}, errors.New("source must be a regular file")
	}
	limit := s.MaxBytes
	if limit <= 0 {
		limit = DefaultMaxImportBytes
	}
	if info.Size() > limit {
		return Record{}, errTooLarge(limit)
	}
	name := filepath.Base(source)
	if !validName(name) {
		return Record{}, fmt.Errorf("source name %q cannot be used as an artifact file name", name)
	}
	dir, err := s.directory(true)
	if err != nil {
		return Record{}, err
	}
	if title == "" {
		title = name
	}
	r := s.record(s.sourceLabel(source), title, "file")
	r.ID, err = newID()
	if err != nil {
		return Record{}, err
	}
	r.File = r.ID + "-" + name
	dest := filepath.Join(dir, r.File)
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return Record{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(dest)
		}
	}()
	hash := sha256.New()
	copyErr := copyLimited(io.MultiWriter(out, hash), f, limit)
	syncErr := out.Sync()
	closeErr := out.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		return Record{}, err
	}
	r.SHA256 = hex.EncodeToString(hash.Sum(nil))
	if err := save(dir, r); err != nil {
		return Record{}, err
	}
	committed = true
	return r, nil
}

// sourceLabel names where an import came from without an absolute personal path:
// the slash-separated path below the project folder, else the base name and the
// first 8 bytes of the SHA-256 of the absolute path, as name#16-hex.
func (s Store) sourceLabel(source string) string {
	abs, err := filepath.Abs(source)
	if err != nil {
		abs = source
	}
	abs = resolveParent(abs)
	if s.ProjectFolder != "" {
		if rel, ok := below(resolveFull(s.ProjectFolder), abs); ok {
			return filepath.ToSlash(rel)
		}
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Base(abs) + "#" + hex.EncodeToString(sum[:8])
}

// resolveParent resolves symlinks in the folders above path, so the project folder
// and the source compare equal when one is reached through a link such as /var.
func resolveParent(path string) string {
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return path
	}
	return filepath.Join(dir, filepath.Base(path))
}

func resolveFull(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

// below returns path relative to base when path is a file inside base.
func below(base, path string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

func errTooLarge(limit int64) error {
	return fmt.Errorf("source is above the import limit of %d bytes", limit)
}

// copyLimited copies src to dst and fails if src holds more than limit bytes, so
// a file that grows after its size was read cannot fill the disk.
func copyLimited(dst io.Writer, src io.Reader, limit int64) error {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err == nil && n > limit {
		err = errTooLarge(limit)
	}
	return err
}

// validName reports whether name is one plain file name: no separator of either
// platform, no NUL, no volume, not "." or "..", and not absolute.
func validName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\\x00") && !filepath.IsAbs(name) && filepath.VolumeName(name) == ""
}

// Link records a URL without fetching it. Credentials and non-HTTPS URLs are refused.
func (s Store) Link(source, title string) (Record, error) {
	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || strings.ContainsAny(source, "\r\n\t ") {
		return Record{}, errors.New("link must be an absolute HTTPS URL without credentials")
	}
	dir, err := s.directory(true)
	if err != nil {
		return Record{}, err
	}
	r := s.record(source, title, "link")
	r.ID, err = newID()
	if err != nil {
		return Record{}, err
	}
	if err := save(dir, r); err != nil {
		return Record{}, err
	}
	return r, nil
}

// List returns records in stable ID order. Files without sidecars are not records.
func (s Store) List() ([]Record, error) {
	if s.TaskPath == "" {
		return nil, errors.New("a task is required")
	}
	if _, err := os.Stat(s.TaskPath); err != nil {
		return nil, err
	}
	records := []Record{}
	dir, err := s.directory(false)
	if errors.Is(err, os.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		id := strings.TrimSuffix(e.Name(), ".json")
		if !strings.HasSuffix(e.Name(), ".json") || !idPattern.MatchString(id) {
			continue
		}
		r, err := read(dir, id)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

// Publish marks a record published locally and is safe to repeat.
func (s Store) Publish(id string) (Record, error) {
	if !idPattern.MatchString(id) {
		return Record{}, errors.New("invalid artifact id")
	}
	dir, err := s.directory(false)
	if err != nil {
		return Record{}, err
	}
	r, err := read(dir, id)
	if err != nil {
		return Record{}, err
	}
	if !r.Published {
		r.Published = true
		if err := save(dir, r); err != nil {
			return Record{}, err
		}
	}
	return r, nil
}

func read(dir, id string) (Record, error) {
	path := filepath.Join(dir, id+".json")
	info, err := os.Lstat(path)
	if err != nil {
		return Record{}, err
	}
	if !info.Mode().IsRegular() {
		return Record{}, errors.New("artifact record must be a regular file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil {
		return Record{}, fmt.Errorf("artifact %s: %w", id, err)
	}
	if r.ID != id || (r.Kind != "file" && r.Kind != "link") {
		return Record{}, fmt.Errorf("invalid artifact record %s", id)
	}
	if r.File != "" && !validName(r.File) {
		return Record{}, fmt.Errorf("artifact %s: invalid file name", id)
	}
	return r, nil
}

func save(dir string, r Record) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".record-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(append(data, '\n'))
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, r.ID+".json"))
}
