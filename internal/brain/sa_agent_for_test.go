package brain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// saPutAgent writes knowledge/agents/<name>.md at the home level (project "") or
// under projects/<project>.
func saPutAgent(t *testing.T, home, project, name, text string) {
	t.Helper()
	dir := filepath.Join(home, "knowledge", "agents")
	if project != "" {
		dir = filepath.Join(home, "projects", project, "knowledge", "agents")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// saAgentText is a valid agent file whose body names where it came from.
func saAgentText(body string) string {
	return "---\nname: x\ndescription: d\nmodel: sonnet\neffort: high\n---\n" + body + "\n"
}

func saWant(t *testing.T, s brain.Store, role, wantBody, wantLevel string) {
	t.Helper()
	md, level, ok := s.AgentFor(role)
	if !ok || md != saAgentText(wantBody) || level != wantLevel {
		t.Fatalf("AgentFor(%q) = %q, %q, %v; want body %q at %s", role, md, level, ok, wantBody, wantLevel)
	}
}

func saWantNone(t *testing.T, s brain.Store, role string) {
	t.Helper()
	if md, level, ok := s.AgentFor(role); ok {
		t.Fatalf("AgentFor(%q) = %q, %q, true; want ok false", role, md, level)
	}
}

func TestSaAgentForOrder(t *testing.T) {
	t.Run("project beats home", func(t *testing.T) {
		home := t.TempDir()
		saPutAgent(t, home, "", "grid-researcher", saAgentText("home"))
		saPutAgent(t, home, "example", "grid-researcher", saAgentText("project"))
		s := brain.Store{GridHome: home, Project: "example"}
		saWant(t, s, "researcher", "project", "project")
		// Without a project the store sees the home level only.
		saWant(t, brain.Store{GridHome: home}, "researcher", "home", "home")
	})
	t.Run("grid-role beats role", func(t *testing.T) {
		home := t.TempDir()
		saPutAgent(t, home, "", "researcher", saAgentText("bare"))
		saWant(t, brain.Store{GridHome: home}, "researcher", "bare", "home")
		saPutAgent(t, home, "", "grid-researcher", saAgentText("prefixed"))
		for _, spelling := range []string{"researcher", "grid-researcher", "Researcher", " researcher "} {
			saWant(t, brain.Store{GridHome: home}, spelling, "prefixed", "home")
		}
	})
	t.Run("names come before levels", func(t *testing.T) {
		// A project <role>.md does not beat a home grid-<role>.md: the name order is the first key.
		home := t.TempDir()
		saPutAgent(t, home, "", "grid-researcher", saAgentText("home prefixed"))
		saPutAgent(t, home, "example", "researcher", saAgentText("project bare"))
		saWant(t, brain.Store{GridHome: home, Project: "example"}, "researcher", "home prefixed", "home")
	})
	t.Run("orchestrator finds grid-planner", func(t *testing.T) {
		home := t.TempDir()
		s := brain.Store{GridHome: home}
		saPutAgent(t, home, "", "orchestrator", saAgentText("bare"))
		saWant(t, s, "orchestrator", "bare", "home")
		saPutAgent(t, home, "", "grid-orchestrator", saAgentText("grid-orchestrator"))
		saWant(t, s, "orchestrator", "grid-orchestrator", "home")
		saPutAgent(t, home, "", "grid-planner", saAgentText("planner"))
		for _, spelling := range []string{"orchestrator", "grid-orchestrator", "Orchestrator"} {
			saWant(t, s, spelling, "planner", "home")
		}
		// grid-planner is the orchestrator's catalog name, not a researcher's.
		saWantNone(t, s, "researcher")
	})
	t.Run("a key outside the agent fields skips the file", func(t *testing.T) {
		home := t.TempDir()
		hooked := "---\nname: x\nhooks:\n  PreToolUse: []\n---\nhooked\n"
		saPutAgent(t, home, "example", "grid-researcher", hooked)
		saPutAgent(t, home, "", "grid-researcher", saAgentText("home"))
		s := brain.Store{GridHome: home, Project: "example"}
		saWant(t, s, "researcher", "home", "home")
		// Nothing else to return: ok false.
		other := t.TempDir()
		saPutAgent(t, other, "", "grid-researcher", hooked)
		saWantNone(t, brain.Store{GridHome: other}, "researcher")
		// The search goes on to the next name when no level of this one has a clean file.
		saPutAgent(t, other, "", "researcher", saAgentText("bare"))
		saWant(t, brain.Store{GridHome: other}, "researcher", "bare", "home")
	})
	t.Run("front matter that does not parse skips the file", func(t *testing.T) {
		home := t.TempDir()
		for name, text := range map[string]string{
			"grid-researcher": "no front matter at all\n",
			"researcher":      "---\nname: x\nbody but no closing line\n",
		} {
			saPutAgent(t, home, "", name, text)
		}
		saWantNone(t, brain.Store{GridHome: home}, "researcher")
		saPutAgent(t, home, "", "grid-researcher", "---\nmodel: [unclosed\n---\nx\n")
		saWantNone(t, brain.Store{GridHome: home}, "researcher")
	})
	t.Run("nothing to find", func(t *testing.T) {
		home := t.TempDir()
		s := brain.Store{GridHome: home, Project: "example"}
		saWantNone(t, s, "researcher")
		saPutAgent(t, home, "", "grid-researcher", saAgentText("home"))
		for _, role := range []string{"", "  ", "grid-", "../researcher", "a/b", "person"} {
			saWantNone(t, s, role)
		}
		saWantNone(t, brain.Store{}, "researcher")
	})
	t.Run("line endings come back as newlines", func(t *testing.T) {
		home := t.TempDir()
		saPutAgent(t, home, "", "grid-researcher", "---\r\nname: x\r\n---\r\nbody\r\n")
		md, _, ok := brain.Store{GridHome: home}.AgentFor("researcher")
		if !ok || md != "---\nname: x\n---\nbody\n" {
			t.Fatalf("AgentFor = %q, %v", md, ok)
		}
	})
}
