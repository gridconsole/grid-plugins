package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

var refusalID = regexp.MustCompile(`^R-[1-9][0-9]*$`)

var planHashToken = regexp.MustCompile(`^([0-9a-f]{6}|[0-9a-f]{64})$`)

// Approve records the plan.md short hash and full digest the person saw. A supplied
// short hash or full digest is a compare-and-set token. With no token, an existing
// stale approval must be explicitly replaced using --plan-hash after reviewing the new plan.
//
// The gate the task raised decides what is recorded: a task at gate ship (set
// by deliver) gets a ship approval in approval-ship.json, a task at gate preview
// a preview approval in approval-preview.json plus a "preview approved" history
// line, and any other waiting task a plan approval in approval.json. The three
// never overwrite each other. A caller that names the gate the person saw
// (o.Gate) is refused when the task now waits at another gate, and a repeat
// whose receipt already matches the plan writes nothing.
//
// The hash comes from plan.md through internal/plan, never from a stored copy.
// Locks are taken in one order: .task.lock (withWrite), then .plan.lock (WithDigest),
// which only plan writers also take; nothing takes them the other way round.
func (s Store) Approve(o WriteOptions, hash string) (Event, error) {
	var event Event
	// Validate before withWrite can take a lock or replay a pending journal.
	switch o.PlanHashToken {
	case "", "any", "digest":
	default:
		return event, fmt.Errorf("%w: plan_hash_token must be any or digest", ErrEditUsage)
	}
	if hash != "" {
		if !planHashToken.MatchString(hash) {
			return event, fmt.Errorf("%w: plan hash must be 6 or 64 lowercase hex digits", ErrEditUsage)
		}
		if o.PlanHashToken == "digest" && len(hash) != 64 {
			return event, fmt.Errorf("%w: plan_hash_token digest requires 64 lowercase hex digits", ErrEditUsage)
		}
	}
	switch o.Gate {
	case "", "plan", "preview", "ship":
	default:
		return event, fmt.Errorf("%w: gate must be plan, preview or ship", ErrEditUsage)
	}
	if err := o.validateText(); err != nil {
		return event, err
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State != "waiting" {
			return fmt.Errorf("cannot approve task in current state %q: expected waiting", t.State)
		}
		name, gate, at := approvalFile, "", "plan"
		switch t.Gate {
		case "ship":
			name, gate, at = shipApprovalFile, "ship", "ship"
		case "preview":
			name, gate, at = previewApprovalFile, "preview", "preview"
		}
		if o.Gate != "" && o.Gate != at {
			return fmt.Errorf("Changed since you approved: the task is at gate %s, not %s", at, o.Gate)
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
					if old.PlanHash != current || old.PlanDigest != digest || old.LandsHash != o.LandsHash {
						return errors.New("Changed since you approved: review the plan and pass --plan-hash with its current hash")
					}
				}
				want = current
			}
			match := current
			if len(want) == 64 {
				match = digest
			}
			if want != match {
				return fmt.Errorf("plan hash no longer matches: approved %q, current %q", want, current)
			}
			// A repeat press on a receipt that already matches returns it and writes nothing.
			if old, err := checkApproval(t, previous, current, digest); err == nil && old.Gate == gate && old.LandsHash == o.LandsHash {
				event = old
				return nil
			}
			o.Action = "approve"
			event = s.event(t, o, t.State)
			event.PlanHash, event.PlanDigest, event.Gate, event.LandsHash = current, digest, gate, o.LandsHash
			data, err := json.Marshal(event)
			if err != nil {
				return err
			}
			var lines []string
			if gate == "preview" {
				lines = append(lines, previewApprovedLine(event, s.clock()))
			}
			return commitWriteLines(t.Path, event, map[string][]byte{name: append(data, '\n')}, lines...)
		})
		if errors.Is(err, plan.ErrNoPlan) {
			return errors.New("plan.md is missing; write the plan before approving it")
		}
		// A stale hash has already returned, so neither the receipt nor the
		// Job result is written. A repeat press posts a result still missing.
		if err == nil && o.JobResult != nil {
			err = o.JobResult(event)
		}
		return err
	})
	return event, err
}

// previewApprovedLine is the prose line the plan guard watches for: only Grid
// writes it, here, with the receipt.
func previewApprovedLine(e Event, now time.Time) string {
	return taskfile.HistoryLine(now.UTC().Format("2006-01-02T15:04"), "preview approved", e.PlanHash, CleanHistory(e.Who.ID), CleanHistory(e.Via))
}

