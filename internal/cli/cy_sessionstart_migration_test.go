package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// cySessAnswer is the SessionStart answer shape (internal/hook/hook.go).
type cySessAnswer struct {
	HookSpecificOutput struct {
		HookEventName     string `json:"hookEventName"`
		AdditionalContext string `json:"additionalContext"`
	} `json:"hookSpecificOutput"`
}

func TestCySessSessionStartSaysMigrationOpen(t *testing.T) {
	for _, event := range []string{"sessionstart", "SessionStart"} {
		t.Run(event, func(t *testing.T) {
			h := vrPendHarness(t, true)
			h.env.Stdin = strings.NewReader(`{"cwd":"` + h.tmp + `"}`)
			code := h.run(h.tmp, "hook", event)
			t.Logf("grid hook %s: exit %d, stdout %q, stderr %q", event, code, h.out, h.errb)
			if code != 0 {
				t.Errorf("exit %d, want 0", code)
			}
			if h.errb.Len() != 0 {
				t.Errorf("stderr %q, want empty", h.errb)
			}
			var got cySessAnswer
			dec := json.NewDecoder(strings.NewReader(h.out.String()))
			dec.DisallowUnknownFields()
			if err := dec.Decode(&got); err != nil {
				t.Fatalf("stdout %q is not one JSON object: %v", h.out, err)
			}
			if dec.More() {
				t.Errorf("stdout %q holds more than one JSON value", h.out)
			}
			if got.HookSpecificOutput.HookEventName != "SessionStart" {
				t.Errorf("hookEventName %q, want SessionStart", got.HookSpecificOutput.HookEventName)
			}
			if got.HookSpecificOutput.AdditionalContext != vrPendWant {
				t.Errorf("additionalContext %q, want %q", got.HookSpecificOutput.AdditionalContext, vrPendWant)
			}
			vrPendHomeAbsent(t, h, "hook "+event)
		})
	}
}

func TestCySessOtherEventsStaySilent(t *testing.T) {
	for _, event := range []string{"stop", "pretooluse", "posttooluse"} {
		t.Run(event, func(t *testing.T) {
			h := vrPendHarness(t, true)
			h.env.Stdin = strings.NewReader(`{"cwd":"` + h.tmp + `"}`)
			code := h.run(h.tmp, "hook", event)
			if code != 0 || h.out.Len() != 0 || h.errb.Len() != 0 {
				t.Errorf("grid hook %s: exit %d, stdout %q, stderr %q; want 0 and nothing printed", event, code, h.out, h.errb)
			}
			vrPendHomeAbsent(t, h, "hook "+event)
		})
	}
}
