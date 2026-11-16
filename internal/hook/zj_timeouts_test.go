package hook

import "testing"

// Every event but Stop and SubagentStop gets 30 s so a loaded machine does not
// cut a hook off; Stop and SubagentStop keep 60.
func TestGuardHookTimeoutsSurviveLoad(t *testing.T) {
	want := map[string]int{
		"SessionStart": 30, "UserPromptSubmit": 30, "SubagentStart": 30,
		"PreToolUse": 30, "PostToolUse": 30, "PreCompact": 30,
		"Stop": 60, "SubagentStop": 60, "SessionEnd": 30,
		"PostToolUseFailure": 30, "PermissionRequest": 30,
	}
	got := Events()
	if len(got) != len(want) {
		t.Fatalf("%d events, want %d", len(got), len(want))
	}
	for _, e := range got {
		if w, ok := want[e.Name]; !ok || e.Timeout != w {
			t.Errorf("%s timeout %d, want %d", e.Name, e.Timeout, w)
		}
	}
}
