package orchestrator

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
)

// An empty finish is an agent that ended done and posted no result of its attempt. The Stop
// hook asks every Job's agent for a result (a Job it cannot do is ended with the blocker named
// in the result), so an empty finish is a failed attempt, and a Job that is Blocked is not one
// (the fold makes it Blocked before the tick sees it). The tick retries it as a new attempt and
// asks the person after emptyFinishLimit of them; it never moves the Job to Submitted, because
// the app's Judge judges every submitted Job and would review or accept one with nothing in it.

// emptyFinishLimit is how many empty finishes of one Job the tick retries before it parks the
// Job and asks the person.
const emptyFinishLimit = 3

// ownerRetryReason is the reason of the control that puts a parked Job back to Ready after the
// owner answered Retry.
const ownerRetryReason = "owner requested retry"

// hasResultOf reports whether the Job has a counted result of this attempt, the test the fold
// makes (contract resultOf) before it folds Submitted.
func hasResultOf(snap Snapshot, j *contract.Job, attempt *int) bool {
	if attempt == nil {
		return false
	}
	for _, id := range j.Results {
		if a := snap.Records[id].Attempt; a != nil && *a == *attempt {
			return true
		}
	}
	return false
}

// emptyFinish reports whether the Job's latest attempt ended done with no result. A Job an older
// tick moved to Submitted for that reason is one too, so it is found and retried.
func emptyFinish(snap Snapshot, j *contract.Job) bool {
	if j.Exited == "" || j.ExitReason != "done" {
		return false
	}
	attempt := j.Attempt
	return !hasResultOf(snap, j, &attempt)
}

// emptyFinishes is the worker-exit of every attempt of the Job that ended done with no result
// since the owner last answered Retry, oldest first. The limit counts these from the Feed, not
// from the crash counter, which progress and a checkpoint clear: an agent that checkpoints and
// then ends empty each time would otherwise be retried for ever.
func emptyFinishes(snap Snapshot, j *contract.Job) []string {
	var out []string
	seen := map[int]bool{}
	for _, id := range j.History {
		r := snap.Records[id]
		switch {
		case r.Type == "worker-exit" && stringValue(r.Body["reason"]) == "done" && r.Attempt != nil && !seen[*r.Attempt]:
			seen[*r.Attempt] = true
			if !hasResultOf(snap, j, r.Attempt) {
				out = append(out, id)
			}
		case r.Type == "control" && r.Body["status"] == string(contract.Ready) && r.Body["reason"] == ownerRetryReason:
			out = nil
		}
	}
	return out
}

// retryEmptyFinish gives an empty finish the retry a crash gets: Ready again with the exit as
// its cause, and the next pass assigns the next attempt. After emptyFinishLimit of them the Job
// is parked with the question a crash loop asks, worded for the person by qtPkWords.
func (s Store) retryEmptyFinish(snap Snapshot, j *contract.Job, m *jobMemory, all *memory) error {
	exits := emptyFinishes(snap, j)
	if len(exits) < emptyFinishLimit {
		return s.control(j, contract.Ready, "retry after failed attempt", j.Exited)
	}
	// The question waits on m.Failure, as the owner's Retry is matched against it.
	m.Failures, m.Failure = exits, exits[len(exits)-1]
	if err := s.saveMemory(*all); err != nil {
		return err
	}
	if err := s.park(snap, j, m); err != nil {
		return err
	}
	m.ParkProgress = s.progress(snap, j)
	return nil
}
