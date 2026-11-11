package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const shippedTemplates = "../../agent-templates"

// frontMatter returns the key: value lines between the first two --- lines.
func frontMatter(t *testing.T, text string) map[string]string {
	t.Helper()
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 3 || lines[0] != "---" {
		t.Fatalf("no front matter")
	}
	fm := map[string]string{}
	for _, l := range lines[1:] {
		if l == "---" {
			return fm
		}
		if k, v, ok := strings.Cut(l, ":"); ok && !strings.HasPrefix(l, " ") {
			fm[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	t.Fatalf("front matter is not closed")
	return nil
}

func shippedTemplateFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(shippedTemplates, "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no shipped templates in %s: %v", shippedTemplates, err)
	}
	return files
}

// Every shipped template names itself, says when it triggers, and picks a model and its tools.
func TestShippedTemplatesFrontMatter(t *testing.T) {
	for _, f := range shippedTemplateFiles(t) {
		stem := strings.TrimSuffix(filepath.Base(f), ".md")
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fm := frontMatter(t, string(b))
		if fm["name"] != stem {
			t.Errorf("%s: name %q, want %q", stem, fm["name"], stem)
		}
		if fm["description"] == "" {
			t.Errorf("%s: no description", stem)
		}
		switch fm["model"] {
		case "haiku", "sonnet", "opus":
		default:
			t.Errorf("%s: model %q is not haiku, sonnet or opus", stem, fm["model"])
		}
		if fm["tools"] == "" {
			t.Errorf("%s: no tools line", stem)
		}
	}
}

// SessionStart in a scratch HOME writes every shipped template, not only a sample.
func TestSessionStartSyncsEveryShippedTemplate(t *testing.T) {
	o, out, root := fixture(t)
	plugin, err := filepath.Abs(filepath.Join(shippedTemplates, ".."))
	if err != nil {
		t.Fatal(err)
	}
	o.Getenv = func(key string) string {
		if key == "CLAUDE_PLUGIN_ROOT" {
			return plugin
		}
		return ""
	}
	out.Reset()
	o.Stdin = strings.NewReader(`{"source":"startup"}`)
	if code := Run(o); code != 0 {
		t.Fatalf("exit %d", code)
	}
	agents := filepath.Join(root, ".claude", "agents")
	for _, f := range shippedTemplateFiles(t) {
		name := filepath.Base(f)
		b, err := os.ReadFile(filepath.Join(agents, name))
		if err != nil {
			t.Errorf("SessionStart did not write %s: %v", name, err)
			continue
		}
		if !strings.Contains(string(b), "owner: grid") {
			t.Errorf("%s has no Grid marker", name)
		}
	}
}
