package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
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

// States lists the task states of the state table, sorted by name.
func States() []string {
	moves := StateMoves()
	out := make([]string, 0, len(moves))
	for state := range moves {
		out = append(out, state)
	}
	sort.Strings(out)
	return out
}

// ErrStateMove marks a move the state table refuses. NextState returns a
// stateMoveError that unwraps to it. A plain %w would add the sentinel's words to
// the refusal; this keeps the text as it was, and a caller classifies the
// refusal with errors.Is.
var ErrStateMove = errors.New("state move refused")

// stateMoveError is the refusal of NextState: its text is the sentence alone and
// errors.Is finds ErrStateMove behind it.
type stateMoveError struct{ text string }

func (e *stateMoveError) Error() string { return e.text }

func (e *stateMoveError) Unwrap() error { return ErrStateMove }

// NextState validates a move without touching disk. Stop means closed/stopped,
// as in the task menu; it records intent, not a claim that a worker was killed.
func NextState(current, action, requested string) (string, error) {
	return nextStateOf("", current, action, requested)
}

// nextStateOf is NextState for the task id, so a refusal can name the task.
func nextStateOf(id, current, action, requested string) (string, error) {
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
	return "", &stateMoveError{stateMoveText(id, current, action, requested)}
}

// stateMoveText is the one plain sentence of a refused move, with none of the
// state table's words. id may be empty when the caller has no task.
func stateMoveText(id, current, action, requested string) string {
	name := id
	if name == "" {
		name = "the task"
	}
	switch {
	case (action == "stop" || action == "close") && current == "closed":
		return name + " is already closed"
	case action == "status" && requested == current:
		return name + " is already " + current
	case action == "reopen":
		return name + " is " + current + "; only a closed task can be reopened"
	}
	return name + " is " + current + " and cannot move to " + requested
}

// Event is a local task write receipt, not a Feed post. The shared contract has
// no task-write schema yet; author identity uses the contract's existing shape.
type Event struct {
	Via        string          `json:"via,omitempty"`
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
	// Gate is read from old history lines only: Grid writes none (a receipt
	// that names a gate is not the plan approval).
	Gate string `json:"gate,omitempty"`
}

// WriteOptions names the task, the requested move and the caller's explanation.
// Who is supplied by the host; the terminal defaults to the local person, never
// an OS username or an inferred remote identity.
type WriteOptions struct {
	Via                          string // person entry point, supplied by the CLI gate
	Cwd, ID, Action, State, Note string
	LandsHash                    string // optional deploy-rule binding supplied by the land approval integration
	Who                          contract.Author
	// PlanHashToken is the approval token policy: empty or "any" accepts either
	// form; "digest" requires 64 hex digits for a non-empty token.
	PlanHashToken string
}

// Change performs a checked state move and appends its receipt under one lock.
func (s Store) Change(o WriteOptions) (Event, error) {
	var event Event
	if err := o.validateText(); err != nil {
		return event, err
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		var err error
		event, err = s.change(t, o)
		return err
	})
	return event, err
}

// change is the move Change makes under the task lock: it checks the move
// against the state table, rewrites task.md and appends the receipt.
func (s Store) change(t Task, o WriteOptions) (Event, error) {
	var event Event
	next, err := nextStateOf(t.ID, t.State, o.Action, o.State)
	if err != nil {
		return event, err
	}
	event = s.event(t, o, next)
	data, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
	if err != nil {
		return event, err
	}
	updated, err := rewriteTask(data, func(mapping *yaml.Node) error {
		setScalar(mapping, "state", next, false)
		return nil
	}, nil)
	if err != nil {
		return event, err
	}
	if _, err := Fold(t.Path, string(updated), t.Intent); err != nil {
		return event, err
	}
	files := map[string][]byte{"task.md": updated}
	if o.Action == "reopen" {
		// A reopened task needs its plan approved again: the receipt below is not an
		// approve event, so approval.json no longer matches. The plan Job's old result
		// is superseded by the new plan Job that grid task reopen posts (see
		// planJobResult).
		previous, err := readOptional(filepath.Join(t.Path, approvalFile))
		if err != nil {
			return event, err
		}
		if previous != nil {
			if files[approvalFile], err = json.Marshal(event); err != nil {
				return event, err
			}
		}
	}
	return event, commitWrite(t.Path, event, files)
}

