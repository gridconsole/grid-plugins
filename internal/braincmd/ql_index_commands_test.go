package braincmd

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// TestQlIndexCommandsRun takes the two commands from the closing line of the
// SessionStart index, fills them with a real note's topic, a word of its
// description and its id, and runs them.
func TestQlIndexCommandsRun(t *testing.T) {
	h := newHarness(t)
	entry := h.keepRecord(t, "checks")
	if code := h.run("keep", "accept", "checks", "--level", "home", "--json"); code != 0 {
		t.Fatalf("keep accept exit %d: %s", code, &h.err)
	}

	text := brain.SessionStartText([]brain.TopicRow{{Topic: "decisions", Description: "Before changing checks.", Entries: 1}})
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	last := lines[len(lines)-1]
	const prefix = "Read a note: "
	if !strings.HasPrefix(last, prefix) || !strings.HasSuffix(last, ".") {
		t.Fatalf("closing line: %q", last)
	}
	search, show, ok := strings.Cut(strings.TrimSuffix(strings.TrimPrefix(last, prefix), "."), ", then ")
	if !ok {
		t.Fatalf("closing line holds two commands: %q", last)
	}
	fill := strings.NewReplacer("<topic>", "decisions", "<words>", "changing", "<id>", entry.ID)

	args := strings.Fields(fill.Replace(search))
	if len(args) < 3 || args[0] != "grid" || args[1] != "brain" {
		t.Fatalf("search command: %q", search)
	}
	if code := h.run(append(args[2:], "--level", "home")...); code != 0 || !strings.Contains(h.out.String(), entry.ID) {
		t.Fatalf("%q: exit %d, out %q, err %q", search, code, &h.out, &h.err)
	}

	args = strings.Fields(fill.Replace(show))
	if len(args) < 3 || args[0] != "grid" || args[1] != "brain" {
		t.Fatalf("show command: %q", show)
	}
	if code := h.run(args[2:]...); code != 0 || !strings.Contains(h.out.String(), entry.ID) {
		t.Fatalf("%q: exit %d, out %q, err %q", show, code, &h.out, &h.err)
	}
}
