package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// sb3Render renders the fixture Job with skills research and house-style and the
// given personal settings.json. It returns the render report path's brief.
func sb3Render(t *testing.T, personal string) string {
	t.Helper()
	e, _, _ := givenFixture(t, []string{"brain:project/export-format"}, func(h jobtest.Home) {
		plan := filepath.Join(h.Task, "plan.md")
		b, err := os.ReadFile(plan)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(plan, []byte(strings.Replace(string(b), "skills: []", "skills: [research, house-style]", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		skill := filepath.Join(h.GridHome, "knowledge", "skills", "house-style")
		if err := os.MkdirAll(skill, 0o755); err != nil {
			t.Fatal(err)
		}
		md := "---\nname: house-style\ndescription: Our house style\n---\nbody\n"
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(md), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(h.GridHome, "settings.json"), []byte(personal), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if !r.OK {
		t.Fatalf("render report not ok: %+v", r)
	}
	brief, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	return string(brief)
}

func sb3SkillsSection(brief string) string {
	_, section, _ := strings.Cut(brief, "### Skills\n")
	section, _, _ = strings.Cut(section, "\n\n")
	return section
}

// The Grid app seeds its own keys (claudePath, maxTurns, agentSandbox) into the
// personal settings.json. The strict settings decoder refuses those keys, and
// render exited 1 on them; it must go on with the full Skills block.
func TestSb3RenderAcceptsSettingsWithAppKeys(t *testing.T) {
	brief := sb3Render(t, `{"claudePath":"/x/bin/claude","maxTurns":7,"agentSandbox":false}`)
	section := sb3SkillsSection(brief)
	for _, want := range []string{"research", "house-style"} {
		if !strings.Contains(section, want) {
			t.Fatalf("nothing is turned off but the Skills block lacks %q:\n%s", want, section)
		}
	}
}

// A settings document that is not JSON at all turns nothing off either.
func TestSb3RenderAcceptsUnreadableSettings(t *testing.T) {
	brief := sb3Render(t, `{not json`)
	if section := sb3SkillsSection(brief); !strings.Contains(section, "research") {
		t.Fatalf("an unreadable settings.json must turn nothing off:\n%s", section)
	}
}

// A skill turned off at the personal level stays out of the Skills block, also
// when the same document carries the app's keys.
func TestSb3PersonalOffStillLeftOut(t *testing.T) {
	brief := sb3Render(t, `{"claudePath":"/x/bin/claude","maxTurns":7,"skills":{"research":false}}`)
	section := sb3SkillsSection(brief)
	if strings.Contains(section, "research") {
		t.Fatalf("research is off at the personal level but the Skills block names it:\n%s", section)
	}
	if !strings.Contains(section, "house-style") {
		t.Fatalf("house-style is on but the Skills block lacks it:\n%s", section)
	}
}
