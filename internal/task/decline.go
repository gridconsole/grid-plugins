package task

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

// DeclineAllowed is the default role policy for declining tasks. A host or the
// workflow may replace it during initialization, before concurrent writes begin.
var DeclineAllowed = defaultTaskEditAllowed

// DeclineOptions carries the required reason and an optional project constraint.
// Role is supplied by the host; Who.ID names its person, or is empty for "you".
type DeclineOptions struct {
	WriteOptions
	Reason, Project, Role string
}

// Decline closes a non-planner task and records the reason through Change's
// locked reread, journal and atomic commit path.
func (s Store) Decline(o DeclineOptions) (Event, error) {
	reason := strings.Join(strings.Fields(o.Reason), " ")
	if reason == "" || hasEditControls(o.Reason) || !utf8.ValidString(o.Reason) {
		return Event{}, fmt.Errorf("%w: a decline needs a reason without control or bidi override/isolate characters", ErrEditUsage)
	}
	if !DeclineAllowed(strings.TrimSpace(o.Role)) {
		return Event{}, errors.New("scope: role cannot decline tasks")
	}
	who := strings.Join(strings.Fields(o.Who.ID), " ")
	if strings.ContainsRune(who, '\x00') || !utf8.ValidString(o.Who.ID) {
		return Event{}, errors.New("host person must be valid text without NUL")
	}
	if who == "" {
		who = "you"
	}
	if o.Who.ID == "" {
		o.Who.ID = "local"
		if o.Who.Role == "" {
			o.Who.Role = "owner"
		}
	}
	o.Action, o.Note = "decline", "closed · declined · "+reason
	return s.editTask(o.WriteOptions, o.Project, func(t Task, mapping *yaml.Node, event *Event) (map[string]string, error) {
		if t.State == "closed" {
			return nil, fmt.Errorf("%w: %s is already closed", ErrEditRejected, t.ID)
		}
		var record struct {
			Kind string `yaml:"kind"`
		}
		if err := mapping.Decode(&record); err != nil {
			return nil, err
		}
		if record.Kind == "planner" {
			return nil, fmt.Errorf("%w: %s is the project's standing Planner; it cannot be declined", ErrEditRejected, t.ID)
		}
		event.ToState = "closed"
		return map[string]string{"state": "closed", "next": "declined by " + who, "updated": event.When}, nil
	})
}
