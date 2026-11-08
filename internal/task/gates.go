package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

var refusalID = regexp.MustCompile(`^R-[1-9][0-9]*$`)

// Approve records the plan.md short hash and full digest the person saw. A supplied hash is a
// compare-and-set token. With no token, an existing stale approval must be
// explicitly replaced using --plan-hash after reviewing the new plan.
//
// The gate the task raised decides what is recorded: a task at gate ship (set
// by deliver) gets a ship approval in approval-ship.json, and any other waiting
// task a plan approval in approval.json. The two never overwrite each other.
//
// The hash comes from plan.md through internal/plan, never from a stored copy.
// Locks are taken in one order: .task.lock (withWrite), then .plan.lock (WithDigest),
// which only plan writers also take; nothing takes them the other way round.
func (s Store) Approve(o WriteOptions, hash string) (Event, error) {
	var event Event
	if err := checkTaskText("plan hash", hash, false); err != nil {
		return event, err
	}
	if err := o.validateText(); err != nil {
		return event, err
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State != "waiting" {
			return fmt.Errorf("cannot approve task in current state %q: expected waiting", t.State)
		}
		name, gate := approvalFile, ""
		if t.Gate == "ship" {
			name, gate = shipApprovalFile, "ship"
		}
		previous, err := readOptional(filepath.Join(t.Path, name))
		if err != nil {
			return err
		}
		err = plan.Store{TaskDir: t.Path}.WithDigest(func(current, digest string) error {
			want := hash
			if want == "" {
				if previous != nil {
					var old Event
					if err := readApproval(previous, &old); err != nil {
						return err
					}
					if old.PlanHash != current || (old.PlanDigest != "" && old.PlanDigest != digest) || old.LandsHash != o.LandsHash {
						return errors.New("Changed since you approved: review the plan and pass --plan-hash with its current hash")
					}
				}
				want = current
			}
			if want != current {
				return fmt.Errorf("plan hash no longer matches: approved %q, current %q", want, current)
			}
			o.Action = "approve"
			event = s.event(t, o, t.State)
			event.PlanHash, event.PlanDigest, event.Gate, event.LandsHash = want, digest, gate, o.LandsHash
			data, err := json.Marshal(event)
			if err != nil {
				return err
			}
			return commitWrite(t.Path, event, map[string][]byte{name: append(data, '\n')})
		})
		if errors.Is(err, plan.ErrNoPlan) {
			return errors.New("plan.md is missing; write the plan before approving it")
		}
		return err
	})
	return event, err
}

// Approval reads a gate only while its approved hash and optional digest match plan.md.
// Consumers must use this check (or Approved) instead of treating approval.json's
// existence as permission. Plan writers hold .plan.lock, not .task.lock.
func (s Store) Approval(cwd, id string) (Event, error) {
	return s.ApprovalForGate(cwd, id, "plan")
}

// ApprovalForGate validates the selected plan or ship receipt under the task and plan locks.
func (s Store) ApprovalForGate(cwd, id, gate string) (Event, error) {
	name, eventGate := approvalFile, ""
	switch gate {
	case "plan":
	case "ship":
		name, eventGate = shipApprovalFile, "ship"
	default:
		return Event{}, fmt.Errorf("unknown approval gate %q", gate)
	}
	var event Event
	err := s.withWrite(cwd, id, func(t Task) error {
		data, err := readOptional(filepath.Join(t.Path, name))
		if err != nil {
			return err
		}
		err = plan.Store{TaskDir: t.Path}.WithDigest(func(hash, digest string) (err error) {
			event, err = checkApproval(t, data, hash, digest)
			if err == nil && event.Gate != eventGate {
				return fmt.Errorf("approval is for another gate")
			}
			return err
		})
		if errors.Is(err, plan.ErrNoPlan) {
			// A missing plan matches no approval; closed and never approved still come first.
			if _, err = checkApproval(t, data, "", ""); err == nil {
				err = plan.ErrNoPlan
			}
		}
		return err
	})
	return event, err
}

// Approved is the lock-free form of Approval for a hook: it answers whether dir
// holds an approval whose hash and optional digest still match plan.md now. It takes no lock and
// replays no journal, so a write in flight or any read failure is false.
func Approved(dir string) bool {
	t, err := Read(dir)
	if err != nil {
		return false
	}
	data, err := readOptional(filepath.Join(dir, "approval.json"))
	if err != nil {
		return false
	}
	text, err := plan.Store{TaskDir: dir}.Read()
	if err != nil {
		return false
	}
	digest := plan.Digest(text)
	event, err := checkApproval(t, data, digest[:6], digest)
	// Match ApprovalForGate: the plan receipt has no gate field. A receipt for
	// ship or an unknown gate cannot authorize plan work, even if misfiled.
	return err == nil && event.Gate == ""
}

func checkApproval(t Task, data []byte, hash, digest string) (Event, error) {
	var event Event
	if t.State == "closed" {
		return event, errors.New("cannot use approval in current state closed")
	}
	if data == nil {
		return event, errors.New("plan has not been approved")
	}
	if err := readApproval(data, &event); err != nil {
		return Event{}, err
	}
	if event.Task != t.ID || event.What != "approve" || event.PlanHash != hash || (event.PlanDigest != "" && event.PlanDigest != digest) {
		return Event{}, errors.New("Changed since you approved: plan hash no longer matches")
	}
	return event, nil
}

// readApproval distinguishes an absent legacy digest from a present but empty,
// null or malformed one. Only absence permits the short-hash compatibility path.
func readApproval(data []byte, event *Event) error {
	var receipt struct {
		*Event
		Digest json.RawMessage `json:"plan_digest"`
	}
	receipt.Event = event
	if err := json.Unmarshal(data, &receipt); err != nil {
		return err
	}
	if receipt.Digest != nil {
		if err := json.Unmarshal(receipt.Digest, &event.PlanDigest); err != nil {
			return err
		}
		if len(event.PlanDigest) != 64 {
			return errors.New("invalid plan digest in approval")
		}
	}
	return nil
}

// Decide records a task-scoped allow or deny for a known refusal. It never runs
// the command, changes agent settings or grants a project-wide permission.
// Refusal producers own refusals/R-n.json; this writer only checks its existence.
func (s Store) Decide(o WriteOptions, id, decision string) (Event, error) {
	var event Event
	if err := o.validateText(); err != nil {
		return event, err
	}
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
