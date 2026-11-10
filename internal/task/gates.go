package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/plan"
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
		// The task raises no gate any more, so a task waits only at its plan.
		// An old gate: line in task.md is ignored (Q10).
		if o.Gate != "" && o.Gate != "plan" {
			return fmt.Errorf("Changed since you approved: the task is at gate plan, not %s", o.Gate)
		}
		name, gate := approvalFile, ""
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
			return commitWrite(t.Path, event, map[string][]byte{name: append(data, '\n')})
		})
		if errors.Is(err, plan.ErrNoPlan) {
			return errors.New("plan.md is missing; write the plan before approving it")
		}
		return err
	})
	return event, err
}

// Approval answers whether the plan is approved, from the result of the plan
// Job, and only while its approved hash and digest match plan.md. Consumers
// must use this check (or Approved) and never treat a file's existence as
// permission. Plan writers hold .plan.lock, not .task.lock.
func (s Store) Approval(cwd, id string) (Event, error) {
	return s.ApprovalForGate(cwd, id, "plan")
}

// ApprovalForGate is Approval under the task and plan locks. The gate name is
// ignored: the plan Job is the only approval. The one exception is Q10, a task
// with no approve person Job at all (an old task in flight): it reads its
// receipt file for the named gate, as before, until its Job exists.
func (s Store) ApprovalForGate(cwd, id, gate string) (Event, error) {
	var event Event
	err := s.withWrite(cwd, id, func(t Task) error {
		found, jobErr := planJobResult(t.Path)
		if errors.Is(jobErr, ErrNoPlanJob) {
			var err error
			event, err = receiptForGate(t)
			return err
		}
		if t.State == "closed" {
			return errors.New("cannot use approval in current state closed")
		}
		err := plan.Store{TaskDir: t.Path}.WithDigest(func(hash, digest string) error {
			if jobErr != nil {
				return jobErr
			}
			if found.PlanHash != hash || found.PlanDigest != digest {
				return errChangedSinceApproved
			}
			event = found.event(t)
			return nil
		})
		if errors.Is(err, plan.ErrNoPlan) {
			// A missing plan matches no approval; a Job with no result still comes first.
			if err = jobErr; err == nil {
				err = plan.ErrNoPlan
			}
		}
		return err
	})
	return event, err
}

// receiptForGate is the Q10 fallback of ApprovalForGate: the approval.json
// receipt of a task that has no approve person Job. The gate name is ignored,
// as it is on the Job path; no preview or ship receipt is written any more. It
// runs under the task lock.
func receiptForGate(t Task) (Event, error) {
	var event Event
	data, err := readOptional(filepath.Join(t.Path, approvalFile))
	if err != nil {
		return event, err
	}
	err = plan.Store{TaskDir: t.Path}.WithDigest(func(hash, digest string) (err error) {
		event, err = checkApproval(t, data, hash, digest)
		return err
	})
	if errors.Is(err, plan.ErrNoPlan) {
		// A missing plan matches no approval; closed and never approved still come first.
		if _, err = checkApproval(t, data, "", ""); err == nil {
			err = plan.ErrNoPlan
		}
	}
	return event, err
}

// Approved is the lock-free form of Approval for a hook: it answers whether the
// plan Job's result still matches plan.md now (hash and full digest). It takes
// no lock and replays no journal, so a write in flight or any read failure is false.
func Approved(dir string) bool {
	text, err := plan.Store{TaskDir: dir}.Read()
	if err != nil {
		return false
	}
	return ApprovedDigest(dir, plan.Hash(text), plan.Digest(text))
}

// ApprovedDigest checks the plan Job's result against a caller's snapshot
// without rereading plan.md. Callers must compute both values from the text they
// use. Like Approved, it takes no locks and replays no journal. Only a task
// with no approve person Job (Q10, an old task) is answered from approval.json.
func ApprovedDigest(dir, hash, digest string) bool {
	t, err := Read(dir)
	if err != nil || t.State == "closed" {
		return false
	}
	found, err := planJobResult(dir)
	if errors.Is(err, ErrNoPlanJob) {
		data, err := readOptional(filepath.Join(dir, approvalFile))
		if err != nil {
			return false
		}
		_, err = checkApproval(t, data, hash, digest)
		return err == nil
	}
	return err == nil && digest != "" && found.PlanHash == hash && found.PlanDigest == digest
}

// JobApproval is what a person approved, as the result of the person Job for
// the plan carries it. Who and When come from the Feed record of the result.
type JobApproval struct {
	Job        string          `json:"job"`
	Record     string          `json:"record"`
	Who        contract.Author `json:"who"`
	When       string          `json:"when"`
	Answer     string          `json:"answer,omitempty"`
	PlanHash   string          `json:"plan_hash"`
	PlanDigest string          `json:"plan_digest"`
	LandsHash  string          `json:"lands_hash,omitempty"`
	Note       string          `json:"note,omitempty"`
}

