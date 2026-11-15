package orchestrator

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// r9OaPerson posts id as a Job for a person that decides goal, as the
// orchestrator posts one when the task needs a decision first.
func r9OaPerson(t *testing.T, f *fixture, id, goal string) {
	t.Helper()
	body := jobBody(t, id)
	body["role"] = "Person"
	body["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": goal}
	f.post(t, feed.Input{Type: "job", Body: body, Job: id}, f.s.Writer)
}

// TestR9OaAskedDecisionPostsNoSecondAsk: round 9, task T-02. The orchestrator
// Job J-1 asked how the old --out flag is handled with the person Job J-2 and
// named it in decisions_needed too. The decisions pass posted a second
// orchestrator Job that asked the same decision again as a question. A decision
// that names a person Job on the Feed is already asked: one decision, one ask.
func TestR9OaAskedDecisionPostsNoSecondAsk(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	r9OaPerson(t, f, "J-2", "Decide how the old --out flag is handled.")
	f.tick(t)
	abDnResult(t, f, "J-1", []string{"J-2: how the old --out flag is handled (removed, kept as an alias, or warned about)"})
	f.tick(t)
	abDnWant(t, f, "after the tick", "J-1", "J-2")
	f.tick(t)
	abDnWant(t, f, "after a second tick", "J-1", "J-2")
}

// TestR9OaOnlyOpenDecisionIsAsked: of the decisions of one result, the one a
// person Job already asks is left out of the decisions Job; the open one is
// asked, and the Job tells the orchestrator to ask only what is still open.
func TestR9OaOnlyOpenDecisionIsAsked(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	r9OaPerson(t, f, "J-2", "Decide how the old --out flag is handled.")
	f.tick(t)
	result := abDnResult(t, f, "J-1", []string{"Asked in J-2: how the old --out flag is handled", "Which export format: csv or json?"})
	f.tick(t)
	bodies := abDnWant(t, f, "after the tick", "J-1", "J-2", "J-3")
	body := bodies["J-3"]
	if want := "Decisions needed on J-1: Which export format: csv or json?"; body["title"] != want {
		t.Fatalf("title = %v, want %q", body["title"], want)
	}
	text := abDnA1(t, body)
	if !strings.HasPrefix(text, abDnA1Lead+result.ID+" (") {
		t.Fatalf("A1 %q does not name result %s", text, result.ID)
	}
	if strings.Contains(text, "--out") {
		t.Errorf("A1 %q asks the decision J-2 already asks", text)
	}
	for _, want := range []string{"Which export format: csv or json?", "already asks", "leave open", "left out and named in the result", "grid question ask", "grid job result"} {
		if !strings.Contains(text, want) {
			t.Errorf("A1 %q lacks %q", text, want)
		}
	}
}

// TestR9OaNonPersonJobIdStillAsks: a decision that names a Job that is not a
// person Job (here the build J-1 itself, or an id no Job has) is not asked yet.
func TestR9OaNonPersonJobIdStillAsks(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	r9OaPerson(t, f, "J-2", "Decide how the old --out flag is handled.")
	f.tick(t)
	abDnResult(t, f, "J-1", []string{"Should J-1 also rename the flag in J-20?"})
	f.tick(t)
	abDnWant(t, f, "after the tick", "J-1", "J-2", "J-3")
}
