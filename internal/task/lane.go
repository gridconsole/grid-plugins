package task

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Lanes are the board lanes a person can move a task to, in board order: the
// order decides whether a move is "back". The lane is a front-matter key
// (`lane:`) the board reads before it derives a lane from the step.
var Lanes = []string{"inbox", "planning", "doing", "delivering"}

var laneNames = map[string]string{
	"inbox":      "Waiting to start",
	"planning":   "Planning",
	"doing":      "Doing",
	"delivering": "Delivering",
}

// ValidLane reports whether lane is one of Lanes.
func ValidLane(lane string) bool { return slices.Contains(Lanes, lane) }

// LaneOptions names the task and the lane a person moved it to. Project, when
// set, must be the project the task is in.
type LaneOptions struct {
	WriteOptions
	Lane, Project string
}

// LaneResult is the task after a lane move.
type LaneResult struct {
	Event
	Lane string
}

// CheckProject refuses a project name that is not registered, so a caller can
// answer that before it looks at the task or the lane.
func (s Store) CheckProject(cwd, project string) error {
	_, err := s.resolve(cwd, project)
	return err
}

// SetLane records the lane a person moved a task to: it sets `lane` in task.md
// and keeps every other key, the body, the plan and the step. A move to inbox
// also sets the state to queued where StateMoves allows that move. A closed
// task, the project's standing Planner and the lane the task is already in are
// refused with nothing written. The move and its history line share Change's
// lock, journal and atomic commit.
func (s Store) SetLane(o LaneOptions) (LaneResult, error) {
	var result LaneResult
	if !ValidLane(o.Lane) {
		return result, fmt.Errorf("%w: lane must be one of %s", ErrEditUsage, strings.Join(Lanes, ", "))
	}
	o.Action = "lane"
	if err := o.WriteOptions.validateText(); err != nil {
		return result, err
	}
	project := ""
	if o.Project != "" {
		resolved, err := s.resolve(o.Cwd, o.Project)
		if err != nil {
			return result, err
		}
		project = resolved.Project
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if project != "" && t.Project != project {
			return fmt.Errorf("scope: task %s is in project %q, not %q", t.ID, t.Project, project)
		}
		data, err := readOptional(filepath.Join(t.Path, "task.md"))
		if err != nil {
			return err
		}
		next, note := t.State, ""
		updated, err := rewriteTask(data, func(mapping *yaml.Node) error {
			var record struct {
				Kind string `yaml:"kind"`
				Lane string `yaml:"lane"`
			}
			if err := mapping.Decode(&record); err != nil {
				return err
			}
			switch {
			case t.State == "closed":
				return fmt.Errorf("%w: %s is closed; reopen it before moving it to a lane", ErrEditRejected, t.ID)
			case record.Kind == "planner":
				return fmt.Errorf("%w: %s is the project's standing Planner; it has no lane to move to", ErrEditRejected, t.ID)
			case record.Lane == o.Lane:
				return fmt.Errorf("%w: %s is already in the %s lane", ErrEditRejected, t.ID, o.Lane)
			}
			if o.Lane == "inbox" && t.State != "queued" {
				moved, err := NextState(t.State, "status", "queued")
				if err != nil {
					return fmt.Errorf("%w: %s is %s and cannot move to queued, so it cannot move to the inbox lane", ErrEditRejected, t.ID, t.State)
				}
				next = moved
				setScalar(mapping, "state", next, false)
			}
			setScalar(mapping, "lane", o.Lane, true)
			note = "moved to " + laneNames[o.Lane]
			if was := slices.Index(Lanes, record.Lane); was > slices.Index(Lanes, o.Lane) {
				note = "moved back to " + laneNames[o.Lane] + " (plan and step kept)"
			}
			return nil
		}, nil)
		if err != nil {
			return err
		}
		o.Note = note
		result.Event = s.event(t, o.WriteOptions, next)
		result.Lane = o.Lane
		if _, err := Fold(t.Path, string(updated), t.Intent); err != nil {
			return err
		}
		return commitWrite(t.Path, result.Event, map[string][]byte{"task.md": updated})
	})
	return result, err
}
