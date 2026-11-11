package packaging

import (
	"strings"
	"testing"
)

// setupAnswersLaneProblems runs checkPluginFiles on the shipped skills only and
// returns the problems that name skills/setup/SKILL.md.
func setupAnswersLaneProblems(t *testing.T, surface contractSurface) []string {
	t.Helper()
	_, skills, skillDirs := readPluginFiles(t)
	problems, _ := checkPluginFiles(map[string]string{}, skills, skillDirs, surface)
	var out []string
	for _, p := range problems {
		if strings.HasPrefix(p, "skills/setup/SKILL.md") {
			out = append(out, p)
		}
	}
	return out
}

// With --answers dropped from setup in memory (never in the file), the skill's
// `grid setup <project> --answers ...` is reported; with the full surface it is not.
func TestSetupAnswersSkillCheckBitesWithoutTheFlag(t *testing.T) {
	full := loadContractSurface(t)
	if !full["setup"][""]["--answers"] {
		t.Fatal("commands.json does not list --answers under setup")
	}
	if got := setupAnswersLaneProblems(t, full); len(got) != 0 {
		t.Fatalf("full surface: skills/setup/SKILL.md has problems: %v", got)
	}

	cut := contractSurface{}
	for group, verbs := range full {
		cut[group] = map[string]contractFlags{}
		for verb, flags := range verbs {
			copied := contractFlags{}
			for name := range flags {
				if group == "setup" && verb == "" && name == "--answers" {
					continue
				}
				copied[name] = true
			}
			cut[group][verb] = copied
		}
	}
	got := setupAnswersLaneProblems(t, cut)
	if len(got) == 0 {
		t.Fatal("without setup --answers, checkPluginFiles reported nothing for skills/setup/SKILL.md")
	}
	for _, p := range got {
		if !strings.Contains(p, "--answers") {
			t.Errorf("problem does not name --answers: %s", p)
		}
	}
	t.Logf("problems without the flag: %v", got)
}
