package job_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func pluginFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func frontAndBody(t *testing.T, name, text string) (front, body string) {
	t.Helper()
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		t.Fatalf("%s has no front matter", name)
	}
	front, body, ok = strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("%s front matter is not closed", name)
	}
	return front, strings.TrimSpace(body)
}

func frontValue(front, key string) string {
	for _, l := range strings.Split(front, "\n") {
		if v, ok := strings.CutPrefix(l, key+":"); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// Every template role a Job can name has instructions: a role with an agents/<role>.md
// gets that file's body, and a role with only its template gets the template's body.
func TestEveryTemplateRoleResolvesForAJob(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "agent-templates", "grid-*.md"))
	if err != nil || len(files) < 10 {
		t.Fatalf("agent-templates/grid-*.md: %d files, err %v", len(files), err)
	}
	for _, f := range files {
		role := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(f), "grid-"), ".md")
		src := "agents/" + role + ".md"
		if _, err := os.Stat(filepath.Join("..", "..", src)); err != nil {
			src = "agent-templates/grid-" + role + ".md"
		}
		_, body := frontAndBody(t, src, pluginFile(t, src))
		got := renderRole(t, role)
		if n := strings.Count(got.Brief, "### Instructions\n"); n != 1 {
			t.Errorf("%s: %d Instructions sections, want 1", role, n)
			continue
		}
		if !strings.Contains(got.Brief, "### Instructions\n\n"+body+"\n\n") {
			t.Errorf("%s: the brief lacks the body of %s", role, src)
		}
	}
}

// A role defined in both folders must not say two things. Each case is a fact the
// two copies disagreed on.
func TestRoleCopiesAgree(t *testing.T) {
	for _, role := range []string{"builder", "checker", "curator", "reviewer"} {
		agent := pluginFile(t, "agents/"+role+".md")
		tpl := pluginFile(t, "agent-templates/grid-"+role+".md")
		if !strings.Contains(agent, "grid job result") {
			continue
		}
		if !strings.Contains(tpl, "grid job result") {
			t.Errorf("agent-templates/grid-%s.md never says to post the result with `grid job result`, agents/%s.md does", role, role)
		}
		front, _ := frontAndBody(t, role, tpl)
		if !regexp.MustCompile(`\bBash\b`).MatchString(frontValue(front, "tools")) {
			t.Errorf("agent-templates/grid-%s.md has no Bash in tools, so it cannot run `grid job result`", role)
		}
	}

	checker, _ := frontAndBody(t, "grid-checker", pluginFile(t, "agent-templates/grid-checker.md"))
	if d := frontValue(checker, "description"); !strings.Contains(d, "builder or reviewer") {
		t.Errorf("grid-checker description %q does not say it runs after a builder or reviewer, as agents/checker.md does", d)
	}

	if !strings.Contains(pluginFile(t, "agent-templates/grid-curator.md"), "decided-by") {
		t.Error("agent-templates/grid-curator.md does not link its own Job with rel decided-by, as agents/curator.md does")
	}

	planner := pluginFile(t, "agent-templates/grid-planner.md")
	if strings.Contains(planner, "then start the agent named in its role") {
		t.Error("grid-planner.md says the planner starts the agent of a Job; agents/orchestrator.md says that loop runs in code")
	}
	if !strings.Contains(planner, "runs in code") {
		t.Error("grid-planner.md does not say the loop that hands Jobs to agents runs in code")
	}
	for _, l := range strings.Split(planner, "\n") {
		if strings.HasPrefix(l, "| `check`") && !strings.Contains(l, "worker posts") {
			t.Errorf("grid-planner.md check row %q does not say the worker posts the review of a run step's Job, as agents/orchestrator.md does", l)
		}
	}
}
