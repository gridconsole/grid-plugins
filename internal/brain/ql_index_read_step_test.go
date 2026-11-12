package brain

import (
	"fmt"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

const qlReadStep = "Read a note: grid brain search --topic <topic> <words>, then grid brain show <id>.\n"

func qlRows(n int, desc string) []TopicRow {
	var rs []TopicRow
	for i := range n {
		rs = append(rs, TopicRow{Topic: fmt.Sprintf("topic-%02d", i), Description: desc, Entries: i})
	}
	return rs
}

func TestQlIndexNamesReadStep(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []TopicRow
		cut  bool
	}{
		{"3 rows", qlRows(3, "Short description."), false},
		{"12 rows of 80 bytes", qlRows(12, strings.Repeat("d", 80)), true},
		{"40 rows of 80 bytes", qlRows(40, strings.Repeat("d", 80)), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := SessionStartText(tc.rows)
			if !strings.HasSuffix(text, qlReadStep) {
				t.Fatalf("text does not end with the read step: %q", text)
			}
			if strings.Count(text, "grid brain show") != 1 {
				t.Fatalf("read step is not stated once: %q", text)
			}
			if len(text) > 1200 {
				t.Fatalf("%d bytes over the 1,200 budget", len(text))
			}
			if tc.cut {
				shown := strings.Count(text, "\ntopic-")
				banner := fmt.Sprintf("%d topics cut; grid brain topics lists them.\n", len(tc.rows)-shown)
				if shown == 0 || !strings.Contains(text, banner+qlReadStep) {
					t.Fatalf("banner then closing line missing: %q", text)
				}
			} else if strings.Contains(text, "cut") {
				t.Fatalf("banner on a short table: %q", text)
			}
		})
	}
}

func TestQlBrainKeeperClaims(t *testing.T) {
	body, ok := gridplugins.SkillFor("brain-keeper")
	if !ok {
		t.Fatal("brain-keeper skill missing")
	}
	if strings.Contains(body, "one line per note") {
		t.Error("brain-keeper still says the index carries one line per note")
	}
	if !strings.Contains(body, "topics/<topic>/TOPIC.md") {
		t.Error("brain-keeper does not name topics/<topic>/TOPIC.md")
	}
	if strings.Contains(body, "`knowledge/<topic>/TOPIC.md`") {
		t.Error("brain-keeper names the read fallback knowledge/<topic>/TOPIC.md as the definition")
	}
	if got := TopicFile("x"); got != "topics/x/TOPIC.md" {
		t.Errorf("TopicFile(x) = %q", got)
	}
	if !strings.Contains(body, "has no Grid record") {
		t.Error("brain-keeper does not say an unregistered repo has no Grid record")
	}
}
