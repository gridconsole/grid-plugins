package job_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

func renderRole(t *testing.T, role string) job.Rendered {
	t.Helper()
	j, f, _ := sample(t)
	j.Role = role
	got, e := job.Render(j, job.RenderOptions{Resolver: f, Generation: "fixture-1"})
	if e != nil {
		t.Fatal(e)
	}
	return got
}

func agentFileBody(t *testing.T, role string) string {
	t.Helper()
	raw, e := os.ReadFile(filepath.Join("..", "..", "agents", role+".md"))
	if e != nil {
		t.Fatal(e)
	}
	rest, ok := strings.CutPrefix(string(raw), "---\n")
	if !ok {
		t.Fatalf("agents/%s.md has no front matter", role)
	}
	_, body, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("agents/%s.md front matter is not closed", role)
	}
	return strings.TrimSpace(body)
}

func TestBuilderBriefCarriesItsResultStep(t *testing.T) {
	got := renderRole(t, "builder")
	if !strings.Contains(got.Brief, "grid job result") {
		t.Fatal("a builder brief does not tell the agent to post its result with `grid job result`")
	}
}

func TestRoleInstructionsSection(t *testing.T) {
	for _, role := range []string{"builder", "reviewer", "orchestrator", "curator", "checker", "operator"} {
		body := agentFileBody(t, role)
		for _, spelling := range []string{role, "grid-" + role, strings.ToUpper(role[:1]) + role[1:]} {
			got := renderRole(t, spelling)
			if n := strings.Count(got.Brief, "### Instructions\n"); n != 1 {
				t.Fatalf("%s: %d Instructions sections", spelling, n)
			}
			if !strings.Contains(got.Brief, "### Instructions\n\n"+body+"\n\n") {
				t.Fatalf("%s: brief lacks the body of agents/%s.md", spelling, role)
			}
			if i, k := strings.Index(got.Brief, "## Job\n"), strings.Index(got.Brief, "### Instructions"); k < i {
				t.Fatalf("%s: Instructions come before ## Job", spelling)
			}
		}
	}
}

func TestRoleWithoutAgentFileGetsNoSection(t *testing.T) {
	base := renderRole(t, "writer")
	// researcher, scout, operator and designer left this list: their templates are their
	// instructions now (TestEveryTemplateRoleResolvesForAJob).
	for _, role := range []string{"person"} {
		got := renderRole(t, role)
		if strings.Contains(got.Brief, "### Instructions") {
			t.Fatalf("%s got an Instructions section", role)
		}
		if strings.ReplaceAll(got.Brief, `role: "`+role+`"`, `role: "writer"`) != base.Brief {
			t.Fatalf("%s: brief differs from the base render beyond the role line", role)
		}
	}
}

func TestInstructionsAreNotASourceOrBrainText(t *testing.T) {
	base := renderRole(t, "writer")
	base.Inputs.BriefBytes = 0 // the instructions make the brief longer, so brief_bytes differs by design
	bj, e := base.Inputs.JSON()
	if e != nil {
		t.Fatal(e)
	}
	for _, role := range []string{"builder", "reviewer", "orchestrator", "curator", "checker", "operator"} {
		got := renderRole(t, role)
		got.Inputs.BriefBytes = 0 // see above
		gj, e := got.Inputs.JSON()
		if e != nil {
			t.Fatal(e)
		}
		if string(gj) != string(bj) || got.Inputs.BrainTokens != base.Inputs.BrainTokens {
			t.Fatalf("%s: inputs.json or brain_tokens changed", role)
		}
	}
}

func TestCuratorAgentFileAndItsVerbs(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "..", "agents", "curator.md"))
	if e != nil {
		t.Fatal(e)
	}
	text := string(raw)
	head, _, _ := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	for _, key := range []string{"name: curator\n", "description: ", "model: ", "effort: ", "tools: ", "skills:\n  - brain-keeper"} {
		if !strings.Contains(head+"\n", key) {
			t.Fatalf("curator front matter lacks %q", key)
		}
	}
	if n := len(strings.Split(agentFileBody(t, "curator"), "\n")); n > 25 {
		t.Fatalf("curator body is %d lines, want at most 25", n)
	}
	for _, s := range []string{"grid brain keep --task", "grid brain show", "--proposal", "grid brain decide", "accept|merge-into", "grid brain index", "grid job result --task", "never ask a person"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(s)) {
			t.Fatalf("curator.md does not say %q", s)
		}
	}
	cmds, e := os.ReadFile(filepath.Join("..", "..", "contract", "commands.json"))
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name string `json:"name"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if e := json.Unmarshal(cmds, &doc); e != nil {
		t.Fatal(e)
	}
	known := map[string]bool{}
	for _, c := range doc.Commands {
		known[c.Name] = true
		for _, s := range c.Subcommands {
			known[c.Name+" "+s.Name] = true
		}
	}
	for _, v := range []string{"brain keep", "brain show", "brain decide", "brain index", "job result"} {
		if !known[v] {
			t.Fatalf("contract/commands.json has no `grid %s`", v)
		}
	}
}
