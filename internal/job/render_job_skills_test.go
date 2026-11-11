package job_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func renderSkills(t *testing.T, skills []string) string {
	t.Helper()
	j, f, _ := sample(t)
	j.Skills = skills
	got, e := job.Render(j, job.RenderOptions{Resolver: f, Generation: "fixture-1"})
	if e != nil {
		t.Fatal(e)
	}
	return got.Brief
}

// descriptionOf reads the description: line of a skill from the embedded SKILL.md.
func descriptionOf(t *testing.T, name string) string {
	t.Helper()
	md, ok := gridplugins.SkillFor(name)
	if !ok {
		t.Fatalf("the plugin carries no skill %q", name)
	}
	for _, line := range strings.Split(md, "\n") {
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Fatalf("skills/%s/SKILL.md has no description", name)
	return ""
}

func skillLines(brief string) []string {
	var out []string
	for _, l := range strings.Split(brief, "\n") {
		if strings.HasPrefix(l, "- grid:") {
			out = append(out, l)
		}
	}
	return out
}

func TestJobSkillsAreNamedAsGridSkills(t *testing.T) {
	brief := renderSkills(t, []string{"plan-jobs"})
	if !strings.Contains(brief, "grid:plan-jobs") {
		t.Fatalf("the brief never names grid:plan-jobs:\n%s", brief)
	}
	if n := strings.Count(brief, "### Skills\n"); n != 1 {
		t.Fatalf("%d Skills sections", n)
	}
	if !strings.Contains(brief, "- grid:plan-jobs: load it with the Skill tool before you start") {
		t.Fatalf("the plan-jobs line is missing:\n%s", brief)
	}
}

func TestJobSkillsBlockFollowsInstructions(t *testing.T) {
	j, f, _ := sample(t)
	j.Role = "orchestrator"
	j.Skills = []string{"plan-jobs"}
	got, e := job.Render(j, job.RenderOptions{Resolver: f, Generation: "fixture-1"})
	if e != nil {
		t.Fatal(e)
	}
	i, k, a := strings.Index(got.Brief, "### Instructions\n"), strings.Index(got.Brief, "### Skills\n"), strings.Index(got.Brief, "## Out of scope")
	if i < 0 || k < i || a < k {
		t.Fatalf("Skills must sit after Instructions and before Out of scope: %d %d %d", i, k, a)
	}
}

func TestJobSkillLineCarriesTheSkillDescription(t *testing.T) {
	want := descriptionOf(t, "plan-jobs")
	lines := skillLines(renderSkills(t, []string{"plan-jobs"}))
	if len(lines) != 1 || !strings.HasSuffix(lines[0], ". "+want) {
		t.Fatalf("want one line ending in the front matter description %q, got %q", want, lines)
	}
}

func TestEveryPluginSkillRendersItsDescription(t *testing.T) {
	dirs, e := os.ReadDir(filepath.Join("..", "..", "skills"))
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		want := descriptionOf(t, d.Name())
		lines := skillLines(renderSkills(t, []string{d.Name()}))
		if len(lines) != 1 || strings.Contains(lines[0], "not in this plugin") || strings.Trim(want, `"'`) == "" || !strings.Contains(lines[0], strings.Trim(want, `"'`)) {
			t.Fatalf("%s: line %q does not carry description %q", d.Name(), lines, want)
		}
	}
}

func TestJobSkillsNameMatchesAreNotDoubledAndMissingOnesAreMarked(t *testing.T) {
	brief := renderSkills(t, []string{"grid:plan-jobs", "no-such-skill"})
	// The metadata block above ## Intent echoes the skills field as written; count the body.
	_, brief, _ = strings.Cut(brief, "\n## Intent")
	if n := strings.Count(brief, "grid:plan-jobs"); n != 1 {
		t.Fatalf("grid:plan-jobs written %d times, want 1:\n%s", n, brief)
	}
	if strings.Contains(brief, "grid:grid:") {
		t.Fatal("a grid: prefix was doubled")
	}
	if !strings.Contains(brief, "- grid:no-such-skill (not in this plugin)") {
		t.Fatalf("the missing skill is not marked:\n%s", brief)
	}
	if got := skillLines(renderSkills(t, []string{"plan-jobs", "grid:plan-jobs"})); len(got) != 1 {
		t.Fatalf("the same skill written two ways listed %d times", len(got))
	}
}

func TestJobWithoutSkillsRendersAsBefore(t *testing.T) {
	for name, skills := range map[string][]string{"no-skills-key.md": nil, "empty-skills.md": {}} {
		want, e := os.ReadFile(filepath.Join("testdata", "render_job_skills", name))
		if e != nil {
			t.Fatal(e)
		}
		got := renderSkills(t, skills)
		if got != string(want) {
			t.Fatalf("%s: the render changed for a Job with no skills", name)
		}
		if strings.Contains(got, "### Skills") {
			t.Fatalf("%s: an empty Skills block", name)
		}
	}
}
