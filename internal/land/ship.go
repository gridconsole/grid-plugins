package land

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// shipEvent is the history event AfterShip writes once per answered ship person
// Job, before land runs; its detail names the Job, so a later pass finds it.
const shipEvent = "ship answered"

// AfterShip is the step after the person answered the ship person Job job of the
// task id: it runs land once for that Job and records the result. The approval
// land asks for is the task's one approval, the result of its plan Job (Run,
// ApprovalForGate). A "ship answered" history line naming job is written first,
// under .land.lock, so a second call for the same Job does nothing (ran is false).
// A land that stops before its command runs gets a "land failed" history line
// with the reason; a command that fails has Run's own line. A land that passed
// closes the task. A project with no deploy topic (ErrNoDeployTopic) has no
// command to run: Merge, when set, merges the task's delivered branch into its
// base first (a merge that fails is a "land failed" line and a *FailedError, the
// task stays waiting), then a "ship answered" line says so and the task closes on
// the answer. A
// deploy topic that is unreadable or malformed is still "land failed". A deploy
// command that ran and failed returns a *FailedError with the text for the person; the
// caller puts the ship Job back with its person and then calls Reopened, so the
// next approve runs land again. Nothing here posts a Job.
func (s Store) AfterShip(ctx context.Context, cwd, id, job string) (result Result, ran bool, err error) {
	t, err := s.tasks().Show(cwd, id)
	if err != nil {
		return result, false, err
	}
	first, err := s.markShip(t.Path, cwd, id, job)
	if err != nil || !first {
		return result, false, err
	}
	who := s.Who
	if who.ID == "" {
		who.ID, who.Role = "local", "owner"
	}
	result, err = s.Run(ctx, cwd, id, false)
	if errors.Is(err, ErrNoDeployTopic) && result.Execution == nil {
		// No deploy command to run: the answer is the approval. The delivered branch
		// still goes into its base, so a task is never closed with its work unlanded.
		merged := ""
		if s.Merge != nil {
			var mergeErr error
			if merged, mergeErr = s.Merge(id); mergeErr != nil {
				text := strings.Join(strings.Fields(mergeErr.Error()), " ")
				if noteErr := s.note(cwd, id, "land failed: "+text); noteErr != nil {
					return result, true, noteErr
				}
				return result, true, &FailedError{Text: text, Err: mergeErr}
			}
		}
		detail, closeNote := "nothing to land", "approved after "+job+"; no deploy topic"
		if merged != "" {
			if len(merged) > 12 {
				merged = merged[:12]
			}
			detail, closeNote = "merged the task's branch as "+merged, closeNote+"; merged as "+merged
		}
		if err = s.note(cwd, id, fmt.Sprintf("%s: %s approved; no deploy topic for project %s, %s", shipEvent, job, t.Project, detail)); err != nil {
			return result, true, err
		}
		_, err = s.tasks().Change(task.WriteOptions{Cwd: cwd, ID: id, Action: "close", Note: closeNote, Who: who})
		return result, true, err
	}
	if err != nil {
		if result.Execution == nil {
			if noteErr := s.note(cwd, id, "land failed: "+strings.Join(strings.Fields(err.Error()), " ")); noteErr != nil {
				return result, true, noteErr
			}
			return result, true, err
		}
		return result, true, &FailedError{Text: failureText(result), Err: err}
	}
	_, err = s.tasks().Change(task.WriteOptions{Cwd: cwd, ID: id, Action: "close", Note: "landed after " + job, Who: who})
	return result, true, err
}

// FailedError is the error AfterShip returns when the deploy command ran and did
// not land: it exited non-zero or timed out. A land that stops before its command
// runs returns its plain error, as before. Text is what the person reads on the
// reopened ship Job: the exit code and the first lines of stderr. Err is the
// land's own error, and Error is its text.
type FailedError struct {
	Text string
	Err  error
}

func (e *FailedError) Error() string { return e.Err.Error() }
func (e *FailedError) Unwrap() error { return e.Err }

// failureLines is how many lines of the deploy command's stderr the note keeps.
const failureLines = 5

// failureText is the note of a failed land: "grid land exited N" (or timed out)
// and the first failureLines lines of the command's stderr. result.Execution is
// not nil.
func failureText(result Result) string {
	x := result.Execution
	text := fmt.Sprintf("grid land exited %d", x.ExitCode)
	if result.Status == "timed-out" {
		text = fmt.Sprintf("grid land timed out (exit %d)", x.ExitCode)
	}
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(x.Stderr, "\r\n", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" && len(lines) < failureLines {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 && x.Error != "" {
		lines = append(lines, x.Error)
	}
	if len(lines) > 0 {
		text += ": " + strings.Join(lines, " / ")
	}
	return text
}

// shipReopenedDetail is the detail of the "ship answered" line Reopened writes
// once the ship Job job is back with its person after a failed land.
func shipReopenedDetail(job string) string {
	return job + " reopened after a failed land; the next approve runs grid land again"
}

