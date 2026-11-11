package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Record hooks only write history, usage, counts, heartbeats and session start
// and end. Verdict v-15: they never block or deny, even when the write fails.
// Each case runs one record event against a Grid home that cannot be written
// and against input that cannot be read, and expects exit 0 and no decision.
func TestRecordEventsNeverBlockOrDeny(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("a file, not a folder"), 0o600); err != nil {
		t.Fatal(err)
	}
	readOnly := filepath.Join(root, "readonly")
	if err := os.MkdirAll(filepath.Join(readOnly, "projects"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
	cwd := filepath.Join(root, "work")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}

	homes := map[string]string{
		"home under a file":   filepath.Join(blocker, "grid"),
		"read-only home":      readOnly,
		"home does not exist": filepath.Join(root, "missing", "grid"),
	}
	events := []string{
		"SessionStart", "UserPromptSubmit", "SubagentStart", "PostToolUse", "PreCompact",
		"SessionEnd", "PostToolUseFailure",
		"heartbeat", "worker-exit", "history-log", "usage-log", "brain-read", "brain-recall",
	}
	well := func(event string) string {
		b, _ := json.Marshal(map[string]any{
			"hook_event_name": event, "session_id": "s-1", "cwd": cwd, "tool_use_id": "tu-1",
			"transcript_path": filepath.Join(cwd, "t.jsonl"), "agent_type": "grid:grid-builder", "agent_id": "a-1",
			"tool_name": "Write", "tool_input": map[string]any{"file_path": filepath.Join(cwd, "x.md"), "prompt": "/grid:plan T-01"},
			"prompt": "/grid:plan T-01", "source": "startup", "reason": "crash",
		})
		return string(b)
	}
	inputs := map[string]func(event string) string{
		"well formed":    well,
		"not json":       func(string) string { return "{not json" },
		"json null":      func(string) string { return "null" },
		"json array":     func(string) string { return "[1,2,3]" },
		"empty":          func(string) string { return "" },
		"wrong types":    func(string) string { return `{"session_id":7,"cwd":false,"tool_input":"x","tool_name":[]}` },
		"truncated json": func(event string) string { return well(event)[:40] },
	}
	env := map[string]string{
		"GRID_FEED_TASK": "T-01", "GRID_JOB": "J-01", "GRID_WORKER": "w-1", "GRID_TASK": "T-01",
	}

	for homeName, home := range homes {
		for inputName, input := range inputs {
			for _, event := range events {
				t.Run(homeName+"/"+inputName+"/"+event, func(t *testing.T) {
					var out, errOut bytes.Buffer
					code := Run(Options{
						Event: event, Stdin: strings.NewReader(input(event)), Stdout: &out, Stderr: &errOut,
						Resolver: where.Resolver{GridHome: home, Home: root, Machine: "m-1"}, Cwd: cwd,
						Getenv: func(k string) string { return env[k] },
					})
					if code != 0 {
						t.Fatalf("exit %d, want 0; stderr %q", code, errOut.String())
					}
					assertNoDecision(t, out.String())
				})
			}
		}
	}
}

// A Go panic exits 2, which Claude Code reads as a block. A record function
// that panics is logged and the hook still exits 0.
func TestRecordPanicIsLoggedNotRaised(t *testing.T) {
	var errOut bytes.Buffer
	o := Options{Stderr: &errOut}
	o.record("history", func() { panic("boom") })
	if !strings.Contains(errOut.String(), "history") || !strings.Contains(errOut.String(), "boom") {
		t.Fatalf("panic not logged: %q", errOut.String())
	}
	CountHomeAllow(nil, hookio.Input{})
}

// The home-allow counter has no return value to fail with; an unwritable
// counter folder must neither panic nor leave a decision behind.
func TestCountHomeAllowUnwritableFolderIsSilent(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, env := range map[string]map[string]string{
		"plugin data under a file": {"CLAUDE_PLUGIN_DATA": filepath.Join(blocker, "data")},
		"grid home under a file":   {"GRID_HOME": filepath.Join(blocker, "grid")},
		"no folder at all":         {},
	} {
		t.Run(name, func(t *testing.T) {
			CountHomeAllow(env, hookio.Input{HookEventName: "PreToolUse", ToolName: "Read"})
		})
	}
}

// assertNoDecision accepts empty output, or JSON that carries only added
// context: no decision, permissionDecision, behavior, continue:false or block.
func assertNoDecision(t *testing.T, out string) {
	t.Helper()
	if strings.TrimSpace(out) == "" {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			// PreCompact prints plain-text guidance for the summary; text is context, not a decision.
			continue
		}
		for _, k := range []string{"decision", "reason", "continue", "stopReason", "permissionDecision"} {
			if _, ok := v[k]; ok {
				t.Fatalf("record event wrote decision key %q: %s", k, line)
			}
		}
		if spec, ok := v["hookSpecificOutput"].(map[string]any); ok {
			for _, k := range []string{"permissionDecision", "decision"} {
				if _, ok := spec[k]; ok {
					t.Fatalf("record event wrote hookSpecificOutput.%s: %s", k, line)
				}
			}
		}
	}
}
