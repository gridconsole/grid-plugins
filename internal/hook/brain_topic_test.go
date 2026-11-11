package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// preToolUse runs one PreToolUse event and returns the stdout and exit code.
func preToolUse(t *testing.T, plugin string, in hookio.Input) (string, int) {
	t.Helper()
	o, _, _ := fixture(t)
	return preToolUseWith(t, o, plugin, in)
}

func preToolUseWith(t *testing.T, o Options, plugin string, in hookio.Input) (string, int) {
	t.Helper()
	in.Cwd = o.Cwd
	stdin, _ := json.Marshal(in)
	out := &bytes.Buffer{}
	o.Event, o.Stdin, o.Stdout = "pretooluse", bytes.NewReader(stdin), out
	o.Getenv = func(k string) string {
		return map[string]string{"CLAUDE_PLUGIN_ROOT": plugin, "CLAUDE_PLUGIN_DATA": filepath.Join(o.Resolver.Home, "plugin-data")}[k]
	}
	code := Run(o)
	return out.String(), code
}

func additionalContext(t *testing.T, out string) string {
	t.Helper()
	var decoded struct {
		Output struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("one JSON object expected: %v: %q", err, out)
	}
	if decoded.Output.Event != "PreToolUse" {
		t.Fatalf("event: %q", out)
	}
	return decoded.Output.Context
}

func fixturePlugin(t *testing.T) string {
	t.Helper()
	plugin := t.TempDir()
	write := func(rel, text string) {
		p := filepath.Join(plugin, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/fixture-skill/SKILL.md", "---\nname: fixture-skill\ndescription: Fixture.\ntopics: fixture-topic\n---\nBody.\n")
	write("skills/plain-skill/SKILL.md", "---\nname: plain-skill\ndescription: Fixture.\n---\nBody.\n")
	write("topics/fixture-topic/TOPIC.md", "---\nname: fixture-topic\nkind: knowledge\nlevel: home\ndescription: Fixture summary.\n---\n# Fixture\n- docs/a.md: why a matters here\n- docs/b.md: why b matters here\n")
	return plugin
}

func TestPreToolUseInjectsTopicFromSkillFrontMatter(t *testing.T) {
	plugin := fixturePlugin(t)
	out, code := preToolUse(t, plugin, hookio.Input{ToolName: "Skill", ToolInput: map[string]any{"skill": "grid:fixture-skill"}})
	if code != 0 {
		t.Fatal(code)
	}
	got := additionalContext(t, out)
	want := "Brain topic fixture-topic: Fixture summary.\n- docs/a.md: why a matters here\n- docs/b.md: why b matters here"
	if got != want {
		t.Fatalf("context %q, want %q", got, want)
	}
	// A skill with no topics adds nothing.
	out, _ = preToolUse(t, plugin, hookio.Input{ToolName: "Skill", ToolInput: map[string]any{"skill": "plain-skill"}})
	if strings.Contains(out, "Brain topic") {
		t.Fatalf("plain skill got topic text: %q", out)
	}
}

func TestPreToolUseInjectsTopicFromToolPathOncePerSession(t *testing.T) {
	plugin := fixturePlugin(t)
	o, _, _ := fixture(t)
	in := hookio.Input{SessionID: "s1", ToolName: "Read", ToolInput: map[string]any{"file_path": "/anywhere/topics/fixture-topic/extra.md"}}
	out, _ := preToolUseWith(t, o, plugin, in)
	if got := additionalContext(t, out); !strings.HasPrefix(got, "Brain topic fixture-topic: Fixture summary.") {
		t.Fatalf("path did not name the topic: %q", out)
	}
	out, _ = preToolUseWith(t, o, plugin, in)
	if strings.Contains(out, "Brain topic") {
		t.Fatalf("second call in the same session injected again: %q", out)
	}
}

func TestShippedSkillsNameShippedTopics(t *testing.T) {
	plugin, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for skill, topic := range map[string]string{"run-job": "delivering", "review-job": "reviewing"} {
		out, _ := preToolUse(t, plugin, hookio.Input{ToolName: "Skill", ToolInput: map[string]any{"skill": "grid:" + skill}})
		got := additionalContext(t, out)
		if !strings.HasPrefix(got, "Brain topic "+topic+": ") || strings.Count(got, "\n") < 2 || len(got) > 1600 {
			t.Fatalf("%s: %q", skill, got)
		}
	}
}
