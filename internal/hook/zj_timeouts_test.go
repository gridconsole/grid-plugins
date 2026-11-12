package hook

import "testing"

// The guard events get 30 s so a loaded machine does not time out and let
// Claude Code skip the hook; every other event keeps its earlier value.
func TestGuardHookTimeoutsSurviveLoad(t *testing.T) {
	want := map[string]int{
		"SessionStart": 10, "UserPromptSubmit": 30, "SubagentStart": 10,
		"PreToolUse": 30, "PostToolUse": 10, "PreCompact": 10,
		"Stop": 60, "SubagentStop": 60, "SessionEnd": 10,
		"PostToolUseFailure": 10, "PermissionRequest": 30,
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