// event is the approval as the receipt Event that the readers return.
func (a JobApproval) event(t Task) Event {
	return Event{Task: t.ID, Who: a.Who, When: a.When, What: "approve", Note: a.Note, LandsHash: a.LandsHash, PlanHash: a.PlanHash, PlanDigest: a.PlanDigest}
}

// ErrNoPlanJob means the task has no person Job of an approve step in its Feed:
// an old task made before the plan became a Job. Readers fall back to the
// approval.json receipt for that task only (decision Q10).
var ErrNoPlanJob = errors.New("plan has not been approved: the task has no plan Job")

var errChangedSinceApproved = errors.New("Changed since you approved: plan hash no longer matches")

// The plan Job is the person Job of the workflow's approve step: a Feed job
// record with role Person and kind approve. A task can have several such Jobs;
// the plan is approved by the newest accepted one whose result carries a
// plan_digest. A Job of another kind, or an accepted result without a
// plan_digest, is not a plan approval.
//
// planJobResult returns that newest result without comparing it to plan.md. It
// returns ErrNoPlanJob when the task has no such Job, and the error "the plan
// Job J-N has no result yet" when it has one that nobody has answered, which is
// what is left when approval.json was written and the post of the result failed.
// It reads the Feed read-only: no lock, no cache, no journal.
func planJobResult(dir string) (JobApproval, error) {
	var found JobApproval
	id, _, err := FeedIdentity(dir)
	if errors.Is(err, feed.ErrNoTaskID) {
		// No stable id means no Feed, so no Job: an old task (Q10).
		return found, ErrNoPlanJob
	}
	if err != nil {
		return found, err
	}
	store := feed.Store{TaskDir: dir, TaskID: id}
	read, err := store.ReadOnly(feed.Filter{})
	if err != nil {
		return found, err
	}
	allowed := store.WithdrawAllowed
	if allowed == nil {
		allowed = feed.QuestionWithdrawReadPolicy(dir)
	}
	state, _ := feed.FoldReadAt(read, time.Now(), allowed)
	admitted := func(verdict string) bool { return verdict == "counted" || verdict == "stale" || verdict == "unknown" }
	text := func(r feed.Entry, key string) string { s, _ := r.Body[key].(string); return s }
	var planJobs []string
	isPlanJob := map[string]bool{}
	// A reopen posts a new plan Job that continues the old one (Q8). Results
	// recorded before the newest such Job stay in the Feed as history but no
	// longer approve the plan.
	reopenAt, reopenJob := "", ""
	for _, r := range read.Records {
		jobID := text(r, "id")
		if r.Type == "job" && admitted(r.Verdict) && jobID != "" && !isPlanJob[jobID] && text(r, "kind") == "approve" && strings.EqualFold(strings.TrimSpace(text(r, "role")), "Person") {
			isPlanJob[jobID] = true
			planJobs = append(planJobs, jobID)
			if text(r, "continues") != "" && r.ID > reopenAt {
				reopenAt, reopenJob = r.ID, jobID
			}
		}
	}
	if len(planJobs) == 0 {
		return found, ErrNoPlanJob
	}
	sort.Strings(planJobs)
	for _, r := range read.Records {
		target := text(r, "target")
		j := state.Jobs[target]
		if r.Type != "control" || !admitted(r.Verdict) || !isPlanJob[target] || text(r, "status") != string(contract.Accepted) || text(r, "plan_digest") == "" || j == nil || j.Status != contract.Accepted || r.ID <= reopenAt {
			continue
		}
		// Record ids are ULIDs, so the greatest is the latest.
		if r.ID > found.Record {
			found = JobApproval{Job: target, Record: r.ID, Who: r.Author, When: r.At, Answer: text(r, "answer"), PlanHash: text(r, "plan_hash"), PlanDigest: text(r, "plan_digest"), LandsHash: text(r, "lands_hash"), Note: text(r, "note")}
		}
	}
	if found.Record == "" {
		if reopenJob != "" {
			planJobs[0] = reopenJob
		}
		return found, fmt.Errorf("plan has not been approved: the plan Job %s has no result yet; run grid approve to post it", planJobs[0])
	}
	return found, nil
}

// ApprovalFromJob reads the latest done result of the task's plan Job (see
// planJobResult) and reports whether its plan_hash and plan_digest match
// plan.md now. A nil error means they match. Like Approved, it takes no lock
// and replays no journal.
func ApprovalFromJob(dir string) (JobApproval, error) {
	found, err := planJobResult(dir)
	if err != nil {
		return found, err
	}
	current, err := plan.Store{TaskDir: dir}.Read()
	if err != nil {
		return found, err
	}
	if found.PlanHash != plan.Hash(current) || found.PlanDigest != plan.Digest(current) {
		return found, errChangedSinceApproved
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
