package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

var planHashToken = regexp.MustCompile(`^([0-9a-f]{6}|[0-9a-f]{64})$`)

// Approve records the plan.md short hash and full digest the person saw. A supplied
// short hash or full digest is a compare-and-set token. With no token, an existing
// stale approval must be explicitly replaced using --plan-hash after reviewing the new plan.
//
// The approval itself is the result of the task's approve person Job in the Feed:
// Approve does the stale-hash check and the compare-and-set against that result
// and writes the history line, and its caller posts the result. A repeat whose
// result already matches the plan writes nothing.
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
	if err := o.validateText(); err != nil {
		return event, err
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if t.State != "waiting" {
			return fmt.Errorf("cannot approve task in current state %q: expected waiting", t.State)
		}
		// The previous approval is the newest result of the plan Job. A task with no
		// plan Job, or a Job nobody has answered, has none.
		previous, jobErr := planJobResult(t.Path)
		if jobErr != nil && !errors.Is(jobErr, ErrNoPlanJob) && !errors.Is(jobErr, ErrPlanJobOpen) {
			return jobErr
		}
		err := plan.Store{TaskDir: t.Path}.WithDigest(func(current, digest string) error {
			want := hash
			if want == "" {
				if jobErr == nil {
					if previous.PlanHash != current || previous.PlanDigest != digest || previous.LandsHash != o.LandsHash {
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
			// A repeat press on a result that already matches returns it and writes nothing.
			if jobErr == nil && previous.PlanHash == current && previous.PlanDigest == digest && previous.LandsHash == o.LandsHash {
				event = previous.event(t)
				return nil
			}
			// The history line and the approval.json receipt are written here: the caller
			// records the approval as the result of the plan Job, once this has released
			// the task and plan locks. The receipt file is a copy for readers of the
			// task folder (the app reads it); the plan Job result stays the approval.
			o.Action = "approve"
			event = s.event(t, o, t.State)
			event.PlanHash, event.PlanDigest, event.LandsHash = current, digest, o.LandsHash
			data, err := json.Marshal(event)
			if err != nil {
				return err
			}
			return commitWrite(t.Path, event, map[string][]byte{approvalFile: append(data, '\n')})
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
// ignored: the plan Job is the only approval, and a task with no approve person
// Job has none (ErrNoPlanJob).
func (s Store) ApprovalForGate(cwd, id, gate string) (Event, error) {
	var event Event
	err := s.withWrite(cwd, id, func(t Task) error {
		found, jobErr := planJobResult(t.Path)
		if t.State == "closed" {
			return errors.New("cannot use approval in current state closed")
		}
		if errors.Is(jobErr, ErrNoPlanJob) {
			return jobErr
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
// use. Like Approved, it takes no locks and replays no journal. A task with no
// approve person Job is not approved.
func ApprovedDigest(dir, hash, digest string) bool {
	t, err := Read(dir)
	if err != nil || t.State == "closed" {
		return false
	}
	found, err := planJobResult(dir)
	return err == nil && digest != "" && found.PlanHash == hash && found.PlanDigest == digest
}

// HasPlanJob reports whether the task has a person Job of an approve step, with
// or without a result. A Feed that cannot be read counts as having one, so a
// reader never falls back to the legacy history line on a read failure.
func HasPlanJob(dir string) bool {
	_, err := planJobResult(dir)
	return !errors.Is(err, ErrNoPlanJob)
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

// ErrNoPlanJob means the task has no person Job of an approve step in its Feed.
// Such a task is not approved; an old receipt file is moved into a Job by
// migrate.ReceiptToJob, never read here.
var ErrNoPlanJob = errors.New("plan has not been approved: the task has no plan Job")

// ErrPlanJobOpen is what planJobResult returns for a task whose approve person
// Job nobody has answered yet.
var ErrPlanJobOpen = errors.New("plan Job has no result yet")

type planJobOpenError struct{ job string }

func (e planJobOpenError) Error() string {
	return fmt.Sprintf("plan has not been approved: the plan Job %s has no result yet; run grid approve to post it", e.job)
}

func (e planJobOpenError) Is(target error) bool { return target == ErrPlanJobOpen }

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
// what is left when the Job was posted and the post of its result failed.
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
		return found, planJobOpenError{planJobs[0]}
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
