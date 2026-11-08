package feed

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
	"reflect"
	"testing"
	"time"
)

func TestPersonGateAnswersAndConflicts(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "visitor", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": "2099-01-01T00:00:00.000Z"}}}, w)
	a := mustPost(t, s, Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Yes"}}, w)
	state, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Questions[q.ID].Open || value(state.Questions[q.ID].Applied) != a.ID {
		t.Fatalf("single answer: %+v", state.Questions[q.ID])
	}
	other := w
	other.Author.ID = "other"
	other.Worker = "other@" + testMachine
	mustPost(t, s, Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "No"}}, other)
	state, err = s.State()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Questions[q.ID].Open || state.Questions[q.ID].Applied != nil {
		t.Fatalf("conflict was silently resolved: %+v", state.Questions[q.ID])
	}
	task := taskWriter(t, s)
	mustPost(t, s, Input{Type: "control", Body: map[string]any{"target": q.ID, "answer": a.ID, "status": "answered"}}, task)
	state, err = s.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Questions[q.ID].Open || value(state.Questions[q.ID].Applied) != a.ID {
		t.Fatal("explicit control did not resolve conflict")
	}
}

func TestQuestionControlRequiresTaskTenure(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": "2099-01-01T00:00:00.000Z"}}}, w)
	in := Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "withdrawn", "text": "No longer needed"}}
	if _, err := s.Post(in, w); err == nil {
		t.Fatal("unleased control accepted")
	}
	task := taskWriter(t, s)
	control := mustPost(t, s, in, task)
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	state, decisions := FoldRead(read)
	if !reflect.DeepEqual(state.Verdicts, contract.Fold(posts(read)).Verdicts) {
		t.Fatal("person layer changed contract verdicts")
	}
	if state.Questions[q.ID].Open || decisions[q.ID].State != "withdrawn" {
		t.Fatalf("withdraw: %+v", decisions)
	}
	for i := range read.Records {
		if read.Records[i].ID == control.ID {
			read.Records[i].Verdict = "stale"
		}
	}
	state, decisions = FoldRead(read)
	if !state.Questions[q.ID].Open || len(decisions[q.ID].Events) != 0 {
		t.Fatal("stale control closed gate")
	}
}

func TestPersonGateDeadlineWithoutControl(t *testing.T) {
	s := testStore(t)
	deadline := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": deadline.Format("2006-01-02T15:04:05.000Z")}}}, testWriter())
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []time.Duration{-time.Nanosecond, 0, time.Hour} {
		state, decisions := FoldReadAt(read, deadline.Add(offset))
		d := decisions[q.ID]
		if state.Questions[q.ID].Open != (offset < 0) || len(d.Events) != 0 {
			t.Fatalf("deadline %s: %+v %+v", offset, state.Questions[q.ID], d)
		}
		if offset >= 0 && (d.State != "defaulted" || d.Answer != "Wait" || len(state.NeedsYou) != 0) {
			t.Fatalf("default not applied: %+v", d)
		}
		if !reflect.DeepEqual(state.Verdicts, contract.Fold(posts(read)).Verdicts) {
			t.Fatal("deadline changed contract verdicts")
		}
	}
	s.Now = func() time.Time { return deadline }
	state, err := s.State()
	if err != nil || state.Questions[q.ID].Open {
		t.Fatalf("State ignored read clock: %+v %v", state, err)
	}
	after, err := s.Read(Filter{})
	if err != nil || len(after.Records) != len(read.Records) {
		t.Fatalf("default wrote a record: %+v %v", after, err)
	}
}

func TestPersonGateRejectsNonPersonSelection(t *testing.T) {
	for _, role := range []string{"agent", "cli"} {
		t.Run(role, func(t *testing.T) {
			s := testStore(t)
			q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "visitor", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": "2099-01-01T00:00:00.000Z"}}}, testWriter())
			w := testWriter()
			w.Author.Role = role
			a := mustPost(t, s, Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Yes"}}, w)
			mustPost(t, s, Input{Type: "control", Body: map[string]any{"target": q.ID, "answer": a.ID, "status": "answered"}}, taskWriter(t, s))
			read, err := s.Read(Filter{})
			if err != nil {
				t.Fatal(err)
			}
			state, _ := FoldRead(read)
			if !state.Questions[q.ID].Open || state.Questions[q.ID].Applied != nil || len(state.NeedsYou) != 1 {
				t.Fatalf("ineligible answer selected: %+v", state)
			}
			if !reflect.DeepEqual(state.Verdicts, contract.Fold(posts(read)).Verdicts) {
				t.Fatal("selection changed contract verdicts")
			}
		})
	}
}
