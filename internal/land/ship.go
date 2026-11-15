package land

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
// closes the task. A project with no deploy topic (ErrNoDeployTopic) has nothing
// to land: a "ship answered" line says so and the task closes on the answer. A
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
		// Nothing to land: the answer is the approval, and the task closes on it.
		if err = s.note(cwd, id, fmt.Sprintf("%s: %s approved; no deploy topic for project %s, nothing to land", shipEvent, job, t.Project)); err != nil {
			return result, true, err
		}
		_, err = s.tasks().Change(task.WriteOptions{Cwd: cwd, ID: id, Action: "close", Note: "approved after " + job + "; no deploy topic", Who: who})
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

// markShip writes the "ship answered" line for job under .land.lock unless
// history.md already holds it with no later reopen line for job (Reopened); first
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
	detail := job + " accepted; grid land runs"
	armed := true
	for _, line := range strings.Split(string(history), "\n") {
		switch {
		case strings.Contains(line, " · "+shipEvent+" · "+detail+" · "):
			armed = false
		case strings.Contains(line, " · "+shipEvent+" · "+shipReopenedDetail(job)+" · "):
			armed = true
		}
	}
	if !armed {
		return false, nil
	}
	return true, s.note(cwd, id, shipEvent+": "+detail)
}
