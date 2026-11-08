package feed

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
	"os"
	"path/filepath"
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
	unleased := in
	unleased.Body = map[string]any{"target": q.ID, "question_event": "aged", "text": "Priority raised", "level": 1}
	if _, err := s.Post(unleased, w); err == nil {
		t.Fatal("unleased control accepted")
	}
	task := taskWriter(t, s)
	control := mustPost(t, s, in, task)
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	state, decisions := FoldReadQuestions(read)
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
	state, decisions = FoldReadQuestions(read)
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
			state, _ := FoldReadQuestions(read)
			if !state.Questions[q.ID].Open || state.Questions[q.ID].Applied != nil || len(state.NeedsYou) != 1 {
				t.Fatalf("ineligible answer selected: %+v", state)
			}
			if !reflect.DeepEqual(state.Verdicts, contract.Fold(posts(read)).Verdicts) {
				t.Fatal("selection changed contract verdicts")
			}
		})
	}
}

func TestPersonWithdrawFold(t *testing.T) {
	for _, tc := range []struct {
		name, role, prior, verdict, want string
		allowed                          func(string) bool
		badRe, badTarget, afterAnswer    bool
	}{
		{name: "owner", role: "owner", want: "withdrawn"},
		{name: "member", role: "member", want: "withdrawn"},
		{name: "forged guest", role: "guest", want: "open"},
		{name: "lead default refused", role: "lead", want: "open"},
		{name: "host permits guest", role: "guest", allowed: func(r string) bool { return r == "guest" }, want: "withdrawn"},
		{name: "host refuses member", role: "member", allowed: func(r string) bool { return r == "owner" }, want: "open"},
		{name: "agent even if permitted", role: "agent", allowed: func(string) bool { return true }, want: "open"},
		{name: "cli even if permitted", role: "cli", allowed: func(string) bool { return true }, want: "open"},
		{name: "stale", role: "owner", verdict: "stale", want: "open"},
		{name: "held", role: "owner", verdict: "held", want: "open"},
		{name: "wrong re", role: "owner", badRe: true, want: "open"},
		{name: "wrong target", role: "owner", badTarget: true, want: "open"},
		{name: "after answer", role: "owner", prior: "answer", want: "answered"},
		{name: "before answer", role: "owner", afterAnswer: true, want: "withdrawn"},
		{name: "after default control", role: "owner", prior: "default", want: "defaulted"},
		{name: "after deadline", role: "owner", prior: "deadline", want: "defaulted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testStore(t)
			now := time.Date(2098, 1, 1, 0, 0, 0, 0, time.UTC)
			s.Now = func() time.Time { now = now.Add(time.Second); return now }
			w := testWriter()
			deadline := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
			q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"kind": "person", "id": "Q-1", "default": "Wait", "at": deadline.Format("2006-01-02T15:04:05.000Z")}}}, w)
			answer := func() { mustPost(t, s, Input{Type: "answer", Re: q.ID, Body: map[string]any{"answer": "Yes"}}, w) }
			if tc.prior == "answer" {
				answer()
			}
			if tc.prior == "default" || tc.prior == "deadline" {
				now = deadline
			}
			if tc.prior == "default" {
				mustPost(t, s, Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "defaulted", "text": "Wait"}}, taskWriter(t, s))
			}
			person := w
			person.Author.Role = tc.role
			if tc.role == "agent" || tc.role == "cli" {
				person.Author.Role = "owner"
			}
			// Posting straight to the Feed bypasses the command's role policy.
			withdrawn := mustPost(t, s, Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "withdrawn", "text": "No longer needed"}}, person)
			if tc.afterAnswer {
				answer()
			}
			read, err := s.ReadOnly(Filter{})
			if err != nil {
				t.Fatal(err)
			}
			for i := range read.Records {
				e := &read.Records[i]
				if e.ID != withdrawn.ID {
					continue
				}
				if e.Verdict != "counted" {
					t.Fatalf("withdraw not counted: %+v", e)
				}
				if tc.verdict != "" {
					e.Verdict = tc.verdict
				}
				if tc.badRe {
					e.Re = nil
				}
				if tc.badTarget {
					e.Body["target"] = testMachine
				}
				// Defense in depth even if an upstream reader marked a non-person counted.
				e.Author.Role, e.Tier = tc.role, contract.TierFor(tc.role)
			}
			state, decisions := FoldReadAt(read, now, tc.allowed)
			d := decisions[q.ID]
			if d.State != tc.want || state.Questions[q.ID].Open != (tc.want == "open") || (len(state.NeedsYou) == 1) != (tc.want == "open") {
				t.Fatalf("withdraw state=%s needs-you=%v; want %s", d.State, state.NeedsYou, tc.want)
			}
			for _, p := range d.Events {
				if p.ID == withdrawn.ID && tc.want != "withdrawn" {
					t.Fatal("refused withdrawal entered history")
				}
			}
			if tc.want == "answered" && (d.Answer != "Yes" || state.Questions[q.ID].Applied == nil) {
				t.Fatalf("lost applied answer: %+v", d)
			}
		})
	}
}

func TestQuestionControlsRetainTenureOrder(t *testing.T) {
	s := testStore(t)
	q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": "2000-01-01T00:00:00.000Z"}}}, testWriter())
	w := taskWriter(t, s)
	first := mustPost(t, s, Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "defaulted", "text": "Wait"}}, w)
	last := mustPost(t, s, Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "withdrawn", "text": "No longer needed"}}, w)
	// The display order disagrees with tenure order; the earlier default wins.
	read := ReadResult{Records: []Entry{{Record: q, Verdict: "counted"}, {Record: last, Verdict: "counted"}, {Record: first, Verdict: "counted"}}}
	_, decisions := FoldReadAt(read, time.Now())
	if d := decisions[q.ID]; d.State != "defaulted" || len(d.Events) != 1 || d.Events[0].ID != first.ID {
		t.Fatalf("tenure order lost: %+v", d)
	}
}

func TestQuestionStateUsesWorkflowWithdrawalPolicy(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "projects", "example", "tasks", "T-01-example")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	policy := "# Workflows\n\n## Default\napplies when: nothing else matches\nreview rounds: 1\nquestion_withdraw_roles: owner\n\n| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Build | run | grid-builder | |\n"
	for path, text := range map[string]string{filepath.Join(home, "workflows.md"): policy, filepath.Join(dir, "task.md"): "---\nid: " + testTask + "\n---\n"} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s := Store{TaskDir: dir, TaskID: testTask}
	q := mustPost(t, s, Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person", "default": "Wait", "at": "2099-01-01T00:00:00.000Z"}}}, testWriter())
	mustPost(t, s, Input{Type: "control", Re: q.ID, Body: map[string]any{"target": q.ID, "question_event": "withdrawn", "text": "Forged"}}, testWriter())
	state, err := s.State()
	if err != nil || !state.Questions[q.ID].Open || len(state.NeedsYou) != 1 {
		t.Fatalf("workflow-refused member closed gate: %+v %v", state, err)
	}
}
