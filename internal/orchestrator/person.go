package orchestrator

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

const maxPersonAnswer = 3500

// maxUnusedQuote is how much of an unused answer its notice quotes.
const maxUnusedQuote = 3000

// WaitingError is the refusal of an answer to a person Job that still waits for
// another Job. Job is the person Job, Target the Job it waits for and Reason the
// wait reason of Ready. PersonReady wraps it in a feed.InvalidRecordError.
type WaitingError struct {
	Job    string
	Target string
	Reason string
}

func (e *WaitingError) Error() string {
	return fmt.Sprintf("%s cannot be answered yet: %s", e.Job, e.Reason)
}

// PersonReady refuses to answer a person Job while a Job it waits for is not
// accepted: job next holds any other Job the same way, and an early answer would
// release the Jobs behind it before the work they depend on exists.
func PersonReady(snap Snapshot, id string, now time.Time) error {
	ready, err := Ready(snap, now)
	if err != nil {
		return err
	}
	for _, w := range ready.Waiting {
		if w.Job == id && w.Target != "" {
			return &feed.InvalidRecordError{Err: &WaitingError{Job: id, Target: w.Target, Reason: w.Reason}}
		}
	}
	return nil
}

// personAnswers is the holder's half of an unleased answer to a person Job: a
// person of the team answers with an `answer` that follows the Job's record and
// has no lease, and this pass writes the message and the accepting control under
// the task lease, as AnswerPerson does for a leased writer. The earliest such
// answer of an open person Job wins. Every other answer that reached the Job
// before this pass is named in a message on the Feed that says it was not used,
// so the person who wrote it can read that in feed read. A Job still waiting for
// another Job keeps its answers for a later pass.
func (s Store) personAnswers(snap Snapshot) error {
	for _, item := range snap.Items {
		j := snap.State.Jobs[item.ID]
		if !item.IsPerson() || j == nil || len(j.History) == 0 || j.Status == contract.Accepted || j.Status == contract.Cancelled || j.Status == contract.Rejected {
			continue
		}
		var answers []feed.Record
		for _, r := range snap.Records {
			if r.Type != "answer" || r.Re == nil || *r.Re != j.History[0] || r.Lease != nil || r.Fence != 0 || contract.TierFor(r.Author.Role) != contract.PersonTeam && !contract.GuestAddressee(r.Author.Role, r.Author.ID, false, item.Role, item.PersonName(), item.Kind) {
				continue
			}
			if text, _ := r.Body["answer"].(string); strings.TrimSpace(text) == "" || utf8.RuneCountInString(text) > maxPersonAnswer {
				continue
			}
			answers = append(answers, r)
		}
		if len(answers) == 0 || PersonReady(snap, item.ID, s.now()) != nil {
			continue
		}
		sort.Slice(answers, func(a, b int) bool {
			if answers[a].At != answers[b].At {
				return answers[a].At < answers[b].At
			}
			return answers[a].ID < answers[b].ID
		})
		first := answers[0]
		answer, _ := first.Body["answer"].(string)
		back := sendsBack(first.Body, first.Author.Role, item.Kind)
		text := fmt.Sprintf("%s answered %s at %s: %s", first.Author.ID, item.ID, first.At, strings.TrimSpace(answer))
		if back {
			text = fmt.Sprintf("%s sent %s back at %s: %s", first.Author.ID, item.ID, first.At, strings.TrimSpace(answer))
		}
		said, err := s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, s.Writer)
		if err != nil {
			return err
		}
		// The notices go before the accepting control, so a pass that stops between
		// the two writes repeats them rather than dropping them.
		for _, late := range answers[1:] {
			unused, _ := late.Body["answer"].(string)
			unused = strings.TrimSpace(unused)
			// A message holds 4000 characters; the answer record keeps the full text.
			if r := []rune(unused); len(r) > maxUnusedQuote {
				unused = string(r[:maxUnusedQuote]) + "…"
			}
			note := fmt.Sprintf("The answer of %s to %s at %s (record %s) was not used: %s answered first, at %s. It said: %s", late.Author.ID, item.ID, late.At, late.ID, first.Author.ID, first.At, unused)
			if _, err := s.Feed.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": note}}, s.Writer); err != nil {
				return err
			}
		}
		body := map[string]any{"target": item.ID, "status": "accepted", "reason": "answered by " + first.Author.ID, "answer": strings.TrimSpace(answer)}
		// What the person's answer carried, such as the plan revision they approved,
		// goes into the result the readers use.
		copyAnswerFields(body, first.Body)
		if back {
			body = sentBackControl(item.ID, first.Author.ID, strings.TrimSpace(answer), first.Body["sent_back_hash"])
		}
		if _, err := s.Feed.Post(feed.Input{Type: "control", Re: said.ID, Body: body}, s.Writer); err != nil {
			return err
		}
	}
	return nil
}

// CheckPersonAnswer refuses an answer, already trimmed, longer than the cap. The
// answer of grid job result and the answer of grid approve --plan-hash share it.
func CheckPersonAnswer(answer string) error {
	if utf8.RuneCountInString(answer) > maxPersonAnswer {
		return &feed.InvalidRecordError{Err: fmt.Errorf("the answer is longer than %d characters", maxPersonAnswer)}
	}
	return nil
}

// PersonOpen is what every answer to a person Job checks first: the Job is a
// person's, it is not already closed, and the Jobs it waits for are accepted.
func PersonOpen(snap Snapshot, id string, now time.Time) error {
	bad := func(format string, a ...any) error { return &feed.InvalidRecordError{Err: fmt.Errorf(format, a...)} }
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
	return PersonReady(snap, id, now)
}

