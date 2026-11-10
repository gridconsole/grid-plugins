package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

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
