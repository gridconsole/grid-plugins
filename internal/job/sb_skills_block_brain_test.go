package job_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

const sbSkillBody = "SECRET BODY: never pasted into a brief.\n"

// sbRender renders the sample Job with the given skills and options.
func sbRender(t *testing.T, skills []string, set func(*job.RenderOptions)) string {
	t.Helper()
	j, f, _ := sample(t)
	j.Skills = skills
	o := job.RenderOptions{Resolver: f, Generation: "fixture-1"}
	if set != nil {
		set(&o)
	}
	got, e := job.Render(j, o)
	if e != nil {
		t.Fatal(e)
	}
	return got.Brief
}

func TestSbBrainSkillListed(t *testing.T) {
	const path = "/grid/knowledge/skills/house-style/SKILL.md"
	md := "---\nname: house-style\ndescription: Our house style\n---\n" + sbSkillBody
	brief := sbRender(t, []string{"house-style"}, func(o *job.RenderOptions) {
		o.BrainSkill = func(name string) (string, string, bool) {
			if name != "house-style" {
				return "", "", false
			}
			return md, path, true
		}
	})
	want := "- house-style (brain skill): read " + path + " before you start. Our house style\n"
	if !strings.Contains(brief, want) {
		t.Fatalf("want line %q in:\n%s", want, brief)
	}
	if strings.Contains(brief, "(not in this plugin)") {
		t.Fatalf("a brain skill is marked as not in this plugin:\n%s", brief)
	}
	if strings.Contains(brief, "SECRET BODY") {
		t.Fatalf("the SKILL.md body was pasted:\n%s", brief)
	}
}

func TestSbSkillOffLeftOut(t *testing.T) {
	off := func(o *job.RenderOptions) { o.SkillOff = func(n string) bool { return n == "research" } }
	brief := sbRender(t, []string{"plan-jobs", "research"}, off)
	if !strings.Contains(brief, "- grid:plan-jobs: load it with the Skill tool before you start") {
		t.Fatalf("the plan-jobs line is missing:\n%s", brief)
	}
	_, section, _ := strings.Cut(brief, "### Skills\n")
	section, _, _ = strings.Cut(section, "\n\n")
	if strings.Contains(section, "research") {
		t.Fatalf("a research line is left:\n%s", section)
	}
	if got := skillLines(brief); len(got) != 1 {
		t.Fatalf("want one skill line, got %q", got)
	}

	all := sbRender(t, []string{"plan-jobs", "research"}, func(o *job.RenderOptions) { o.SkillOff = func(string) bool { return true } })
	if strings.Contains(all, "### Skills") {
		t.Fatalf("every skill is off but a Skills section is left:\n%s", all)
	}

	// A name the plugin and the brain both lack keeps today's line, with a brain func present.
	none := sbRender(t, []string{"no-such-skill"}, func(o *job.RenderOptions) {
		o.BrainSkill = func(string) (string, string, bool) { return "", "", false }
		o.SkillOff = func(string) bool { return false }
	})
	if !strings.Contains(none, "- grid:no-such-skill (not in this plugin): load it with the Skill tool before you start\n") {
		t.Fatalf("the not-in-this-plugin line changed:\n%s", none)
	}
}
