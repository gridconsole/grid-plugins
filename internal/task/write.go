package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"go.yaml.in/yaml/v4"
)

// StateMoves is the operational state table. Terminal moves use stop, close and
// reopen, so status cannot silently reopen a closed task. Callers receive a copy.
func StateMoves() map[string][]string {
	return map[string][]string{
		"queued":    {"running", "waiting", "paused", "stuck"},
		"running":   {"waiting", "stuck", "reworking", "pausing", "paused"},
		"waiting":   {"running", "queued", "stuck", "reworking", "paused"},
		"stuck":     {"running", "queued", "waiting", "paused"},
		"reworking": {"running", "waiting", "stuck", "pausing", "paused"},
		"pausing":   {"paused", "running", "stuck"},
		"paused":    {"queued", "running"},
		"closed":    {},
	}
}

// NextState validates a move without touching disk. Stop means closed/stopped,
// as in the task menu; it records intent, not a claim that a worker was killed.
func NextState(current, action, requested string) (string, error) {
	moves := StateMoves()
	allowed, known := moves[current]
	if known {
		switch action {
		case "stop", "close":
			if current != "closed" {
				return "closed", nil
			}
		case "reopen":
			if current == "closed" {
				return "queued", nil
			}
		case "status":
			for _, next := range allowed {
				if next == requested {
					return next, nil
				}
			}
		}
	}
	return "", fmt.Errorf("cannot %s task in current state %q to %q", action, current, requested)
}

// Event is a local task write receipt, not a Feed post. The shared contract has
// no task-write schema yet; author identity uses the contract's existing shape.
type Event struct {
	Task       string          `json:"task"`
	Who        contract.Author `json:"who"`
	When       string          `json:"when"`
	What       string          `json:"what"`
	FromState  string          `json:"from_state"`
	ToState    string          `json:"to_state"`
	Note       string          `json:"note,omitempty"`
	LandsHash  string          `json:"lands_hash,omitempty"`
	PlanHash   string          `json:"plan_hash,omitempty"`
	PlanDigest string          `json:"plan_digest,omitempty"`
	Gate       string          `json:"gate,omitempty"`
	RefusalID  string          `json:"refusal_id,omitempty"`
	Decision   string          `json:"decision,omitempty"`
}

// WriteOptions names the task, the requested move and the caller's explanation.
// Who is supplied by the host; the terminal defaults to the local person, never
// an OS username or an inferred remote identity.
type WriteOptions struct {
	Cwd, ID, Action, State, Note string
	LandsHash                    string // optional deploy-rule binding supplied by the land approval integration
	Who                          contract.Author
}

// Change performs a checked state move and appends its receipt under one lock.
func (s Store) Change(o WriteOptions) (Event, error) {
	var event Event
	if err := o.validateText(); err != nil {
		return event, err
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		next, err := NextState(t.State, o.Action, o.State)
		if err != nil {
			return err
		}
		event = s.event(t, o, next)
		data, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
		if err != nil {
			return err
		}
		updated, err := rewriteTask(data, func(mapping *yaml.Node) error {
			setScalar(mapping, "state", next, false)
			return nil
		}, nil)
		if err != nil {
			return err
		}
		if _, err := Fold(t.Path, string(updated), t.Intent); err != nil {
			return err
		}
		files := map[string][]byte{"task.md": updated}
		if o.Action == "reopen" {
			// A reopened task needs both gates approved again: the receipt below
			// is not an approve event, so neither file stays valid.
			for _, name := range []string{approvalFile, shipApprovalFile} {
				previous, err := readOptional(filepath.Join(t.Path, name))
				if err != nil {
					return err
				}
				if previous != nil {
					files[name], err = json.Marshal(event)
					if err != nil {
						return err
					}
				}
			}
		}
		return commitWrite(t.Path, event, files)
	})
	return event, err
}

// rewriteTask edits the task.md header as a YAML document, so key order,
// comments and unknown keys survive, and optionally rewrites the body.
func rewriteTask(data []byte, edit func(mapping *yaml.Node) error, body func(string) string) ([]byte, error) {
	fm := taskfile.ParseFrontmatter(string(data))
	if fm == nil {
		return nil, errors.New("task.md has no YAML front matter")
	}
	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("task.md front matter is not a mapping")
	}
	if err := edit(doc.Content[0]); err != nil {
		return nil, err
	}
	front, err := yaml.Dump(&doc, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	if err != nil {
		return nil, err
	}
	text := fm.Body
	if body != nil {
		text = body(text)
	}
	return append(append([]byte("---\n"), front...), []byte("---\n"+text)...), nil
}

// setScalar sets a string key of the header mapping, adding it at the end when
// add is true and the key is missing. It reports whether the key was set.
func setScalar(mapping *yaml.Node, key, value string, add bool) bool {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			old := mapping.Content[i+1]
			mapping.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value, HeadComment: old.HeadComment, LineComment: old.LineComment, FootComment: old.FootComment}
			return true
		}
	}
	if !add {
		return false
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	return true
}

// scalar returns the string value of a header key, or "" when it is absent or
// not a scalar.
func scalar(mapping *yaml.Node, key string) string {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key && mapping.Content[i+1].Kind == yaml.ScalarNode {
			return mapping.Content[i+1].Value
		}
	}
	return ""
}

