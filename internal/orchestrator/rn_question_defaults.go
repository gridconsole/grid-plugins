package orchestrator

import (
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

// rnQdPortForTest is nil outside tests; a test sets it to wrap the Job port the
// pass hands the question store, so it can look at the locks around a call.
var rnQdPortForTest func(question.JobPort) question.JobPort

// CloseQuestionJob gives a question's person Job its result: the JobPort.Close
// of the question store, shared by `grid question` and the worker's pass. A Job
// that is not a question Job, is closed already, or has no writer that may close
// it is left alone. It takes .orchestrator.lock inside AnswerPerson, so a caller
// must not hold questions/.lock (question.Store.postJob says why).
func CloseQuestionJob(f feed.Store, lease, person feed.Writer, id, result string) error {
	snap, err := Load(f)
	if err != nil {
		return err
	}
	j := snap.State.Jobs[id]
	ask := false
	for _, it := range snap.Items {
		ask = ask || it.ID == id && it.IsPerson() && it.Kind == question.JobKind
	}
	if j == nil || !ask || j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
		return nil
	}
	w := person
	if lease.Lease == "task" {
		w = lease
	}
	if w.Lease != "task" && !PersonWriter(w) {
		return nil
	}
	result = strings.TrimSpace(result)
	if CheckPersonAnswer(result) != nil {
		result = string([]rune(result)[:200]) + "…"
	}
	// The store's clock reaches the Feed here, as Load reads it; unset is the wall clock.
	now := time.Now()
	if f.Now != nil {
		now = f.Now()
	}
	_, err = AnswerPerson(f, w, id, result, now)
	return err
}

// rnQuestionDefaults applies the due defaults and aging of the task's questions
// in the worker's pass, so a question that nobody answered is decided at its
// deadline and the Jobs behind its person Job carry on. w is the task lease
// writer of the pass; it posts the default and aging controls and closes the
// person Job with the default as its answer.
//
// Lock order (C-jw): question.Store.Tick holds questions/.lock for the Feed
// read and the question files only; it lets the lock go before it closes the
// person Jobs, and CloseQuestionJob then reaches .orchestrator.lock. The caller
// runs this before Store.Tick and outside its locked, so the lock is never held
// while questions/.lock is taken, which ReserveGate does in the other order.
//
// An error is one diag line, again only when its words change: the question
// files are the owner's, and a broken one must not stop the Jobs of the task.
// A lost lease stops Store.Tick next, which skips the task with its own reason.
func (s *Stream) rnQuestionDefaults(dir, taskID string, snap Snapshot, w feed.Writer) {
	key := dir + "\x00question-defaults"
	// No question on the Feed: nothing to decide, and no questions/ folder to make.
	if len(snap.State.Questions) == 0 {
		delete(s.skipped, key)
		return
	}
	port := question.JobPort{Close: CloseQuestionJob}
	if rnQdPortForTest != nil {
		port = rnQdPortForTest(port)
	}
	qs := question.Store{TaskDir: dir, TaskID: taskID, Writer: w, ControlWriter: w, Jobs: port, Now: s.Now}
	if _, err := qs.Tick(); err != nil {
		if msg := oneLine(err); s.skipped[key] != msg {
			s.skipped[key] = msg
			s.diag("grid worker next: task %s: question defaults: %s\n", taskID, msg)
		}
		return
	}
	delete(s.skipped, key)
}
