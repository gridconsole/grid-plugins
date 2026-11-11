package contract

import (
	"regexp"
	"strings"
	"testing"
)

var gateWord = regexp.MustCompile(`(?i)\bgates?\b`)

// TestDtGateWordsTemplates: the planner and curator templates speak of no fixed
// gate and never send an agent to `question add`.
func TestDtGateWordsTemplates(t *testing.T) {
	for _, rel := range []string{"agent-templates/grid-planner.md", "agent-templates/grid-curator.md"} {
		text := readPluginFile(t, rel)
		for i, line := range strings.Split(text, "\n") {
			if gateWord.MatchString(strings.ReplaceAll(line, "gate:Q-<n>", "")) {
				t.Errorf("%s:%d says gate: %s", rel, i+1, line)
			}
			if strings.Contains(line, "question add") {
				t.Errorf("%s:%d says question add: %s", rel, i+1, line)
			}
		}
	}
	planner := readPluginFile(t, "agent-templates/grid-planner.md")
	for _, want := range []string{"grid question ask", "Person", "decisions_needed"} {
		if !strings.Contains(planner, want) {
			t.Errorf("grid-planner.md does not name %q", want)
		}
	}
}
