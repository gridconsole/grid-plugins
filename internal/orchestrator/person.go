package orchestrator

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const maxPersonAnswer = 3500

// AnswerPerson records who answered a role Person Job, when, and the answer,
// then marks the Job accepted so the Jobs behind it are released. It is a plain
// record: it does not check who the caller is. The task lease writes it, since a
// person's Job is never assigned to a worker.
func AnswerPerson(store feed.Store, w feed.Writer, id, answer string, now time.Time) (record feed.Record, err error) {
	if w.Lease != "task" {
		return record, feed.ErrStaleLease
	}
	bad := func(format string, a ...any) error { return &feed.InvalidRecordError{Err: fmt.Errorf(format, a...)} }
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return record, bad("a person Job needs an answer")
	}
	if utf8.RuneCountInString(answer) > maxPersonAnswer {
		return record, bad("the answer is longer than %d characters", maxPersonAnswer)
	}
	err = (Store{Feed: store, Writer: w}).locked(func() error {
		snap, e := Load(store)
		if e != nil {
			return e
		}
		j := snap.State.Jobs[id]
		person := false
		for _, item := range snap.Items {
			if item.ID == id {
				person = item.IsPerson()
			}
		}
		if j == nil || !person {
			return bad("%s is not a Job for a person", id)
		}
		if j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
			return bad("%s is already %s", id, j.Status)
		}
		text := fmt.Sprintf("%s answered %s at %s: %s", w.Author.ID, id, contract.FormatTime(now), answer)
		said, e := store.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, w)
		if e != nil {
			return e
		}
		record, e = store.Post(feed.Input{Type: "control", Re: said.ID, Body: map[string]any{"target": id, "status": "accepted", "reason": "answered by " + w.Author.ID}}, w)
		return e
	})
	return
}
