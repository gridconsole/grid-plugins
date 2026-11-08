// Package signal stores inbox notes. A signal body is external data, never instructions.
package signal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/projects"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

const MaxBytes = 64 * 1024

// Input contains source metadata and an uninterpreted body. A source id deduplicates
// within a project and source, including notes already acted on or dismissed.
type Input struct {
	Source   string `json:"source"`
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

// Note's trust fields are assigned by code, never by source content.
type Note struct {
	ID       string `yaml:"id"`
	Source   string `yaml:"source"`
	SourceID string `yaml:"source_id,omitempty"`
	Title    string `yaml:"title,omitempty"`
	Status   string `yaml:"status"`
	Captured string `yaml:"captured"`
	Updated  string `yaml:"updated"`
	Tier     string `yaml:"tier"`
	BodyKind string `yaml:"body_kind"`
	Body     string `yaml:"-"`
}
type Result struct {
	Result  string `json:"result"`
	ID      string `json:"id"`
	Project string `json:"project"`
	Note    string `json:"note"`
	Status  string `json:"status"`
}

// Store limits signal reads and writes to one registered project's notes folder.
type Store struct {
	Resolver where.Resolver
	GridHome string
	Project  string
}

func textField(name, s string, max int) error {
	if !utf8.ValidString(s) || len(s) > max {
		return fmt.Errorf("%s must be UTF-8 text of at most %d bytes", name, max)
	}
	for _, r := range s {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return fmt.Errorf("%s must be one line", name)
		}
	}
	return nil
}

