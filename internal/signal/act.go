package signal

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// BadInputError identifies an invalid act request before any note is opened.
type BadInputError struct{ Message string }

func (e *BadInputError) Error() string { return e.Message }

// RefusedError identifies a valid request refused by caller or note state.
type RefusedError struct {
	Message string
	Cause   error
}

func (e *RefusedError) Error() string { return e.Message }
func (e *RefusedError) Unwrap() error { return e.Cause }

// ErrNoNote identifies a missing signal or inbox note.
var ErrNoNote = errors.New("no such note")

// ActOptions carries the decision and the same task author context as task new.
// TaskSet and UntilSet distinguish explicitly empty flags from absent flags.
type ActOptions struct {
	Act, Task, Until         string
	TaskSet, UntilSet, Later bool
	Cwd, Who, Role           string
}

const localMinute = "2006-01-02T15:04"

var taskIDPattern = regexp.MustCompile(`^T-[0-9]+$`)

func (o ActOptions) normalized(id string) (ActOptions, error) {
	bad := func(message string) (ActOptions, error) { return o, &BadInputError{Message: message} }
	if !validID(id) {
		return bad("invalid signal id")
	}
	switch o.Act {
	case "turn", "add", "note", "snooze", "dismiss", "wake", "reopen":
	default:
		return bad(fmt.Sprintf("unknown signal act %q", o.Act))
	}
	if o.Later {
		return bad("--later is not supported")
	}
	if (o.TaskSet || o.Task != "") && o.Act != "add" {
		return bad("--task is only valid for add")
	}
	if (o.UntilSet || o.Until != "") && o.Act != "snooze" {
		return bad("--until is only valid for snooze")
	}
	if o.Act == "add" && !taskIDPattern.MatchString(o.Task) {
		return bad("add requires --task T-<n>")
	}
	if o.Act == "snooze" {
		parsed, err := time.Parse(time.RFC3339, o.Until)
		if err != nil {
			parsed, err = time.ParseInLocation(localMinute, o.Until, time.Local)
		}
		if err != nil {
			return bad("snooze requires --until RFC3339 or YYYY-MM-DDTHH:MM")
		}
		o.Until = parsed.In(time.Local).Format(localMinute)
	}
	return o, nil
}

// Validate checks an act without reading the project or note.
func (o ActOptions) Validate(id string) error { _, err := o.normalized(id); return err }

// Act keeps a signal without starting work; it is the historical default verb.
func (s Store) Act(id string) (Result, error) { return s.Apply(id, ActOptions{Act: "note"}) }

// Dismiss keeps the note and its deduplication identity, marking it dismissed.
func (s Store) Dismiss(id string) (Result, error) { return s.Apply(id, ActOptions{Act: "dismiss"}) }

// Apply serializes state checks, task creation and note publication under the
// signals lock. The task allocation lock is always nested inside this lock.
// Defaults correspond to inbox_act_roles, inbox_acts and inbox_turn_body; no
// workflow configuration is read here. The turn body policy is link.
func (s Store) Apply(id string, o ActOptions) (Result, error) {
	if s.Resolver.Caller.IsGuest() {
		return Result{}, &RefusedError{Message: "guests cannot act on signals"}
	}
	o, err := o.normalized(id)
	if err != nil {
		return Result{}, err
	}
	if _, err := s.notes(); err != nil {
		return Result{}, err
	}
	unlock, err := localdata.Lock(s.GridHome, ".signals.lock")
	if err != nil {
		return Result{}, err
	}
	defer unlock()
	// Recheck confinement under the lock, alongside the state read.
	path, err := s.path(id)
	if err != nil {
		return Result{}, err
	}
	n, err := read(path)
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, &RefusedError{Message: "no such note", Cause: ErrNoNote}
	}
	if err != nil {
		return Result{}, err
	}
	open := openStatus(n.Status)
	switch {
	case o.Act == "wake" && n.Status != "snoozed":
		return Result{}, &RefusedError{Message: "this note is not snoozed"}
	case o.Act == "reopen" && open:
		return Result{}, &RefusedError{Message: "this note is not decided"}
	case o.Act == "note" && n.Status == "kept", o.Act == "dismiss" && n.Status == "dismissed":
		return result(s, n, "unchanged"), nil
	case o.Act != "wake" && o.Act != "reopen" && !open:
		return Result{}, &RefusedError{Message: fmt.Sprintf("this note is already decided (%s)", n.Status)}
	}
	if _, err := localdata.Machine(s.GridHome); err != nil {
		return Result{}, err
	}
	n.Until = ""
	switch o.Act {
	case "turn", "add":
		n.Status, n.Became = "done", o.Task
	case "note":
		n.Status = "kept"
	case "dismiss":
		n.Status = "dismissed"
	case "snooze":
		n.Status, n.Until = "snoozed", o.Until
	case "wake":
		n.Status = "new"
	case "reopen":
		n.Status, n.Became = "new", ""
	}
	n.Updated = contract.FormatTime(time.Now())
	if o.Act == "turn" {
		// Reserve room for any allocatable task id before creating a task. Encoding
		// errors must not create tasks that cannot be linked back to their signal.
		n.Became = "T-18446744073709551615"
		if _, err := encode(n); err != nil {
			return Result{}, err
		}
		if err := textField("title", n.Title, 1024); err != nil {
			return Result{}, err
		}
		r := s.Resolver
		r.GridHome = s.GridHome
		made, err := (task.Store{Resolver: r}).New(task.NewOptions{
			Cwd: o.Cwd, Project: s.Project, Who: o.Who, Role: o.Role,
			Words: n.Title + "\n\nSignal: projects/" + s.Project + "/notes/" + id + ".md",
		})
		if err != nil {
			return Result{}, err
		}
		n.Became = made.ID
	}
	data, err := encode(n)
	if err != nil {
		return Result{}, err
	}
	if err := atomicfile.Write(path, data, 0o600); err != nil {
		return Result{}, err
	}
	return result(s, n, "updated"), nil
}

func openStatus(status string) bool {
	return status == "new" || status == "placed" || status == "snoozed"
}