// RunningFromQueued moves a task from queued to running, as the worker does
// when it assigns the task's first Job. It makes the move of Change for action
// status and writes the same receipt, under the same lock. The state is read
// inside the lock and only queued moves: a person may have set waiting, stuck
// or paused a moment before, and then the task is left as it is, with no
// receipt and no error. moved reports whether this call wrote the move. o names
// the task (ID, Cwd), the author (Who) and the note; Action and State are set here.
func (s Store) RunningFromQueued(o WriteOptions) (moved bool, event Event, err error) {
	o.Action, o.State = "status", "running"
	if err := o.validateText(); err != nil {
		return false, event, err
	}
	err = s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State != "queued" {
			return nil
		}
		var moveErr error
		event, moveErr = s.change(t, o)
		moved = moveErr == nil
		return moveErr
	})
	return moved, event, err
}

// SyncPersonWait keeps state true to who the task waits on: a running task whose
// open Jobs all wait on a person moves to waiting, and a waiting task with an
// agent Job open moves back to running. Any other state, and a task waiting at
// step ship, is left as it is, with no receipt and no error. o names the task and
// the author as for RunningFromQueued; Action and State are set here.
func (s Store) SyncPersonWait(o WriteOptions, onPerson bool) (moved bool, event Event, err error) {
	o.Action, o.State = "status", "waiting"
	from := "running"
	if !onPerson {
		o.State, from = "running", "waiting"
	}
	if err := o.validateText(); err != nil {
		return false, event, err
	}
	err = s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State != from {
			return nil
		}
		if !onPerson {
			data, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
			if err != nil {
				return err
			}
			if fm := taskfile.ScalarFrontmatter(string(data)); fm != nil && fm.Data["step"] == "ship" {
				return nil
			}
		}
		var moveErr error
		event, moveErr = s.change(t, o)
		moved = moveErr == nil
		return moveErr
	})
	return moved, event, err
}

