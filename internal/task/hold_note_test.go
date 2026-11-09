package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// The full dated line, with its bullet, typed into a note or an actor id. The
// writers put it after "- {" in a JSON event, mid-line, where only a regexp that
// lost its line-start anchor would read it as a person's acceptance.
const datedAcceptance = "- 2026-01-02T03:04 · plan accepted · by person"

func TestDatedAcceptanceTypedIntoAWriterFieldDoesNotRelease(t *testing.T) {
	for _, tc := range []struct {
		name string
		o    WriteOptions
	}{
		{"note", WriteOptions{Note: datedAcceptance}},
		{"actor", WriteOptions{Note: "checked", Who: contract.Author{ID: datedAcceptance, Role: "agent"}}},
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
				if strings.Contains(line, datedAcceptance) {
					event = line
				}
			}
			// The row proves nothing unless the whole line sits inside a JSON event.
			if !strings.HasPrefix(event, "- {") || strings.Count(event, datedAcceptance) != 1 {
				t.Fatalf("no JSON event line carries the dated line: %q", history)
			}
			text, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !PlanHeld(task.Path, string(text), string(history)) {
				t.Fatalf("PlanHeld released by %q", event)
			}
			released := string(history) + taskfile.HistoryLine("2026-01-02T03:04", "plan accepted", "000000", "person") + "\n"
			if PlanHeld(task.Path, string(text), released) {
				t.Fatal("a dated prose acceptance no longer releases the hold")
			}
		})
	}
}
