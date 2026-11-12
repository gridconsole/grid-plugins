package question

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// bbWaitAsk is the planner's own-action question: "Not yet" keeps it waiting.
func bbWaitAsk(extra ...string) []string {
	args := []string{"ask", "Connect the browser", "--who", "owner", "--option", "Done, carry on", "--option", "Not yet", "--recommended", "Done, carry on", "--default", "Done, carry on", "--applies", start.Add(24 * time.Hour).Format(time.RFC3339)}
	return append(args, extra...)
}

// bbWaitEnv is the command env of the package tests with the fake JobPort.
func bbWaitEnv(t *testing.T) (Env, Store, *jobCalls, func() string) {
	t.Helper()
	env, s, _, errout := commandEnv(t)
	calls := &jobCalls{}
	env.Jobs = calls.port("J-7", nil)
	s.Jobs = env.Jobs
	return env, s, calls, errout.String
}

// bbWaitFile reads questions/Q-1.md as written from the Feed.
func bbWaitFile(t *testing.T, s Store) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.TaskDir, "questions", "Q-1.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// bbWaitProjected rebuilds Q-1 from the Feed alone, as Sync does.
func bbWaitProjected(t *testing.T, s Store) Record {
	t.Helper()
	records, err := s.Sync()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("records %+v", records)
	}
	return records[0]
}

// bbWaitWaitsOn reads waits_on from the record's JSON, as the question file holds it.
func bbWaitWaitsOn(t *testing.T, r Record) []string {
	t.Helper()
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		WaitsOn []string `json:"waits_on"`
	}
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got.WaitsOn
}

func bbWaitKinds(r Record) []string {
	kinds := []string{}
	for _, e := range r.History {
		kinds = append(kinds, e.Kind+":"+e.Text)
	}
	return kinds
}

