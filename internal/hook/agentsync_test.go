package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// syncSession wires a plugin root with two templates and a Claude config folder, and runs SessionStart.
func syncSession(t *testing.T) (run func() string, agentsDir string) {
	t.Helper()
	o, out, root := fixture(t)
	plugin := filepath.Join(root, "plugin")
	templates := filepath.Join(plugin, "agent-templates")
	if err := os.MkdirAll(templates, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"grid-builder", "grid-scout"} {
		text := "---\nname: " + name + "\ndescription: " + name + "\nmodel: sonnet\neffort: medium\n---\n\nBody of " + name + ".\n"
		if err := os.WriteFile(filepath.Join(templates, name+".md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	o.Getenv = func(key string) string {
		if key == "CLAUDE_PLUGIN_ROOT" {
			return plugin
		}
		return ""
	}
	return func() string {
		out.Reset()
		o.Stdin = strings.NewReader(`{"source":"startup"}`)
		if code := Run(o); code != 0 {
			t.Fatalf("exit %d", code)
		}
		return contextFrom(t, out)
	}, filepath.Join(root, ".claude", "agents")
}

func TestSessionStartSyncIdempotent(t *testing.T) {
	run, agentsDir := syncSession(t)
	run()
	path := filepath.Join(agentsDir, "grid-builder.md")
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the first SessionStart wrote no agent file: %v", err)
	}
	if !strings.Contains(string(first), "owner: grid") {
		t.Fatalf("the agent file has no Grid marker:\n%s", first)
	}
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{"grid-builder.md", "grid-scout.md"} {
		if err := os.Chtimes(filepath.Join(agentsDir, name), old, old); err != nil {
			t.Fatal(err)
		}
	}
	run()
	for _, name := range []string{"grid-builder.md", "grid-scout.md"} {
		info, err := os.Stat(filepath.Join(agentsDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(old) {
			t.Errorf("%s was rewritten by the second run (mtime %s)", name, info.ModTime())
		}
	}
}

func TestSessionStartNamesBareAgents(t *testing.T) {
	run, _ := syncSession(t)
	context := run()
	if strings.Contains(context, "grid:grid-") {
		t.Errorf("context names plugin-qualified agents, which sync does not write: %s", context)
	}
	for _, name := range []string{"scout", "researcher", "builder", "runner", "reviewer", "designer", "checker", "operator", "planner", "curator", "library"} {
		if !strings.Contains(context, "grid-"+name+" ") {
			t.Errorf("context lacks grid-%s: %s", name, context)
		}
	}
}
