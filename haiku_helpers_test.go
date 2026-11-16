package gridplugins

import (
	"os"
	"strings"
	"testing"
)

// hhFront is the value of one top-level front matter key of a plugin file.
func hhFront(t *testing.T, name, text, key string) string {
	t.Helper()
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		t.Fatalf("%s has no front matter", name)
	}
	front, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("%s front matter is not closed", name)
	}
	for _, l := range strings.Split(front, "\n") {
		if v, ok := strings.CutPrefix(l, key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func hhRead(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The scout, runner and library ship as plugin agents (grid:scout, grid:runner,
// grid:library), on Haiku with the tools their templates set, so every session that
// has the plugin can hand them a read or a run with no grid setup step.
func TestHhHelpersShipAsPluginAgents(t *testing.T) {
	for _, name := range []string{"scout", "runner", "library"} {
		agent, ok := AgentFileFor(name)
		if !ok {
			t.Errorf("agents/%s.md does not ship", name)
			continue
		}
		tpl := hhRead(t, "agent-templates/grid-"+name+".md")
		if got := hhFront(t, name, agent, "name"); got != name {
			t.Errorf("agents/%s.md: name %q, want %q", name, got, name)
		}
		if got := hhFront(t, name, agent, "model"); got != "haiku" {
			t.Errorf("agents/%s.md: model %q, want haiku", name, got)
		}
		for _, key := range []string{"tools", "description", "effort"} {
			if got, want := hhFront(t, name, agent, key), hhFront(t, name, tpl, key); got != want {
				t.Errorf("agents/%s.md: %s %q, want the template's %q", name, key, got, want)
			}
		}
		// A Job of this role renders the agents file, so it keeps the template's whole text.
		_, tplBody, _ := strings.Cut(strings.TrimPrefix(tpl, "---\n"), "\n---\n")
		if !strings.Contains(agent, strings.TrimSpace(tplBody)) {
			t.Errorf("agents/%s.md does not hold the whole body of agent-templates/grid-%s.md", name, name)
		}
	}
}

// run-job holds the hand-off rules, each one line with its reason; review-job holds the
// ones a reviewer reading code needs.
func TestHhSkillsHandHeavyReadsToHelpers(t *testing.T) {
	run, _ := SkillFor("run-job")
	review, _ := SkillFor("review-job")
	for _, c := range []struct{ skill, text, want string }{
		{"run-job", run, "## Helpers"},
		{"run-job", run, "`grid:scout`"},
		{"run-job", run, "`grid:runner`"},
		{"run-job", run, "line range"},
		{"review-job", review, "`grid:scout`"},
		{"review-job", review, "line range"},
	} {
		if !strings.Contains(c.text, c.want) {
			t.Errorf("skills/%s/SKILL.md lacks %q", c.skill, c.want)
		}
	}
}

// builder, reviewer and checker point at the rules in one line and paste none of them.
func TestHhAgentsPointAtHelperRules(t *testing.T) {
	for _, role := range []string{"builder", "reviewer", "checker"} {
		text, _ := AgentFileFor(role)
		n := 0
		for _, l := range strings.Split(text, "\n") {
			if strings.Contains(l, "Helpers") {
				n++
				if !strings.Contains(l, "run-job") {
					t.Errorf("agents/%s.md: the Helpers line does not name the run-job skill: %s", role, l)
				}
			}
		}
		if n != 1 {
			t.Errorf("agents/%s.md: %d lines point at the Helpers rules, want 1", role, n)
		}
		if strings.Contains(text, "only the failing lines") {
			t.Errorf("agents/%s.md pastes the rule text instead of pointing at it", role)
		}
	}
	// A builder or reviewer can only hand work to a helper with the Agent tool.
	for _, role := range []string{"builder", "reviewer"} {
		text, _ := AgentFileFor(role)
		if tools := hhFront(t, role, text, "tools"); !strings.Contains(", "+tools+",", ", Agent,") {
			t.Errorf("agents/%s.md: tools %q lack Agent", role, tools)
		}
	}
}
