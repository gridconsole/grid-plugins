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
// deploy topic that is unreadable or malformed is still "land failed". Nothing
// here posts a Job.
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
		}
		return result, true, err
	}
	_, err = s.tasks().Change(task.WriteOptions{Cwd: cwd, ID: id, Action: "close", Note: "landed after " + job, Who: who})
	return result, true, err
}

// markShip writes the "ship answered" line for job under .land.lock unless
// history.md already holds it; first reports whether this call wrote it.
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
	for _, line := range strings.Split(string(history), "\n") {
		if strings.Contains(line, " · "+shipEvent+" · "+detail+" · ") {
			return false, nil
		}
	}
	return true, s.note(cwd, id, shipEvent+": "+detail)
}
