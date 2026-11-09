package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/persongate"
)

const invalidPersonWarning = "grid: warning: invalid person_verbs; ignoring the key and keeping the person-only floor\n"

func TestInvalidPersonVerbsHookProbe(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: 5\n---\n")
	h.env.Getenv = func(key string) string {
		if key == "GRID_AGENT" {
			return "builder"
		}
		return ""
	}
	for _, tool := range []string{"Bash", "Write", "Edit"} {
		t.Run(tool, func(t *testing.T) {
			input, err := json.Marshal(map[string]any{
				"tool_name": tool, "agent_type": "grid-builder",
				"tool_input": map[string]string{
					"command": "grid approve T-01", "file_path": filepath.Join(fixture.Path, "workflow.md"),
					"content": "---\nperson_verbs: 5\n---\n", "old_string": "5", "new_string": "false",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			h.env.Stdin = strings.NewReader(string(input))
			if code := h.run(fixture.Path, "hook", "PreToolUse"); code != 0 {
				t.Fatalf("exit %d, want 0; stdout=%q stderr=%q", code, h.out, h.errb)
			}
			var got struct {
				Output struct {
					HookEventName      string `json:"hookEventName"`
					PermissionDecision string `json:"permissionDecision"`
				} `json:"hookSpecificOutput"`
			}
			if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Output.HookEventName != "PreToolUse" || got.Output.PermissionDecision != "deny" || h.errb.Len() != 0 {
				t.Fatalf("want normal deny JSON and no gate diagnostic; stdout=%s stderr=%s", h.out, h.errb)
			}
		})
	}
}

func TestInvalidPersonVerbsCommands(t *testing.T) {
	for _, policy := range []string{"5", "false", "[task show, 5]", "{approve: 5}", "{approve: [agent]}"} {
		t.Run(policy, func(t *testing.T) {
			h := newHarness(t)
			fixture := taskFixture(t, h)
			h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: "+policy+"\n---\n")
			h.env.Getenv = func(key string) string {
				if key == "GRID_AGENT" {
					return "builder"
				}
				return ""
			}
			if code := h.run(fixture.Path, "task", "show", "T-01"); code != 0 || h.out.Len() == 0 || h.errb.String() != invalidPersonWarning {
				t.Fatalf("task show: exit %d, want 0 with one warning; stdout=%q stderr=%q", code, h.out, h.errb)
			}
			before := fileBytes(t, h.gridHome)
			for _, verb := range persongate.Floor() {
				args := append(strings.Fields(verb), "T-01", "--json")
				code := h.run(fixture.Path, args...)
				var refusal map[string]string
				if err := json.Unmarshal(h.out.Bytes(), &refusal); err != nil || code != 3 || refusal["reason"] != "agent_env" || refusal["error"] != "person_required" {
					t.Errorf("%s: exit %d, want 3 with agent_env; stdout=%q stderr=%q", verb, code, h.out, h.errb)
				}
				if strings.Count(h.errb.String(), invalidPersonWarning) != 1 {
					t.Errorf("%s: want one warning; stderr=%q", verb, h.errb)
				}
			}
			if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
				t.Fatal("refused commands changed files")
			}
		})
	}
}

func TestHookCommandsBypassPersonPolicy(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	h.env.Getenv = func(string) string { return "" }
	h.env.Procs = testProcs("codex")
	for _, policy := range []string{"5", "[hook, hook PreToolUse, hook PermissionRequest, hook SessionStart, hook Stop, hook levels]"} {
		h.write(filepath.Join(fixture.Path, "workflow.md"), "---\nperson_verbs: "+policy+"\n---\n")
		for _, event := range []string{"PreToolUse", "PermissionRequest", "SessionStart", "Stop", "PostToolUse", "unknown", "levels"} {
			h.env.Cwd = fixture.Path
			h.out.Reset()
			h.errb.Reset()
			h.env.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"grid approve T-01"}}`)
			wantCode := runHook([]string{event}, h.env)
			wantOut, wantErr := h.out.String(), h.errb.String()
			h.env.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"grid approve T-01"}}`)
			if code := h.run(fixture.Path, "hook", event); code != wantCode || h.out.String() != wantOut || h.errb.String() != wantErr {
				t.Errorf("policy=%s event=%s: wrapper differs from handler: exit=%d want=%d stdout=%q stderr=%q", policy, event, code, wantCode, h.out, h.errb)
			}
		}
		if code := h.run(fixture.Path, "hook", "level", "plan-guard", "off"); code != 3 {
			t.Errorf("hook level: exit %d, want 3", code)
		}
	}
}