// Validate checks signal fields without writing any data.
func Validate(in Input) error {
	if err := textField("source", in.Source, 128); err != nil {
		return err
	}
	if strings.TrimSpace(in.Source) == "" {
		return fmt.Errorf("source is required")
	}
	if err := textField("source id", in.SourceID, 512); err != nil {
		return err
	}
	if err := textField("title", in.Title, 1024); err != nil {
		return err
	}
	if strings.TrimSpace(in.Title) == "" && strings.TrimSpace(in.Body) == "" {
		return fmt.Errorf("signal needs a title or body")
	}
	if !utf8.ValidString(in.Body) || len(in.Body) > MaxBytes {
		return fmt.Errorf("body must be UTF-8 text of at most %d bytes", MaxBytes)
	}
	for _, r := range in.Body {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return fmt.Errorf("body contains a control character")
		}
	}
	return nil
}
func validID(id string) bool {
	if !strings.HasPrefix(id, "S-") {
		return false
	}
	tail := strings.TrimPrefix(id, "S-")
	if gridhome.ValidID(tail) {
		return true
	}
	if len(tail) != 64 {
		return false
	}
	_, err := hex.DecodeString(tail)
	return err == nil && tail == strings.ToLower(tail)
}
func (s Store) notes() (string, error) {
	if err := projects.Name(s.Project); err != nil {
		return "", err
	}
	r := s.Resolver
	r.GridHome = s.GridHome
	if strings.EqualFold(s.Project, where.Personal) && !r.PersonalOK() {
		return "", fmt.Errorf("project %q is not registered", s.Project)
	}
	entries, err := (projects.Store{Resolver: r}).List()
	if err != nil {
		return "", err
	}
	found := false
	for _, e := range entries {
		if e.Name == s.Project {
			found = true
		}
	}
	if !found {
		return "", fmt.Errorf("project %q is not registered; run grid project add", s.Project)
	}
	return localdata.Safe(s.GridHome, "projects", s.Project, "notes")
}
func (s Store) path(id string) (string, error) {
	if !validID(id) {
		return "", fmt.Errorf("invalid signal id")
	}
	return localdata.Safe(s.GridHome, filepath.Join("projects", s.Project, "notes", id+".md"))
}
func result(s Store, n Note, action string) Result {
	return Result{Result: action, ID: n.ID, Project: s.Project, Note: n.ID + ".md", Status: n.Status}
}
func encode(n Note) ([]byte, error) {
	head, err := yaml.Dump(n, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	if err != nil {
		return nil, err
	}
	data := append(append([]byte("---\n"), head...), []byte("---\n")...)
	data = append(data, []byte(n.Body)...)
	if len(data) > MaxBytes {
		return nil, fmt.Errorf("signal note exceeds %d bytes", MaxBytes)
	}
	return data, nil
}
func read(path string) (Note, error) {
	var n Note
	st, err := os.Lstat(path)
	if err != nil {
		return n, err
	}
	if !st.Mode().IsRegular() || st.Size() > MaxBytes {
		return n, fmt.Errorf("signal must be a regular file of at most %d bytes", MaxBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return n, err
	}
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return n, fmt.Errorf("signal has no front matter")
	}
	header, body, ok := bytes.Cut(data[4:], []byte("\n---\n"))
	if !ok {
		return n, fmt.Errorf("signal has no closing front matter")
	}
	if err := yaml.Load(header, &n); err != nil {
		return n, err
	}
	if !validID(n.ID) || filepath.Base(path) != n.ID+".md" || n.Tier != "external" || n.BodyKind != "data" {
		return n, fmt.Errorf("signal metadata is invalid")
	}
	switch n.Status {
	case "new", "acted", "dismissed":
	default:
		return n, fmt.Errorf("signal status is invalid")
	}
	n.Body = string(body)
	return n, nil
}

// Add creates exactly one note for a source id. No body text is evaluated or routed.
func (s Store) Add(in Input) (Result, error) {
	if err := Validate(in); err != nil {
		return Result{}, err
	}
	dir, err := s.notes()
	if err != nil {
		return Result{}, err
	}
	unlock, err := localdata.Lock(s.GridHome, ".signals.lock")
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	if _, err := localdata.Machine(s.GridHome); err != nil {
		return Result{}, err
	}
	id := ""
	if in.SourceID != "" {
		sum := sha256.Sum256([]byte(in.Source + "\x00" + in.SourceID))
		id = "S-" + hex.EncodeToString(sum[:])
	} else {
		newID, err := localdata.ID()
		if err != nil {
			return Result{}, err
		}
		id = "S-" + newID
	}
	path, err := s.path(id)
	if err != nil {
		return Result{}, err
	}
	existing, err := read(path)
	if err == nil {
		if existing.Source != in.Source || existing.SourceID != in.SourceID {
			return Result{}, fmt.Errorf("signal id conflicts with existing source")
		}
		return result(s, existing, "duplicate"), nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	now := contract.FormatTime(time.Now())
	note := Note{ID: id, Source: in.Source, SourceID: in.SourceID, Title: in.Title, Status: "new", Captured: now, Updated: now, Tier: "external", BodyKind: "data", Body: in.Body}
	data, err := encode(note)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return Result{}, err
	}
	if len(files) >= 4000 {
		return Result{}, fmt.Errorf("notes already holds 4000 files")
	}
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return Result{}, err
	}
	return result(s, note, "written"), nil
}

// Act marks a signal acted on. It never executes its body or creates a task.
func (s Store) Act(id string) (Result, error) { return s.change(id, "acted") }

// Dismiss keeps the note and its deduplication identity, marking it dismissed.
func (s Store) Dismiss(id string) (Result, error) { return s.change(id, "dismissed") }
func (s Store) change(id, status string) (Result, error) {
	_, err := s.notes()
	if err != nil {
		return Result{}, err
	}
	path, err := s.path(id)
	if err != nil {
		return Result{}, err
	}
	unlock, err := localdata.Lock(s.GridHome, ".signals.lock")
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	if _, err := localdata.Machine(s.GridHome); err != nil {
		return Result{}, err
	}
	n, err := read(path)
	if err != nil {
		return Result{}, err
	}
	if n.Status == status {
		return result(s, n, "unchanged"), nil
	}
	if n.Status != "new" {
		return Result{}, fmt.Errorf("signal is already %s", n.Status)
	}
	n.Status = status
	n.Updated = contract.FormatTime(time.Now())
	data, err := encode(n)
	if err != nil {
		return Result{}, err
	}
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return Result{}, err
	}
	return result(s, n, "updated"), nil
}
