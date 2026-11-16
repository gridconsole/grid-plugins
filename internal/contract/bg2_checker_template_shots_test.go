package contract

import (
	"strings"
	"testing"
)

// The synced grid-checker subagent has no skills key, so the template itself
// must say to read a screenshot by numbers first and open only a crop.
func TestBg2CheckerTemplateReadsShotsByNumbers(t *testing.T) {
	text := readPluginFile(t, "agent-templates/grid-checker.md")
	for _, want := range []string{
		"grid shot diff",
		"grid shot crop",
		"taller than one viewport",
		"3840-wide",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("agent-templates/grid-checker.md lacks %q", want)
		}
	}
}