// Reopened records that the ship Job job went back to its person after a failed
// land, so the next acceptance of job runs land once more (markShip). The caller
// writes it only after the Job is open again on the Feed: a Job still accepted
// with this line would land again on every pass.
func (s Store) Reopened(cwd, id, job string) error {
	t, err := s.tasks().Show(cwd, id)
	if err != nil {
		return err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(t.Path, ".land.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return err
	}
	return s.note(cwd, id, shipEvent+": "+shipReopenedDetail(job))
}

// shipRebasePrefix starts the detail of the "ship answered" line RebaseQueued
// writes: the answer to job stands (a conflict-only rebase that keeps the tests green
// needs no new approval) and land runs again once the rebase Job is accepted.
func shipRebasePrefix(job string) string { return job + " approval kept; waits for rebase " }

// RebaseQueued records that a land of the ship Job job stopped on a merge conflict
// and the rebase Job rebase was posted: the person's approval is kept, and the next
// acceptance pass of job runs land again (markShip) once the rebase is accepted.
// tip is the commit the target branch pointed at when the conflict was seen ("" when
// unknown), kept as its first tipLen characters since a history detail is short;
// RebaseTip reads it back, so a later conflict can tell whether the target moved
// since. The caller writes it after the rebase Job is on the Feed.
func (s Store) RebaseQueued(cwd, id, job, rebase, tip string) error {
	return s.withShipLock(cwd, id, func(string) error {
		at := ""
		if tip != "" {
			at = " (" + rebaseTipMark + tip[:min(len(tip), tipLen)] + ")"
		}
		return s.note(cwd, id, shipEvent+": "+shipRebasePrefix(job)+rebase+at+", which keeps both changes; grid land runs again once it is accepted")
	})
}

// rebaseTipMark starts the target tip RebaseQueued puts in its line, and tipLen is how
// many characters of the commit it keeps.
const (
	rebaseTipMark = "target tip "
	tipLen        = 12
)

var rebaseLine = regexp.MustCompile(`waits for rebase (\S+?)(?: \(` + rebaseTipMark + `([0-9a-f]+)\))?, which keeps both changes`)

// RebaseTip is the rebase Job and the target tip of the newest RebaseQueued line of
// the ship Job job. Both are "" when job has none, and tip is "" for a line written
// before the tip was kept; a tip is at most tipLen characters, so compare it as a prefix.
func (s Store) RebaseTip(cwd, id, job string) (rebase, tip string, err error) {
	err = s.withShipLock(cwd, id, func(history string) error {
		for _, line := range strings.Split(history, "\n") {
			if !strings.Contains(line, " · "+shipEvent+" · "+shipRebasePrefix(job)) {
				continue
			}
			if m := rebaseLine.FindStringSubmatch(line); m != nil {
				rebase, tip = m[1], m[2]
			}
		}
		return nil
	})
	return rebase, tip, err
}

// shipRetryDetail is the detail of the "ship answered" line Retry writes.
func shipRetryDetail(job string) string {
	return job + " land failed; the approval stands and grid land runs again on the next pass"
}

// Retry records that a land of the ship Job job failed and the person's approval
// stands: the next pass runs land again (markShip) with no new answer, and Armed
// reports true until it has. The caller writes it instead of putting the ship Job
// back with its person, and only for a failure that is not a deploy command's own.
func (s Store) Retry(cwd, id, job string) error {
	return s.withShipLock(cwd, id, func(string) error {
		return s.note(cwd, id, shipEvent+": "+shipRetryDetail(job))
	})
}

// Armed reports whether the answer to the ship Job job still has a land to run: it has
// not run yet, or it was reopened, queued a rebase or is to retry (shipState).
func (s Store) Armed(cwd, id, job string) (bool, error) {
	armed := false
	err := s.withShipLock(cwd, id, func(history string) error {
		armed, _ = shipState(history, job)
		return nil
	})
	return armed, err
}

// RebasePending reports whether the ship Job job waits for a rebase Job: the newest
// arming line of its history is the one RebaseQueued writes, and land has not run
// since.
func (s Store) RebasePending(cwd, id, job string) (bool, error) {
	pending := false
	err := s.withShipLock(cwd, id, func(history string) error {
		_, pending = shipState(history, job)
		return nil
	})
	return pending, err
}

func (s Store) withShipLock(cwd, id string, fn func(history string) error) error {
	t, err := s.tasks().Show(cwd, id)
	if err != nil {
		return err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(t.Path, ".land.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return err
	}
	history, err := os.ReadFile(filepath.Join(t.Path, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return fn(string(history))
}

// shipState reads the "ship answered" lines of job in history.md in order. armed is
// true until land has run for the answer, and again after a reopen (Reopened), a
// retry (Retry) or a queued rebase (RebaseQueued); rebasing is true when the newest of
// those lines is the rebase one.
func shipState(history, job string) (armed, rebasing bool) {
	armed = true
	for _, line := range strings.Split(history, "\n") {
		switch {
		case strings.Contains(line, " · "+shipEvent+" · "+job+" accepted; grid land runs · "):
			armed, rebasing = false, false
		case strings.Contains(line, " · "+shipEvent+" · "+shipReopenedDetail(job)+" · "),
			strings.Contains(line, " · "+shipEvent+" · "+shipRetryDetail(job)+" · "):
			armed, rebasing = true, false
		case strings.Contains(line, " · "+shipEvent+" · "+shipRebasePrefix(job)):
			armed, rebasing = true, true
		}
	}
	return armed, rebasing
}

// markShip writes the "ship answered" line for job under .land.lock unless
// history.md already holds it with no later reopen, retry or rebase line for job; first
// reports whether this call wrote it.
func (s Store) markShip(dir, cwd, id, job string) (first bool, err error) {
	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".land.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return false, err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return false, err
	}
	history, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if armed, _ := shipState(string(history), job); !armed {
		return false, nil
	}
	return true, s.note(cwd, id, shipEvent+": "+job+" accepted; grid land runs")
}
