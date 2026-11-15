package job_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// saBrainAgent writes knowledge/agents/<name>.md in a temp Grid home and returns what
// the brain answers for the role, as jobcmd passes it to Render.
func saBrainAgent(t *testing.T, name, role, body string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "knowledge", "agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nname: " + name + "\ndescription: from the brain\nmodel: sonnet\neffort: high\n---\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	md, level, ok := brain.Store{GridHome: home}.AgentFor(role)
	if !ok || level != "home" {
		t.Fatalf("AgentFor(%q) = %q, %v", role, level, ok)
	}
	return md
}

func saRenderWith(t *testing.T, role, agent string) job.Rendered {
	t.Helper()
	j, f, _ := sample(t)
	j.Role = role
	got, e := job.Render(j, job.RenderOptions{Resolver: f, Generation: "fixture-1", Agent: agent})
	if e != nil {
		t.Fatal(e)
	}
	return got
}

func TestSaBrainAgentRendered(t *testing.T) {
	body := "You research one question.\n\nWrite research.md and name every source."
	md := saBrainAgent(t, "grid-researcher", "researcher", body)
	got := saRenderWith(t, "researcher", md)
	want := "### Instructions\n\n" + body + "\n\n"
	if n := strings.Count(got.Brief, "### Instructions\n"); n != 1 {
		t.Fatalf("%d Instructions sections", n)
	}
	i := strings.Index(got.Brief, want)
	if i < 0 {
		t.Fatalf("brief lacks %q", want)
	}
	if k := strings.Index(got.Brief, "## Job\n"); k < 0 || i < k {
		t.Fatalf("Instructions at %d, ## Job at %d: Instructions must come after ## Job", i, k)
	}
	if strings.Contains(got.Brief, "model: sonnet") || strings.Contains(got.Brief, "description: from the brain") {
		t.Fatal("the agent's front matter is in the brief")
	}
	// The same Job without the brain's agent has its template's Instructions: researcher has no agents/ file, only agent-templates/grid-researcher.md.
	_, tplBody := frontAndBody(t, "grid-researcher", pluginFile(t, "agent-templates/grid-researcher.md"))
	if bare := saRenderWith(t, "researcher", ""); !strings.Contains(bare.Brief, "### Instructions\n\n"+tplBody+"\n\n") {
		t.Fatal("a researcher without a brain agent got no Instructions from its template")
	}
	// A role with neither file has none.
	if bare := saRenderWith(t, "writer", ""); strings.Contains(bare.Brief, "### Instructions") {
		t.Fatal("a writer with no agent file got an Instructions section")
	}
}

func TestSaBrainAgentReplacesEmbedded(t *testing.T) {
	body := "Build it the brain's way."
	md := saBrainAgent(t, "grid-builder", "builder", body)
	got := saRenderWith(t, "builder", md)
	if !strings.Contains(got.Brief, "### Instructions\n\n"+body+"\n\n") {
		t.Fatal("brief lacks the brain agent's body")
	}
	if embedded := agentFileBody(t, "builder"); strings.Contains(got.Brief, embedded) {
		t.Fatal("brief still carries the body of agents/builder.md")
	}
	if n := strings.Count(got.Brief, "### Instructions\n"); n != 1 {
		t.Fatalf("%d Instructions sections", n)
	}
	// An empty Agent keeps the plugin's own file, as before.
	if !strings.Contains(saRenderWith(t, "builder", "").Brief, "### Instructions\n\n"+agentFileBody(t, "builder")+"\n\n") {
		t.Fatal("without a brain agent the builder lost its own instructions")
	}
	// The instructions are not a source: inputs.json is the same but for the brief's length.
	base := saRenderWith(t, "builder", "")
	got.Inputs.BriefBytes, base.Inputs.BriefBytes = 0, 0
	gj, e := got.Inputs.JSON()
	if e != nil {
		t.Fatal(e)
	}
	bj, e := base.Inputs.JSON()
	if e != nil {
		t.Fatal(e)
	}
	if string(gj) != string(bj) {
		t.Fatalf("inputs.json changed:\n%s\n%s", gj, bj)
	}
}