// rewriteTask edits the task.md header as a YAML document and writes back only
// the lines of the keys the edit set, added or removed: every other header line
// keeps its bytes (comments, spelling, unknown keys), and optionally rewrites
// the body.
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
	mapping := doc.Content[0]
	if mapping.Style&yaml.FlowStyle != 0 {
		return nil, errors.New("task.md front matter needs a block mapping")
	}
	// Snapshot the header as read: setScalar swaps a value node for a new one,
	// so the old nodes keep their source lines and show an edit made in place.
	type entry struct {
		key, value *yaml.Node
		was        yaml.Node
	}
	var entries []entry
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		entries = append(entries, entry{mapping.Content[i], mapping.Content[i+1], *mapping.Content[i+1]})
	}
	if err := edit(mapping); err != nil {
		return nil, err
	}
	lines := strings.SplitAfter(string(data), "\n")
	end := 1
	for end < len(lines) && strings.TrimRight(lines[end], "\r\n") != "---" {
		end++
	}
	if end == len(lines) || strings.Join(lines[end+1:], "") != fm.Body {
		return nil, errors.New("task.md front matter is not closed by a plain --- line")
	}
	newline := "\n"
	if strings.HasSuffix(lines[0], "\r\n") {
		newline = "\r\n"
	}
	// Each original key owns its lines up to the next key's, less the comments
	// and blank lines at the end, which belong to the text around it.
	type span struct{ start, stop int }
	spans := map[*yaml.Node]span{}
	for n, e := range entries {
		start, stop := e.key.Line-1, end
		if n+1 < len(entries) {
			stop = entries[n+1].key.Line - 1
		}
		if start < 1 || start >= stop || stop > end {
			return nil, fmt.Errorf("task.md key %q has no lines of its own to rewrite", e.key.Value)
		}
		for stop > start+1 {
			line := strings.TrimSpace(lines[stop-1])
			if line != "" && !strings.HasPrefix(line, "#") {
				break
			}
			stop--
		}
		spans[e.key] = span{start, stop}
	}
	// A key whose value node is new was set by the edit; every other key must
	// read the same afterwards. A value changed in place counts as untouched and
	// would be lost with its lines kept, so the write is refused instead.
	set := map[string]bool{}
	replace := map[int]string{}
	drop := map[int]int{}
	oneLine := map[string]bool{}
	kept := map[*yaml.Node]bool{}
	added := ""
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		sp, old := spans[key]
		if !old {
			text, err := dumpEntry(key.Value, value, newline)
			if err != nil {
				return nil, err
			}
			set[key.Value] = true
			added += text
			continue
		}
		kept[key] = true
		var e entry
		for _, e = range entries {
			if e.key == key {
				break
			}
		}
		if value != e.value {
			if key.Kind != yaml.ScalarNode || key.Column != 1 || e.value.Anchor != "" {
				return nil, fmt.Errorf("task.md key %q must be an unanchored key at the start of its line to be set", key.Value)
			}
			text, err := dumpEntry(key.Value, value, newline)
			if err != nil {
				return nil, err
			}
			set[key.Value] = true
			replace[sp.start], drop[sp.start] = text, sp.stop
			continue
		}
		if value.Kind != e.was.Kind || value.Tag != e.was.Tag || value.Value != e.was.Value || value.Style != e.was.Style || len(value.Content) != len(e.was.Content) {
			return nil, fmt.Errorf("task.md key %q would read differently after this write; the edit changed it in place; nothing was written", key.Value)
		}
		// One line is kept byte for byte, so the reader sees it as it did.
		oneLine[key.Value] = oneLine[key.Value] || sp.stop-sp.start == 1
	}
	for _, e := range entries {
		if sp := spans[e.key]; !kept[e.key] {
			replace[sp.start], drop[sp.start] = "", sp.stop
		}
	}
	var out strings.Builder
	for i := 0; i < end; {
		if text, ok := replace[i]; ok {
			out.WriteString(text)
			i = drop[i]
		} else {
			out.WriteString(lines[i])
			i++
		}
	}
	text := fm.Body
	if body != nil {
		text = body(text)
	}
	result := []byte(out.String() + added + lines[end] + text)
	// The reader's view of the workflow must not change, whatever it was.
	after := taskfile.ParseFrontmatter(string(result))
	if after == nil || !sameWorkflow(fm, after) {
		return nil, errors.New(workflowRefusal)
	}
	if key := changedKey(fm, after, set); key != "" {
		return nil, fmt.Errorf(keyRefusal, key)
	}
	if fm.Odd != after.Odd {
		return nil, errors.New("task.md has a __proto__, constructor or prototype key the plugin's reader refuses, and this write would change that; remove the key, then retry; nothing was written")
	}
	// A value the reader cannot see whole (a list with items at column 0, a
	// continuation line) is read by YAML as more than the reader sees. Such a
	// header was never written by Grid, so it is refused until it is fixed by
	// hand, as a whole-header dump would have rewritten it into something the
	// reader reads differently, and moved a held task past its plan gate.
	// A one-line value is kept byte for byte, so there is nothing to refuse.
	front, err := yaml.Dump(&doc, yaml.WithIndent(2), yaml.WithLineWidth(-1), yaml.WithCompactSeqIndent(false))
	if err != nil {
		return nil, err
	}
	dumped := taskfile.ParseFrontmatter("---\n" + string(front) + "---\n" + fm.Body)
	if dumped == nil {
		return nil, errors.New(workflowRefusal)
	}
	skip := map[string]bool{}
	for key := range set {
		skip[key] = true
	}
	for key, one := range oneLine {
		if one {
			skip[key] = true
		}
	}
	if !skip["workflow"] && !sameWorkflow(fm, dumped) {
		return nil, errors.New(workflowRefusal)
	}
	if key := changedKey(fm, dumped, skip); key != "" {
		return nil, fmt.Errorf(keyRefusal, key)
	}
	return result, nil
}

