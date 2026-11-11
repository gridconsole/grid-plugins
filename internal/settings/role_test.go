package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

// Row 4: a role with no catalog agent has no agent, whatever its spelling.
func TestAgentForRole(t *testing.T) {
	for role, want := range map[string]string{
		"builder": "grid-builder", "grid-builder": "grid-builder", "Builder": "grid-builder",
		"checker": "grid-checker", "researcher": "grid-researcher", "reviewer": "grid-reviewer",
		"curator": "grid-curator", "orchestrator": "grid-planner", "grid-orchestrator": "grid-planner",
	} {
		if got, ok := AgentForRole(role); !ok || got != want {
			t.Errorf("AgentForRole(%q) = %q, %v; want %q", role, got, ok, want)
		}
	}
	for _, role := range []string{"person", "Person", "", "no-such-role", "grid-", "grid-person"} {
		if got, ok := AgentForRole(role); ok {
			t.Errorf("AgentForRole(%q) = %q, true; want false", role, got)
		}
	}
}

// ModelFor lays the personal values over the catalog, and a role with no catalog
// agent answers two empty strings without reading the settings file at all.
func TestModelForLayersAndNoAgent(t *testing.T) {
	home := t.TempDir()
	st := Store{Home: home}
	if m, e, err := st.ModelFor("builder"); err != nil || m != "sonnet" || e != "medium" {
		t.Fatalf("catalog builder = %q/%q, %v", m, e, err)
	}
	if err := st.SetAgent("grid-builder", Agent{Model: "opus"}); err != nil {
		t.Fatal(err)
	}
	if m, e, err := st.ModelFor("grid-builder"); err != nil || m != "opus" || e != "medium" {
		t.Fatalf("personal builder = %q/%q, %v; want opus/medium", m, e, err)
	}
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if m, e, err := st.ModelFor("person"); err != nil || m != "" || e != "" {
		t.Fatalf("role without agent = %q/%q, %v; want empty and no error", m, e, err)
	}
	if _, _, err := st.ModelFor("builder"); err == nil || !strings.Contains(err.Error(), "settings.json") {
		t.Fatalf("unparsable settings: err = %v; want one naming settings.json", err)
	}
}

// frontMatter reads the model and effort keys of an agent file's front matter.
func frontMatter(t *testing.T, text string) (model, effort string) {
	t.Helper()
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		t.Fatalf("no front matter: %q", text[:min(len(text), 40)])
	}
	for _, l := range lines[1:] {
		if l == "---" {
			return model, effort
		}
		if v, ok := strings.CutPrefix(l, "model:"); ok {
			model = strings.TrimSpace(v)
		}
		if v, ok := strings.CutPrefix(l, "effort:"); ok {
			effort = strings.TrimSpace(v)
		}
	}
	t.Fatal("front matter has no closing ---")
	return
}

// Row 8: the model and effort an agents/<role>.md file names are the ones a Job of
// that role runs on, so a person reading the file and the worker agree.
func TestAgentFilesMatchTheCatalog(t *testing.T) {
	for _, role := range []string{"builder", "reviewer", "curator", "orchestrator"} {
		text, ok := gridplugins.AgentFor(role)
		if !ok {
			t.Fatalf("no agents/%s.md", role)
		}
		fileModel, fileEffort := frontMatter(t, text)
		model, effort, err := Store{Home: t.TempDir()}.ModelFor(role)
		if err != nil {
			t.Fatal(err)
		}
		if fileModel != model || fileEffort != effort {
			t.Errorf("agents/%s.md says %s/%s, the catalog says %s/%s", role, fileModel, fileEffort, model, effort)
		}
	}
}
