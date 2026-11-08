package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

var refusalID = regexp.MustCompile(`^R-[1-9][0-9]*$`)

// Approve records exactly the opaque plan.hash seen by the person. A supplied
// hash is a compare-and-set token. With no token, an existing stale approval
// must be explicitly replaced using --plan-hash after reviewing the new plan.
func (s Store) Approve(o WriteOptions, hash string) (Event, error) {
	var event Event
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State != "waiting" {
			return fmt.Errorf("cannot approve task in current state %q: expected waiting", t.State)
		}
		current, err := planHash(t.Path)
		if err != nil {
			return err
		}
		if hash == "" {
			previous, err := readOptional(filepath.Join(t.Path, "approval.json"))
			if err != nil {
				return err
			}
			if previous != nil {
				var old Event
				if err := json.Unmarshal(previous, &old); err != nil {
					return err
				}
				if old.PlanHash != current {
					return errors.New("Changed since you approved: review the plan and pass --plan-hash with its current hash")
				}
			}
			hash = current
		}
		if hash != current {
			return fmt.Errorf("plan hash no longer matches: approved %q, current %q", hash, current)
		}
		o.Action = "approve"
		event = s.event(t, o, t.State)
		event.PlanHash = hash
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return commitWrite(t.Path, event, map[string][]byte{"approval.json": append(data, '\n')})
	})
	return event, err
}

// Approval reads a gate only while its approved hash still matches plan.hash.
// Consumers must use this check instead of treating approval.json's existence
// as permission. Plan writers must also use the task's .task.lock.
func (s Store) Approval(cwd, id string) (Event, error) {
	var event Event
	err := s.withWrite(cwd, id, func(t Task) error {
		if t.State == "closed" {
			return errors.New("cannot use approval in current state closed")
		}
		data, err := readOptional(filepath.Join(t.Path, "approval.json"))
		if err != nil {
			return err
		}
		if data == nil {
			return errors.New("plan has not been approved")
		}
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		hash, err := planHash(t.Path)
		if err != nil {
			return err
		}
		if event.Task != t.ID || event.What != "approve" || event.PlanHash != hash {
			return errors.New("Changed since you approved: plan hash no longer matches")
		}
		return nil
	})
	return event, err
}

func planHash(dir string) (string, error) {
	data, err := readOptional(filepath.Join(dir, "plan.hash"))
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(string(data))
	if hash == "" {
		return "", errors.New("plan.hash is missing or empty; prepare a plan before approving")
	}
	return hash, nil
}

// Decide records a task-scoped allow or deny for a known refusal. It never runs
// the command, changes agent settings or grants a project-wide permission.
// Refusal producers own refusals/R-n.json; this writer only checks its existence.
func (s Store) Decide(o WriteOptions, id, decision string) (Event, error) {
	var event Event
	if !refusalID.MatchString(id) {
		return event, errors.New("invalid refusal id: use R-N")
	}
	if decision != "allow" && decision != "deny" {
		return event, errors.New("refusal decision must be allow or deny")
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if _, ok := StateMoves()[t.State]; !ok || t.State == "closed" {
			return fmt.Errorf("cannot decide refusal in current state %q", t.State)
		}
		dir := filepath.Join(t.Path, "refusals")
		if err := safeDir(gridhome.Real(s.Resolver.GridHome), dir); err != nil {
			return err
		}
		refusal, err := readOptional(filepath.Join(dir, id+".json"))
		if err != nil {
			return err
		}
		if refusal == nil {
			return fmt.Errorf("refusal %s not found", id)
		}
		name := "refusal-" + id + ".json"
		previous, err := readOptional(filepath.Join(t.Path, name))
		if err != nil {
			return err
		}
		if previous != nil {
			return fmt.Errorf("refusal %s already decided", id)
		}
		o.Action = "refusal decide"
		event = s.event(t, o, t.State)
		event.RefusalID, event.Decision = id, decision
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		return commitWrite(t.Path, event, map[string][]byte{name: append(data, '\n')})
	})
	return event, err
}
