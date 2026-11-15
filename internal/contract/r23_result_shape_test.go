package contract

import (
	"strings"
	"testing"
)

// Every agent that runs under a Job lease posts the run-job result body (outcome,
// changed, evidence, acceptance, ...). These tests read the shipped agent texts
// and pin the result shape the run-job skill shows, the found route the
// review-job skill shows, and the questions skill the reviewer and checker name.

// r23Front returns the front matter lines of an agent file.
func r23Front(t *testing.T, rel string) []string {
	t.Helper()
	text := strings.ReplaceAll(readPluginFile(t, rel), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		t.Fatalf("%s: no front matter", rel)
	}
	rest := text[4:]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		t.Fatalf("%s: front matter not closed", rel)
	}
	return strings.Split(rest[:end], "\n")
}

// r23Tools returns the names on the `tools:` line.
func r23Tools(t *testing.T, rel string) []string {
	t.Helper()
	for _, l := range r23Front(t, rel) {
		if v, ok := strings.CutPrefix(l, "tools:"); ok {
			var out []string
			for _, n := range strings.Split(v, ",") {
				out = append(out, strings.TrimSpace(n))
			}
			return out
		}
	}
	t.Fatalf("%s: no tools: line", rel)
	return nil
}

// r23Skills returns the `- name` items under `skills:`.
func r23Skills(t *testing.T, rel string) []string {
	t.Helper()
	var out []string
	in := false
	for _, l := range r23Front(t, rel) {
		switch {
		case strings.HasPrefix(l, "skills:"):
			in = true
		case in && strings.HasPrefix(l, "  - "):
			out = append(out, strings.TrimSpace(strings.TrimPrefix(l, "  - ")))
		default:
			in = false
		}
	}
	return out
}

func r23Has(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// Row 4: the reviewer template tells the agent to post findings with
// `grid job post --type found` (review-job skill), so its Bash sentence names
// the command and it has no Edit tool.
func TestR23ReviewerTemplatePostsFoundWithoutEdit(t *testing.T) {
	const rel = "agent-templates/grid-reviewer.md"
	if r23Has(r23Tools(t, rel), "Edit") {
		t.Errorf("%s: the tools: line has Edit, but the reviewer never edits the work", rel)
	}
	text := readPluginFile(t, rel)
	sentence := ""
	for _, p := range strings.Split(text, "\n") {
		if strings.Contains(p, "Use Bash only") {
			sentence = p
			break
		}
	}
	if sentence == "" {
		t.Fatalf("%s: no sentence says \"Use Bash only\"", rel)
	}
	if !strings.Contains(sentence, "`grid job post --type found`") {
		t.Errorf("%s: the \"Use Bash only\" sentence does not name `grid job post --type found`: %s", rel, sentence)
	}
	if strings.Contains(text, "Your only write is the verdict") {
		t.Errorf("%s: still says the verdict is the only write, which forbids the found records", rel)
	}
}

// Rows 5 and 11: the builder and operator templates end a blocked or refused
// Job with a result whose `outcome` says so, never a first line `Blocked:`,
// `BLOCKED`, `DONE` or `PARTIALLY DONE`, and never a `Noticed` section, which
// are not keys of the result body.
func TestR23WorkerTemplatesUseResultKeys(t *testing.T) {
	banned := []string{
		"first line is `Blocked:`",
		"is a `Blocked:` line",
		"Its first line is `DONE`",
		"`PARTIALLY DONE`",
		"exactly `BLOCKED`",
		"under `Noticed`",
		"post `Blocked:`",
	}
	for _, rel := range []string{"agent-templates/grid-builder.md", "agent-templates/grid-operator.md"} {
		text := readPluginFile(t, rel)
		for _, b := range banned {
			if strings.Contains(text, b) {
				t.Errorf("%s: still says %q, which is not a key of the run-job result body", rel, b)
			}
		}
		if !strings.Contains(text, "`outcome`") {
			t.Errorf("%s: never names the result's `outcome` key", rel)
		}
	}
	// The builder's refused-command paragraph posts a result before the turn ends.
	text := readPluginFile(t, "agent-templates/grid-builder.md")
	found := false
	for _, p := range strings.Split(text, "\n") {
		if !strings.Contains(p, "refused by the permission check") {
			continue
		}
		found = true
		if !strings.Contains(p, "post the result") || !strings.Contains(p, "`outcome`") {
			t.Errorf("agent-templates/grid-builder.md: the refusal paragraph ends the turn without posting a result with `outcome`: %s", p)
		}
	}
	if !found {
		t.Error("agent-templates/grid-builder.md: no paragraph about a command refused by the permission check")
	}
	// The operator reads the result shape from the run-job skill.
	if !r23Has(r23Skills(t, "agent-templates/grid-operator.md"), "run-job") {
		t.Error("agent-templates/grid-operator.md: names the run-job result body but does not list run-job under skills:")
	}
}

// Row 12: agents/reviewer.md and agents/checker.md send a person question
// through "the `questions` skill", so each lists it under skills:, which loads
// it with the agent. The reviewer also has the Skill tool; the checker's tools:
// line is pinned by TestBeWorkerSpawnToolsFromAgentFile.
func TestR23ReviewerAndCheckerCanUseQuestions(t *testing.T) {
	for _, rel := range []string{"agents/reviewer.md", "agents/checker.md"} {
		if !strings.Contains(readPluginFile(t, rel), "`questions` skill") {
			t.Fatalf("%s: no longer names the `questions` skill; update this test", rel)
		}
		if !r23Has(r23Skills(t, rel), "questions") {
			t.Errorf("%s: skills: does not list questions", rel)
		}
	}
	if !r23Has(r23Tools(t, "agents/reviewer.md"), "Skill") {
		t.Error("agents/reviewer.md: tools: has no Skill")
	}
}