// Approval reads a gate only while its approved hash and optional digest match plan.md.
// Consumers must use this check (or Approved) instead of treating approval.json's
// existence as permission. Plan writers hold .plan.lock, not .task.lock.
func (s Store) Approval(cwd, id string) (Event, error) {
	return s.ApprovalForGate(cwd, id, "plan")
}

// ApprovalForGate validates the selected plan, preview or ship receipt under the task and plan locks.
func (s Store) ApprovalForGate(cwd, id, gate string) (Event, error) {
	name, eventGate := approvalFile, ""
	switch gate {
	case "plan":
	case "preview":
		name, eventGate = previewApprovalFile, "preview"
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
// holds an approval whose hash and full digest still match plan.md now. It takes no lock and
// replays no journal, so a write in flight or any read failure is false.
func Approved(dir string) bool {
	text, err := plan.Store{TaskDir: dir}.Read()
	if err != nil {
		return false
	}
	return ApprovedDigest(dir, plan.Hash(text), plan.Digest(text))
}

// ApprovedDigest checks the plan receipt against a caller's snapshot without
// rereading plan.md. Callers must compute both values from the text they use.
// Like Approved, it takes no locks and replays no journal.
func ApprovedDigest(dir, hash, digest string) bool {
	t, err := Read(dir)
	if err != nil {
		return false
	}
	data, err := readOptional(filepath.Join(dir, approvalFile))
	if err != nil {
		return false
	}
	event, err := checkApproval(t, data, hash, digest)
	// Match ApprovalForGate: the plan receipt has no gate field. A receipt for
	// ship or an unknown gate cannot authorize plan work, even if misfiled.
	return err == nil && event.Gate == ""
}

// JobApproval is what a person approved, as the result of the person Job for
// the plan carries it. The Feed record carries who and when.
type JobApproval struct {
	Job        string `json:"job"`
	Record     string `json:"record"`
	Answer     string `json:"answer,omitempty"`
	PlanHash   string `json:"plan_hash"`
	PlanDigest string `json:"plan_digest"`
	LandsHash  string `json:"lands_hash,omitempty"`
	Note       string `json:"note,omitempty"`
}

// ApprovalFromJob reads the latest done result of the task's plan Job, the
// newest accepted result of a person Job that carries a plan_digest, and
// reports whether its plan_hash and plan_digest match plan.md now. A nil error
// means they match. Like Approved, it takes no lock and replays no journal.
func ApprovalFromJob(dir string) (JobApproval, error) {
	var found JobApproval
	id, _, err := FeedIdentity(dir)
	if err != nil {
		return found, err
	}
	store := feed.Store{TaskDir: dir, TaskID: id}
	state, err := store.State()
	if err != nil {
		return found, err
	}
	read, err := store.ReadOnly(feed.Filter{Type: "control"})
	if err != nil {
		return found, err
	}
	for _, r := range read.Records {
		text := func(key string) string { s, _ := r.Body[key].(string); return s }
		j := state.Jobs[text("target")]
		if r.Type != "control" || text("status") != string(contract.Accepted) || text("plan_digest") == "" || j == nil || !strings.EqualFold(strings.TrimSpace(j.Role), "Person") || j.Status != contract.Accepted {
			continue
		}
		// Record ids are ULIDs, so the greatest is the latest.
		if r.ID > found.Record {
			found = JobApproval{Job: j.ID, Record: r.ID, Answer: text("answer"), PlanHash: text("plan_hash"), PlanDigest: text("plan_digest"), LandsHash: text("lands_hash"), Note: text("note")}
		}
	}
	if found.Record == "" {
		return found, errors.New("plan has not been approved")
	}
	current, err := plan.Store{TaskDir: dir}.Read()
	if err != nil {
		return found, err
	}
	if found.PlanHash != plan.Hash(current) || found.PlanDigest != plan.Digest(current) {
		return found, errors.New("Changed since you approved: plan hash no longer matches")
	}
	return found, nil
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
	if event.Task != t.ID || event.What != "approve" || event.PlanHash != hash || event.PlanDigest == "" || event.PlanDigest != digest {
		return Event{}, errors.New("Changed since you approved: plan hash no longer matches")
	}
	return event, nil
}

// readApproval decodes legacy receipts so explicit reapproval can replace them;
// checkApproval requires a matching full digest before authorizing plan work.
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
