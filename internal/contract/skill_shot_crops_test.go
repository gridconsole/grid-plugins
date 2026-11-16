package contract

import (
	"strings"
	"testing"
)

// The browser and visual-design-review skills tell an agent to compare numbers
// first and open only a crop; the checker agent points at those rules.
func TestSkillsReadScreenshotsByNumbersThenCrops(t *testing.T) {
	for _, rel := range []string{"skills/browser/SKILL.md", "skills/visual-design-review/SKILL.md"} {
		text := readPluginFile(t, rel)
		if !strings.Contains(text, "## Numbers before pictures") {
			t.Errorf("%s has no \"Numbers before pictures\" section", rel)
		}
		for _, want := range []string{
			"grid shot diff",
			"grid shot crop",
			"1568",
			"taller than one viewport",
			"3840-wide screenshot whole",
			"widths to measure, not images to read",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("%s lacks %q", rel, want)
			}
		}
	}
	checker := readPluginFile(t, "agents/checker.md")
	for _, want := range []string{"Numbers before pictures", "grid shot crop"} {
		if !strings.Contains(checker, want) {
			t.Errorf("agents/checker.md lacks %q", want)
		}
	}
}
