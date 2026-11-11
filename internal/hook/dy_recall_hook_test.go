package hook

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// Verdict v-6: brain entries reach an agent as skills, rules and Job pointers.
// No hook injects recall or topic text.

func recallFixturePlugin(t *testing.T) string {
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
	write("topics/fixture-topic/TOPIC.md", "---\nname: fixture-topic\nkind: knowledge\nlevel: home\ndescription: Fixture summary.\n---\n# Fixture\n- docs/a.md: why a matters here\n")
	return plugin
}

func noRecallText(t *testing.T, what, out, log string, code int) {
	t.Helper()
	if code != 0 || log != "" {
		t.Fatalf("%s: exit %d log %q", what, code, log)
	}
	for _, text := range []string{"Brain recall", "Brain pointers", "Brain topic", "additionalContext"} {
		if strings.Contains(out, text) {
			t.Fatalf("%s: hook output holds %q: %s", what, text, out)
		}
	}
}

func historyText(t *testing.T, f *eventFixture) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A planner dispatch on a task with a brain note writes no recall text and no
// `brain recall` history line, at the Agent call and at the SubagentStart that
// follows it.
func TestPlannerDispatchWritesNoBrainRecall(t *testing.T) {
	f := eventSetup(t)
	acceptedNote(t, f)
	agent := hookio.Input{ToolName: "Agent", ToolUseID: "agent-1", ToolInput: map[string]any{"subagent_type": "grid-planner", "prompt": "Plan the cache configuration task", "description": "plan"}}
	out, log, code := f.run(t, "PreToolUse", agent)
	noRecallText(t, "PreToolUse Agent", out, log, code)
	out, log, code = f.run(t, "SubagentStart", hookio.Input{AgentType: "grid-planner", AgentID: "agent-1"})
	noRecallText(t, "SubagentStart planner", out, log, code)
	out, log, code = f.run(t, "PostToolUse", hookio.Input{ToolName: "Skill", ToolInput: map[string]any{"skill": "grid:plan", "args": "T-01"}})
	noRecallText(t, "PostToolUse plan skill", out, log, code)
	if h := historyText(t, f); strings.Contains(h, "brain recall") {
		t.Fatalf("history holds a brain recall line: %s", h)
	}
}

// Skill front matter topics, knowledge/<topic>/ paths, SubagentStart and
// UserPromptSubmit add no text; the other hooks on those events still run and
// exit 0.
func TestNoTopicOrRecallTextFromAnyHook(t *testing.T) {
	f := eventSetup(t)
	acceptedNote(t, f)
	plugin := recallFixturePlugin(t)
	f.env["CLAUDE_PLUGIN_ROOT"] = plugin

	out, log, code := f.run(t, "PreToolUse", hookio.Input{ToolName: "Skill", ToolInput: map[string]any{"skill": "grid:fixture-skill"}})
	noRecallText(t, "PreToolUse Skill", out, log, code)
	out, log, code = f.run(t, "PreToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, "knowledge", "fixture-topic", "extra.md")}})
	noRecallText(t, "PreToolUse Read knowledge path", out, log, code)

	for _, tc := range []struct {
		event string
		in    hookio.Input
	}{
		{"SubagentStart", hookio.Input{AgentType: "grid:grid-planner", AgentID: "agent-2"}},
		{"SubagentStart", hookio.Input{AgentType: "grid:grid-builder", AgentID: "agent-3"}},
		{"UserPromptSubmit", hookio.Input{Prompt: "/grid:plan"}},
		{"UserPromptSubmit", hookio.Input{Prompt: "/grid:execute"}},
	} {
		out, log, code = f.run(t, tc.event, tc.in)
		noRecallText(t, tc.event+" "+tc.in.AgentType+tc.in.Prompt, out, log, code)
	}
	// The session record is still written for the session.
	if _, err := (session.Store{TaskDir: f.dir}).Activity("session-demo"); err != nil {
		t.Fatal(err)
	}
	if h := historyText(t, f); strings.Contains(h, "brain recall") {
		t.Fatalf("history holds a brain recall line: %s", h)
	}
}

// A settings.json written while the recall switches existed still loads; the
// three names are ignored, not listed, and no hook reads them.
func TestRetiredRecallHookNamesAreIgnored(t *testing.T) {
	f := eventSetup(t)
	path := filepath.Join(f.home, "settings.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"topic-trigger":"block","subagent-start":"block","user-prompt":"block","host-guard":"warn"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := settings.Store{Home: f.home}
	d, err := s.Effective()
	if err != nil {
		t.Fatalf("settings naming the retired recall hooks refused: %v", err)
	}
	levels, err := settings.ReadHookLevels(f.home, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"topic-trigger", "subagent-start", "user-prompt"} {
		if _, ok := d.Hooks[name]; ok {
			t.Fatalf("%s still in the effective hooks: %v", name, d.Hooks)
		}
		if _, ok := levels[name]; ok {
			t.Fatalf("%s still read as a level: %v", name, levels)
		}
		if slices.Contains(settings.HookNames(), name) {
			t.Fatalf("%s still a hook name", name)
		}
		if err := s.SetHook(name, "off"); err == nil {
			t.Fatalf("the command accepted the removed hook %s", name)
		}
	}
	if d.Hooks["host-guard"] != "warn" {
		t.Fatalf("host-guard = %q, want warn", d.Hooks["host-guard"])
	}
	// The hooks themselves run without a settings error.
	for _, event := range []string{"SubagentStart", "UserPromptSubmit"} {
		out, log, code := f.run(t, event, hookio.Input{AgentType: "grid:grid-planner", Prompt: "/grid:plan"})
		noRecallText(t, event, out, log, code)
	}
}
