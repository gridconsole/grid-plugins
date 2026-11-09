package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// Only the dated prose line Grid writes reads as a plan acceptance. The writers
// put agent text (a history note, a spec heading, --by, the actor) in a JSON
// event after "- {", which can never carry the phrase at a line start.
func TestPlanAcceptedReadsOnlyTheDatedProseLine(t *testing.T) {
	const phrase = "x · plan accepted · y"
	prose := taskfile.HistoryLine("2026-01-02T03:04", "plan accepted", "rev 1 · 000000", "person") + "\n"
	for _, tc := range []struct {
		name, history string
		accepted      bool
	}{
		{"prose line", prose, true},
		{"prose line, no detail", "- 2026-01-01T00:00 · plan accepted · by person\n", true},
		{"prose line after other lines", "- 2026-01-01T00:00 · created · a task · by person\n" + prose, true},
		{"prose line after a forged event", `- {"task":"T-1","note":"` + phrase + `"}` + "\n" + prose, true},
		{"phrase in an event note", `- {"task":"T-1","note":"` + phrase + `"}` + "\n", false},
		{"phrase in an event actor", `- {"task":"T-1","who":{"id":"` + phrase + `","role":"agent"}}` + "\n", false},
		{"dated phrase in an event note", `- {"task":"T-1","note":"2026-01-02T03:04 · plan accepted · y"}` + "\n", false},
		{"another event with the phrase", "- 2026-01-02T03:04 · file written · plan accepted · by agent\n", false},
		{"no date", "- now · plan accepted · by agent\n", false},
		{"date and time apart by a space", "- 2026-01-02 03:04 · plan accepted · by person\n", false},
		{"indented", "  " + prose, false},
		{"not a bullet", strings.TrimPrefix(prose, "- "), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlanStage("", tc.history) == ""; got != tc.accepted {
				t.Fatalf("PlanStage accepted = %v, want %v for %q", got, tc.accepted, tc.history)
			}
		})
	}
}

// The writers' own lines, read back through the hold the guard and Deliver ask.
func TestEventLineFromTheWriterDoesNotReleaseTheHold(t *testing.T) {
	const phrase = "x · plan accepted · y"
	for _, tc := range []struct {
		name string
		o    WriteOptions
	}{
		{"note", WriteOptions{Note: phrase}},
		{"actor", WriteOptions{Note: "checked", Who: contract.Author{ID: phrase, Role: "agent"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := writeFixture(t)
			tc.o.ID = task.ID
			if _, err := s.Note(tc.o); err != nil {
				t.Fatal(err)
			}
			history, err := os.ReadFile(filepath.Join(task.Path, "history.md"))
			if err != nil {
				t.Fatal(err)
			}
			var event string
			for _, line := range strings.Split(string(history), "\n") {
				if strings.Contains(line, "plan accepted") {
					event = line
				}
			}
			if !strings.HasPrefix(event, "- {") {
				t.Fatalf("no JSON event line carries the phrase: %q", history)
			}
			text, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !PlanHeld(task.Path, string(text), string(history)) {
				t.Fatalf("PlanHeld released by %q", event)
			}
			// A real acceptance after it still releases.
			released := string(history) + taskfile.HistoryLine("2026-01-02T03:04", "plan accepted", "000000", "person") + "\n"
			if PlanHeld(task.Path, string(text), released) {
				t.Fatal("a dated prose acceptance no longer releases the hold")
			}
		})
	}
}