const (
	workflowRefusal = "task.md workflow list is not readable as written; indent each item under workflow: (for example \"  - {step: build, kind: run}\") or remove the list, then retry; nothing was written"
	keyRefusal      = "task.md key %q would read differently after this write; put its whole value on the key's own line (no indented continuation lines) or remove the key, then retry; nothing was written"
)

// dumpEntry spells one key and its value as the whole-header dump does.
// Sequences stay indented under their key: the plugin's reader only takes list
// items that are indented.
func dumpEntry(key string, value *yaml.Node, newline string) (string, error) {
	own := *value
	own.HeadComment, own.FootComment = "", ""
	pair := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &own}}
	front, err := yaml.Dump(&yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{pair}}, yaml.WithIndent(2), yaml.WithLineWidth(-1), yaml.WithCompactSeqIndent(false))
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(front), "\n", newline), nil
}

// sameWorkflow reports whether the plugin's reader sees the same workflow in
// both headers: the same value of any type, or none in both (a present key is
// never nil to the reader, so an absent key differs from every present one).
func sameWorkflow(before, after *taskfile.Frontmatter) bool {
	return reflect.DeepEqual(before.Data["workflow"], after.Data["workflow"])
}

// changedKey returns the first header key, in name order, that the edit did not
// set and whose value the plugin's reader sees differently in the two headers
// (present in one only, or of another value or type).
func changedKey(before, after *taskfile.Frontmatter, set map[string]bool) string {
	names := map[string]bool{}
	for key := range before.Data {
		names[key] = true
	}
	for key := range after.Data {
		names[key] = true
	}
	sorted := make([]string, 0, len(names))
	for key := range names {
		sorted = append(sorted, key)
	}
	sort.Strings(sorted)
	for _, key := range sorted {
		was, wasThere := before.Data[key]
		now, nowThere := after.Data[key]
		if !set[key] && (wasThere != nowThere || !reflect.DeepEqual(was, now)) {
			return key
		}
	}
	return ""
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
	return Event{Via: o.Via, Task: t.ID, Who: who, When: contract.FormatTime(now), What: o.Action, FromState: t.State, ToState: next, Note: o.Note}
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
	journalName = ".task-write.json"
	// approvalFile is the receipt of the plan approval that grid approve writes beside
	// the plan Job result, and that reopen overwrites with its own event.
	approvalFile = "approval.json"
)

var decisionFile = regexp.MustCompile(`^refusal-R-[1-9][0-9]*\.json$`)

// legacyReceipt names the per-gate receipt files of older plugins (no gate but the
// plan is approved any more); journal replay drops them.
var legacyReceipt = regexp.MustCompile(`^approval-(?:ship|preview)\.json$`)

// commitWrite uses the existing atomic writer. The durable redo journal is the
// commit point. Recovery is idempotent (full file images, including the appended
// history line), so a crash between renames never loses or duplicates a receipt.
func commitWrite(dir string, event Event, files map[string][]byte) error {
	return commitWriteLines(dir, event, files)
}

// commitWriteLines also appends the given prose history lines after the event,
// in the same journal, so a receipt and its readable line land together.
func commitWriteLines(dir string, event Event, files map[string][]byte, lines ...string) error {
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
	history = append(history, []byte("- "+string(encoded)+"\n")...)
	for _, line := range lines {
		history = append(history, []byte(line+"\n")...)
	}
	files["history.md"] = history
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
		// A journal an older plugin left before this one started may carry a per-gate
		// receipt file. Nothing reads those, so it is dropped and the rest replays.
		if legacyReceipt.MatchString(name) {
			delete(files, name)
			continue
		}
		if name != "task.md" && name != "history.md" && name != approvalFile && name != summaryFile && !decisionFile.MatchString(name) {
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