// Note appends one history line through the same lock, journal and receipt as a
// state move, so it can neither race Change nor be overwritten by journal replay.
// It changes no task state, and a closed task can still be annotated.
func (s Store) Note(o WriteOptions) (Event, error) {
	var event Event
	if err := o.validateText(); err != nil {
		return event, err
	}
	if strings.ContainsRune(o.Note, '\n') {
		return event, fmt.Errorf("%w: history needs one nonempty line", ErrEditUsage)
	}
	if strings.TrimSpace(o.Note) == "" {
		return event, errors.New("history needs one nonempty line")
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		o.Action = "note"
		event = s.event(t, o, t.State)
		return commitWrite(t.Path, event, map[string][]byte{})
	})
	return event, err
}

func (s Store) event(t Task, o WriteOptions, next string) Event {
	now := s.clock()
	who := o.Who
	if who.ID == "" {
		who = contract.Author{ID: "local", Role: "owner"}
	}
	return Event{Task: t.ID, Who: who, When: contract.FormatTime(now), What: o.Action, FromState: t.State, ToState: next, Note: o.Note}
}

func (s Store) clock() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// withWrite re-reads the record after locking: a resolved task is only a path,
// never a state snapshot on which a later writer may base a transition.
func (s Store) withWrite(cwd, id string, fn func(Task) error) error {
	if id == "" {
		return errors.New("give a task id (T-NN)")
	}
	if s.Resolver.GridHome == "" {
		return errors.New("cannot find the Grid home; set GRID_HOME")
	}
	s.Resolver.GridHome = gridhome.Real(s.Resolver.GridHome)
	t, err := s.Show(cwd, id)
	if err != nil {
		return err
	}
	if err := safeDir(s.Resolver.GridHome, t.Path); err != nil {
		return err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(t.Path, ".task.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	if err := checkFormat(s.Resolver.GridHome); err != nil {
		return err
	}
	if err := writableFile(t.Path, "task.md"); err != nil {
		return err
	}
	if err := recoverWrite(t.Path); err != nil {
		return err
	}
	t, err = Read(t.Path)
	if err != nil {
		return err
	}
	return fn(t)
}

const (
	journalName      = ".task-write.json"
	approvalFile     = "approval.json"
	shipApprovalFile = "approval-ship.json"
)

var decisionFile = regexp.MustCompile(`^refusal-R-[1-9][0-9]*\.json$`)

// commitWrite uses the existing atomic writer. The durable redo journal is the
// commit point. Recovery is idempotent (full file images, including the appended
// history line), so a crash between renames never loses or duplicates a receipt.
func commitWrite(dir string, event Event, files map[string][]byte) error {
	history, err := readOptional(filepath.Join(dir, "history.md"))
	if err != nil {
		return err
	}
	if len(history) > 0 && history[len(history)-1] != '\n' {
		history = append(history, '\n')
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return err
	}
	// A JSON object on each Markdown bullet is readable and unambiguous, including
	// embedded newlines and delimiter characters in notes. Old prose stays intact.
	files["history.md"] = append(history, []byte("- "+string(encoded)+"\n")...)
	for name := range files {
		if err := writableFile(dir, name); err != nil {
			return err
		}
	}
	data, err := json.Marshal(files)
	if err != nil {
		return err
	}
	if err := writableFile(dir, journalName); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(dir, journalName), data); err != nil {
		return err
	}
	return recoverWrite(dir)
}

func recoverWrite(dir string) error {
	data, err := readOptional(filepath.Join(dir, journalName))
	if err != nil || data == nil {
		return err
	}
	var files map[string][]byte
	if err := json.Unmarshal(data, &files); err != nil {
		return fmt.Errorf("task write journal: %w", err)
	}
	// A note writes history alone, so one file is a complete journal.
	if files["history.md"] == nil {
		return errors.New("incomplete task write journal")
	}
	for name := range files {
		if name != "task.md" && name != "history.md" && name != approvalFile && name != shipApprovalFile && !decisionFile.MatchString(name) {
			return fmt.Errorf("invalid journal file %q", name)
		}
		if err := writableFile(dir, name); err != nil {
			return err
		}
	}
	// Publish history last. Readers of several files must hold .task.lock.
	// A Feed writer may have initialized a legacy task after this journal was
	// left by a crash. Never replay an older image that drops that stable ID.
	if data, ok := files["task.md"]; ok {
		id, err := feed.TaskID(dir)
		if err == nil {
			files["task.md"], err = feed.WithTaskID(data, id)
		}
		if err != nil && !errors.Is(err, feed.ErrNoTaskID) {
			return err
		}
	}
	for name, data := range files {
		if name != "history.md" {
			if err := atomicWrite(filepath.Join(dir, name), data); err != nil {
				return err
			}
		}
	}
	if err := atomicWrite(filepath.Join(dir, "history.md"), files["history.md"]); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, journalName)); err != nil {
		return err
	}
	return syncDir(dir)
}

func writableFile(dir, name string) error {
	if filepath.Base(name) != name {
		return errors.New("task write must name a direct child")
	}
	info, err := os.Lstat(filepath.Join(dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s must be a regular file, not a symlink", name)
	}
	return nil
}

func readOptional(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file, not a symlink", filepath.Base(path))
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s must be a regular file", filepath.Base(path))
	}
	return io.ReadAll(f)
}

// History returns every original line in append order, including legacy prose.
// JSON callers get a stable array of lines without trying to reinterpret legacy
// timestamps or ownership. New lines carry all five audit fields in Event.
func (s Store) History(cwd, id string) ([]string, error) {
	lines := []string{}
	err := s.withWrite(cwd, id, func(t Task) error {
		data, err := readOptional(filepath.Join(t.Path, "history.md"))
		if err != nil {
			return err
		}
		if len(data) > 0 {
			lines = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		}
		return nil
	})
	return lines, err
}
