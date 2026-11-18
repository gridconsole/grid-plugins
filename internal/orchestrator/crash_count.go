package orchestrator

import (
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// crashLimit is how many crashes of one Job in a row the tick retries before it parks the Job
// and asks the person.
const crashLimit = 3

// crashExits is the exit of every attempt of the Job that crashed in the streak that is still
// running, oldest first. It counts from the Feed, as emptyFinishes does, and not from the memory
// counter: the task folder changes on every attempt (a hook file, a usage row) and a changed
// progress hash used to clear that counter each pass, so a Job that crashed a hundred times
// never reached the limit. The streak ends at an attempt that ran to its end (done), at a stop
// (cancelled) and at the owner's Retry. A usage_limit exit is no crash and does not end it.
func crashExits(snap Snapshot, j *contract.Job) []string {
	var out []string
	seen := map[int]bool{}
	for _, id := range j.History {
		r := snap.Records[id]
		switch {
		case r.Type == "worker-exit" || r.Type == "exit":
			switch stringValue(r.Body["reason"]) {
			case "usage_limit":
			case "done", "cancelled":
				out = nil
			default:
				if r.Attempt != nil {
					if seen[*r.Attempt] {
						continue
					}
					seen[*r.Attempt] = true
				}
				out = append(out, id)
			}
		case r.Type == "control" && r.Body["status"] == string(contract.Ready) && r.Body["reason"] == ownerRetryReason:
			out = nil
		}
	}
	return out
}

// CrashBackoff is the wait before the next attempt after n failed attempts in a row: ten seconds,
// doubling each time, at most five minutes.
func CrashBackoff(n int) time.Duration {
	const first, longest = 10 * time.Second, 5 * time.Minute
	d := first
	for i := 1; i < n && d < longest; i++ {
		d *= 2
	}
	return min(d, longest)
}

// backOff holds the next attempt of a Job that has just failed for the wait RetryDelay gives; with
// no RetryDelay the next pass assigns it at once.
func (s Store) backOff(m *jobMemory, failures int) {
	if s.RetryDelay != nil {
		m.RetryAt = s.now().Add(s.RetryDelay(failures))
	}
}

// retryWhy is the reason a Ready Job shows while it waits out its back-off.
func retryWhy(at time.Time) string {
	return "Waiting to retry after a failed attempt, next try " + at.Local().Format("15:04:05")
}

// retryCrash retries a crashed attempt as a new one, after a back-off, and parks the Job with the
// question a crash loop asks once crashLimit crashes in a row are in the Feed.
func (s Store) retryCrash(snap Snapshot, j *contract.Job, m *jobMemory, all *memory) error {
	exits := crashExits(snap, j)
	if len(exits) < crashLimit {
		s.backOff(m, len(exits))
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
