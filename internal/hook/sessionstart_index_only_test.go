package hook

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// TestSessionStartPrintsTheIndexNotNoteBodies pins docs/decisions/standing-rules-in-the-brain.md:
// SessionStart carries the brain index only (topic, one-line description, count),
// never a note's body or summary; a worker reads a note with grid brain show.
func TestSessionStartPrintsTheIndexNotNoteBodies(t *testing.T) {
	f := eventSetup(t)
	s := brain.Store{GridHome: f.home}
	const body = "STANDING RULE BODY: never rename the export columns without a decision."
	const summary = "STANDING RULE SUMMARY kept out of the index."
	n := brain.Note{Name: "export-rule", Topic: "decisions", Type: "convention", Description: "Use when changing the export columns.", Summary: summary, Source: "task:T-01", Level: "home", Body: body, Origin: brain.Origin{Kind: "session", ID: "session-demo"}}
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept("home", "export-rule"); err != nil {
		t.Fatal(err)
	}
	out, log, code := f.run(t, "SessionStart", hookio.Input{})
	if code != 0 || log != "" {
		t.Fatalf("SessionStart: exit %d, log %q", code, log)
	}
	got := contextFrom(t, bytes.NewBufferString(out))
	if !strings.Contains(got, "decisions | Decisions and their reasons | 1 |") {
		t.Fatalf("the index row does not count the note:\n%s", got)
	}
	if !strings.Contains(got, "grid brain show <id>") {
		t.Fatalf("the index does not end with the read step:\n%s", got)
	}
	for _, leak := range []string{"STANDING RULE", "never rename the export columns", "Use when changing the export columns."} {
		if strings.Contains(got, leak) {
			t.Fatalf("SessionStart printed note text %q:\n%s", leak, got)
		}
	}
}
