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

// Record is the sidecar for an imported file or an HTTPS link. File is relative
// to artifacts/. Publishing changes only Published; it never uploads anything.
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

// Store is scoped to a task resolved by where. Now can be fixed in tests.
type Store struct {
	TaskPath string
	Now      func() time.Time
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
func (s Store) Import(source, title string) (Record, error) {
	// Inspect before opening so a pipe or device cannot block an ordinary import.
	info, err := os.Stat(source)
	if err != nil {
		return Record{}, err
	}
	if !info.Mode().IsRegular() {
		return Record{}, errors.New("source must be a regular file")
	}
	f, err := os.Open(source)
	if err != nil {
		return Record{}, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return Record{}, err
	}
	if !info.Mode().IsRegular() {
		return Record{}, errors.New("source must be a regular file")
	}
	dir, err := s.directory(true)
	if err != nil {
		return Record{}, err
	}
	if title == "" {
		title = filepath.Base(source)
	}
	r := s.record(source, title, "file")
	r.ID, err = newID()
	if err != nil {
		return Record{}, err
	}
	r.File = r.ID + "-" + filepath.Base(source)
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
	_, copyErr := io.Copy(io.MultiWriter(out, hash), f)
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