func TestBbWaitNotYetKeepsOpen(t *testing.T) {
	env, s, calls, stderr := bbWaitEnv(t)
	if code := Run(bbWaitAsk("--waits-on", "Not yet"), env); code != 0 {
		t.Fatalf("ask --waits-on: exit %d: %s", code, stderr())
	}
	for i := 0; i < 2; i++ {
		if code := RunAnswer([]string{s.TaskDir, "Q-1", "Not yet"}, env); code != 0 {
			t.Fatalf("answer %d 'Not yet': exit %d: %s", i+1, code, stderr())
		}
	}
	r := bbWaitProjected(t, s)
	if r.State != "open" || r.Answer != "" {
		t.Fatalf("after 'Not yet' twice: state %q answer %q, want open and no answer", r.State, r.Answer)
	}
	if got := bbWaitWaitsOn(t, r); !slices.Equal(got, []string{"Not yet"}) {
		t.Fatalf("waits_on %v did not come back from the Feed", got)
	}
	if got, want := bbWaitKinds(r), []string{"waiting:Not yet", "waiting:Not yet"}; !slices.Equal(got, want) {
		t.Fatalf("history %v, want %v", got, want)
	}
	for _, e := range r.History {
		if e.Who != "owner" || e.Role != "owner" || e.Post == "" || e.At.IsZero() {
			t.Fatalf("waiting event lacks author, post or time: %+v", e)
		}
	}
	if len(calls.closes) != 0 {
		t.Fatalf("person Job closed after 'Not yet': %v", calls.closes)
	}
	file := bbWaitFile(t, s)
	if !strings.Contains(file, `"waits_on": [`) || !strings.Contains(file, `"kind": "waiting"`) || !strings.Contains(file, "| open\n") {
		t.Fatalf("question file does not hold the waiting record:\n%s", file)
	}
	// The Feed keeps the question in Needs you, so the Job behind it stays held.
	f, err := s.feed()
	if err != nil {
		t.Fatal(err)
	}
	read, err := f.ReadOnly(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	state, _ := feed.FoldReadAt(read, start)
	if !slices.Contains(state.NeedsYou, r.Post) {
		t.Fatalf("Needs you %v lacks the waiting question %s", state.NeedsYou, r.Post)
	}
}

func TestBbWaitThenDone(t *testing.T) {
	t.Run("owner", func(t *testing.T) {
		env, s, calls, stderr := bbWaitEnv(t)
		if code := Run(bbWaitAsk("--waits-on", "Not yet"), env); code != 0 {
			t.Fatalf("ask: exit %d: %s", code, stderr())
		}
		for _, text := range []string{"Not yet", "Done, carry on"} {
			if code := RunAnswer([]string{s.TaskDir, "Q-1", text}, env); code != 0 {
				t.Fatalf("answer %q: exit %d: %s", text, code, stderr())
			}
		}
		r := bbWaitProjected(t, s)
		if r.State != "answered" || r.Answer != "Done, carry on" {
			t.Fatalf("state %q answer %q, want answered 'Done, carry on'", r.State, r.Answer)
		}
		if got, want := bbWaitKinds(r), []string{"waiting:Not yet", "answered:Done, carry on"}; !slices.Equal(got, want) {
			t.Fatalf("history %v, want %v", got, want)
		}
		if want := [][2]string{{"J-7", "Done, carry on"}}; !slices.Equal(calls.closes, want) {
			t.Fatalf("closes %v, want %v", calls.closes, want)
		}
		// 'Not yet' after the answer is the different answer the base refuses.
		if code := RunAnswer([]string{s.TaskDir, "Q-1", "Not yet"}, env); code != 1 || !strings.Contains(stderr(), "Q-1 is answered") {
			t.Fatalf("'Not yet' after 'Done, carry on': exit %d: %s", code, stderr())
		}
	})
	t.Run("guest addressed", func(t *testing.T) {
		env, s, calls, stderr := bbWaitEnv(t)
		if code := Run(bbWaitAsk("--who", "sam", "--waits-on", "Not yet"), env); code != 0 {
			t.Fatalf("ask: exit %d: %s", code, stderr())
		}
		other := s
		other.Writer.Author = feed.Author{ID: "pat", Role: "guest"}
		if _, err := other.Answer("Q-1", "Not yet"); err == nil || err.Error() != "guest answer must name a question addressed to that guest" {
			t.Fatalf("guest not addressed: err %v", err)
		}
		s.Writer.Author, s.Writer.Worker = feed.Author{ID: "sam", Role: "guest"}, "sam@"+questionMachine
		r, err := s.Answer("Q-1", "Not yet")
		if err != nil {
			t.Fatal(err)
		}
		if r = bbWaitProjected(t, s); r.State != "open" || len(calls.closes) != 0 || !slices.Equal(bbWaitKinds(r), []string{"waiting:Not yet"}) || r.History[0].Role != "guest" || r.History[0].Who != "sam" {
			t.Fatalf("guest 'Not yet': %+v closes %v", r, calls.closes)
		}
		if _, err = s.Answer("Q-1", "Done, carry on"); err != nil {
			t.Fatal(err)
		}
		if r = bbWaitProjected(t, s); r.State != "answered" || r.Answer != "Done, carry on" {
			t.Fatalf("guest 'Done, carry on': %+v", r)
		}
		if want := [][2]string{{"J-7", "Done, carry on"}}; !slices.Equal(calls.closes, want) {
			t.Fatalf("closes %v, want %v", calls.closes, want)
		}
	})
}

func TestBbWaitRefusesUnknownLabel(t *testing.T) {
	env, s, calls, stderr := bbWaitEnv(t)
	code := Run(bbWaitAsk("--waits-on", "Later"), env)
	msg := stderr()
	if code == 0 || !strings.Contains(msg, `"Later"`) || strings.Count(strings.TrimSpace(msg), "\n") != 0 {
		t.Fatalf("--waits-on Later: exit %d, stderr %q; want non-zero and one line naming the label", code, msg)
	}
	if _, err := os.Stat(filepath.Join(s.TaskDir, "questions", "Q-1.md")); err == nil {
		t.Fatal("refused ask wrote questions/Q-1.md")
	}
	if got := list(t, s); len(got) != 0 || len(calls.posts) != 0 {
		t.Fatalf("refused ask posted: questions %+v, Jobs %d", got, len(calls.posts))
	}
}

func TestBbWaitNoFlagAsBase(t *testing.T) {
	env, s, calls, stderr := bbWaitEnv(t)
	if code := Run(bbWaitAsk(), env); code != 0 {
		t.Fatalf("ask: exit %d: %s", code, stderr())
	}
	if code := RunAnswer([]string{s.TaskDir, "Q-1", "Not yet"}, env); code != 0 {
		t.Fatalf("answer 'Not yet': exit %d: %s", code, stderr())
	}
	r := bbWaitProjected(t, s)
	if r.State != "answered" || r.Answer != "Not yet" || bbWaitWaitsOn(t, r) != nil {
		t.Fatalf("state %q answer %q waits_on %v, want answered 'Not yet' as on the base", r.State, r.Answer, bbWaitWaitsOn(t, r))
	}
	if want := [][2]string{{"J-7", "Not yet"}}; !slices.Equal(calls.closes, want) {
		t.Fatalf("closes %v, want %v", calls.closes, want)
	}
	if file := bbWaitFile(t, s); strings.Contains(file, "waits_on") {
		t.Fatalf("question asked without --waits-on writes waits_on:\n%s", file)
	}
	if code := RunAnswer([]string{s.TaskDir, "Q-1", "Done, carry on"}, env); code != 1 || !strings.Contains(stderr(), "Q-1 is answered") {
		t.Fatalf("later 'Done, carry on': exit %d: %s", code, stderr())
	}
}

// A waiting control on the Feed counts only from a person who may answer the
// question, with no lease, and only for a label the question names.
func TestBbWaitFeedFold(t *testing.T) {
	env, s, _, stderr := bbWaitEnv(t)
	if code := Run(bbWaitAsk("--who", "sam", "--waits-on", "Not yet"), env); code != 0 {
		t.Fatalf("ask: exit %d: %s", code, stderr())
	}
	rec := list(t, s)[0]
	f, err := s.feed()
	if err != nil {
		t.Fatal(err)
	}
	post := func(w feed.Writer, text string) error {
		_, err := f.Post(feed.Input{Type: "control", Re: rec.Post, Body: map[string]any{"target": rec.Post, "question_event": "waiting", "text": text}}, w)
		return err
	}
	if err := post(s.ControlWriter, "Not yet"); err == nil {
		t.Fatal("Feed admitted a leased waiting control")
	}
	guest := s.Writer
	guest.Author, guest.Worker = feed.Author{ID: "pat", Role: "guest"}, "pat@"+questionMachine
	for _, tc := range []struct {
		w    feed.Writer
		text string
	}{{guest, "Not yet"}, {s.Writer, "Done, carry on"}, {s.Writer, "Not yet"}} {
		if err := post(tc.w, tc.text); err != nil {
			t.Fatal(err)
		}
	}
	r := bbWaitProjected(t, s)
	if r.State != "open" || r.Answer != "" || !slices.Equal(bbWaitKinds(r), []string{"waiting:Not yet"}) || r.History[0].Who != "owner" {
		t.Fatalf("fold kept %v (state %q), want only the owner's waiting:Not yet", bbWaitKinds(r), r.State)
	}
}
