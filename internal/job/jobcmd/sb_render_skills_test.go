package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// sbSkillsRender renders the fixture Job with skills research and house-style,
// the house-style skill in the home brain, and settingsAt (a folder under the
// Grid home) holding settings.json with research off. It returns the saved brief.
func sbSkillsRender(t *testing.T, settingsAt func(h jobtest.Home) string) string {
	t.Helper()
	e, _, _ := givenFixture(t, []string{"brain:project/export-format"}, func(h jobtest.Home) {
		plan := filepath.Join(h.Task, "plan.md")
		b, err := os.ReadFile(plan)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "skills: []") {
			t.Fatal("fixture plan lost its skills line")
		}
		if err := os.WriteFile(plan, []byte(strings.Replace(string(b), "skills: []", "skills: [research, house-style]", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		skill := filepath.Join(h.GridHome, "knowledge", "skills", "house-style")
		if err := os.MkdirAll(skill, 0o755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: house-style\ndescription: Our house style\n---\nSECRET BODY of the skill.\n"
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(settingsAt(h), "settings.json"), []byte(`{"skills":{"research":false}}`), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	brief, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	return string(brief)
}

func sbCheckSkillsBlock(t *testing.T, brief, home string) {
	t.Helper()
	want := "- house-style (brain skill): read " + filepath.Join(home, "knowledge", "skills", "house-style", "SKILL.md") + " before you start. Our house style\n"
	if !strings.Contains(brief, want) {
		t.Fatalf("the brief lacks the brain skill line %q:\n%s", want, brief)
	}
	_, section, _ := strings.Cut(brief, "### Skills\n")
	section, _, _ = strings.Cut(section, "\n\n")
	if strings.Contains(section, "research") {
		t.Fatalf("research is off but the Skills block names it:\n%s", section)
	}
	if strings.Contains(brief, "SECRET BODY") {
		t.Fatal("the brain SKILL.md body was pasted into the brief")
	}
}

func TestSbRenderReadsSettingsAndBrain(t *testing.T) {
	var home string
	// The task is under projects/demo, so the project's own settings.json turns research off.
	brief := sbSkillsRender(t, func(h jobtest.Home) string {
		home = h.GridHome
		return filepath.Join(h.GridHome, "projects", jobtest.Project)
	})
	sbCheckSkillsBlock(t, brief, home)
}

func TestSbRenderReadsPersonalSettings(t *testing.T) {
	var home string
	// The personal settings.json applies to the project's Jobs too.
	brief := sbSkillsRender(t, func(h jobtest.Home) string {
		home = h.GridHome
		return h.GridHome
	})
	sbCheckSkillsBlock(t, brief, home)
}