// AnswerPerson records who answered a role Person Job, when, and the answer,
// then marks the Job accepted so the Jobs behind it are released. It is a plain
// record: it does not check who the caller is. The task lease writes it, since a
// person's Job is never assigned to a worker. A person of the team with no lease
// writes it unleased (fence zero, like an answer to a question), so a held task
// lease does not stop the answer; the next pass of the holder finds the Job
// accepted and releases the Jobs behind it.
func AnswerPerson(store feed.Store, w feed.Writer, id, answer string, now time.Time) (record feed.Record, err error) {
	return AnswerPersonWith(store, w, id, answer, nil, now)
}

// answerFields are the keys of an answer, besides the text, that reach the
// accepting control of a person Job: the plan revision an approval was made on,
// the lands hash, the person's note, and the reason when it is not "answered by".
// send_back and sent_back_hash send the plan of an approve Job back instead; they
// act only through sentBackControl and copyAnswerFields never copies them.
var answerFields = []string{"plan_hash", "plan_digest", "lands_hash", "note", "reason", "send_back", "sent_back_hash"}

// sendBackKey names the answerFields of a plan sent back.
var sendBackKey = map[string]bool{"send_back": true, "sent_back_hash": true}

// copyAnswerFields copies the answerFields that src holds into dst, except the
// send-back keys: on a Job they do not send back they are dropped.
func copyAnswerFields(dst, src map[string]any) {
	for _, key := range answerFields {
		if v, ok := src[key]; ok && !sendBackKey[key] {
			dst[key] = v
		}
	}
}

// sendsBack reports whether an answer sends the plan back (D-ig): it says
// send_back true, a person of the team wrote it and the Job is an approve Job.
// Who may answer the approve Job at all comes from the contract tier, as for an
// approval.
func sendsBack(fields map[string]any, role, kind string) bool {
	back, _ := fields["send_back"].(bool)
	return back && kind == "approve" && contract.TierFor(role) == contract.PersonTeam
}

// sentBackControl closes the approve Job id rejected with the person's words: the
// plan is not approved, and the re-plan keeper (guReplanRejected) posts the
// re-plan Job for the Jobs that wait on it. It carries no plan_hash, so no reader
// takes it for an approval.
func sentBackControl(id, who, answer string, hash any) map[string]any {
	body := map[string]any{"target": id, "status": "rejected", "reason": "sent back by " + who, "close_reason": "sent back by " + who, "answer": answer, "send_back": true}
	if h, ok := hash.(string); ok && h != "" {
		body["sent_back_hash"] = h
	}
	return body
}

// AnswerPersonWith is AnswerPerson with fields that become part of the result:
// the plan_hash, plan_digest, lands_hash and note of an approval. A leased writer
// puts them in the accepting control; an unleased person puts them in the answer,
// and the holder's next pass copies them into the control (personAnswers).
func AnswerPersonWith(store feed.Store, w feed.Writer, id, answer string, fields map[string]any, now time.Time) (record feed.Record, err error) {
	// A guest with no lease is taken only for the Job that names it (checked below
	// against the Feed); every other writer needs the task lease or the team role.
	guest := guestAnswerer(w)
	unleased := PersonWriter(w) || guest
	if w.Lease != "task" && !unleased {
		return record, feed.ErrStaleLease
	}
	lock := (Store{Feed: store, Writer: w}).locked
	if unleased {
		lock = (Store{Feed: store, Writer: w}).exclusive
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return record, &feed.InvalidRecordError{Err: fmt.Errorf("a person Job needs an answer")}
	}
	if err := CheckPersonAnswer(answer); err != nil {
		return record, err
	}
	// Only a person of the team sends a plan back (D-ig, owner question 3): a guest
	// or a writer of another tier that asks for it gets nothing written.
	if wants, _ := fields["send_back"].(bool); wants && (guest || contract.TierFor(w.Author.Role) != contract.PersonTeam) {
		return record, &feed.InvalidRecordError{Err: fmt.Errorf("only a person of the team sends a plan back")}
	}
	err = lock(func() error {
		snap, e := Load(store)
		if e != nil {
			return e
		}
		if guest && !guestAddressed(snap, w, id) {
			return feed.ErrStaleLease
		}
		if e := PersonOpen(snap, id, now); e != nil {
			return e
		}
		j := snap.State.Jobs[id]
		kind := ""
		for _, item := range snap.Items {
			if item.ID == id {
				kind = item.Kind
			}
		}
		back := sendsBack(fields, w.Author.Role, kind)
		if unleased {
			// A person writes no closing control without the lease; the answer waits
			// for the holder's next pass, which accepts the Job (personAnswers) or,
			// for a plan sent back, rejects it.
			body := map[string]any{"answer": answer}
			copyAnswerFields(body, fields)
			if back {
				body["send_back"] = true
				if h, ok := fields["sent_back_hash"].(string); ok && h != "" {
					body["sent_back_hash"] = h
				}
			}
			record, e = store.Post(feed.Input{Type: "answer", Re: j.History[0], Body: body}, w)
			return e
		}
		text := fmt.Sprintf("%s answered %s at %s: %s", w.Author.ID, id, contract.FormatTime(now), answer)
		if back {
			text = fmt.Sprintf("%s sent %s back at %s: %s", w.Author.ID, id, contract.FormatTime(now), answer)
		}
		said, e := store.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}, w)
		if e != nil {
			return e
		}
		body := map[string]any{"target": id, "status": "accepted", "reason": "answered by " + w.Author.ID, "answer": answer}
		copyAnswerFields(body, fields)
		if back {
			body = sentBackControl(id, w.Author.ID, answer, fields["sent_back_hash"])
		}
		record, e = store.Post(feed.Input{Type: "control", Re: said.ID, Body: body}, w)
		return e
	})
	return
}
