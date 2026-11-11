package orchestrator

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// gqCauseRig is gkRig with a worker-exit that carries code (nil for none): J-1
// and J-2 (done edge to J-1), J-1 assigned and acked, then worker-exit cancelled.
func gqCauseRig(t *testing.T, code any) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", []any{"J-1"})
	f.tick(t)
	f.ack(t)
	j := f.state(t)
	body := map[string]any{"reason": "cancelled"}
	if code != nil {
		body["code"] = code
	}
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
	return f
}

// gqCauseAcceptance ticks twice and returns the acceptance text of the re-plan Job.
func gqCauseAcceptance(t *testing.T, f *fixture) string {
	t.Helper()
	f.tick(t)
	f.tick(t)
	replans, all := gkReplans(t, f)
	body := replans["J-3"]
	if len(replans) != 1 || body == nil {
		t.Fatalf("want one re-plan Job J-3, got %v (Jobs %v)", replans, all)
	}
	rows, _ := body["acceptance"].([]any)
	if len(rows) != 1 {
		t.Fatalf("acceptance = %v", body["acceptance"])
	}
	a1, _ := rows[0].(map[string]any)
	text, _ := a1["text"].(string)
	if intent, _ := body["intent"].(map[string]any); intent["goal"] != "intent.md#request" {
		t.Fatalf("intent.goal = %v, want the request pointer only", intent["goal"])
	}
	return text
}

// Row 1: a string exit code is named in the acceptance text.
func TestGqReplanNamesTheStringCause(t *testing.T) {
	text := gqCauseAcceptance(t, gqCauseRig(t, "launch_failed: no such file"))
	if !strings.Contains(text, "Stopped because: launch_failed: no such file") {
		t.Fatalf("acceptance text lacks the cause: %q", text)
	}
}

// Row 2: no exit code, no sentence.
func TestGqReplanWithoutCauseHasNoSentence(t *testing.T) {
	text := gqCauseAcceptance(t, gqCauseRig(t, nil))
	if strings.Contains(text, "Stopped because") {
		t.Fatalf("acceptance text names a cause that was not given: %q", text)
	}
}

// Row 3: a numeric exit code shows as its digits.
func TestGqReplanNamesTheNumericCause(t *testing.T) {
	text := gqCauseAcceptance(t, gqCauseRig(t, 2))
	if !strings.HasSuffix(text, "Stopped because: 2") {
		t.Fatalf("acceptance text does not end with the numeric cause: %q", text)
	}
}

// The cause is cut to 500 bytes, never inside a rune.
func TestGqCauseIsCutTo500Bytes(t *testing.T) {
	long := strings.Repeat("x", 800)
	body := ReplanJobBody("J-3", "J-1", long, []string{"J-2"})
	text := body["acceptance"].([]map[string]any)[0]["text"].(string)
	i := strings.Index(text, "Stopped because: ")
	if i < 0 || len(text[i+len("Stopped because: "):]) != 500 {
		t.Fatalf("cause not cut to 500 bytes: %d bytes after the label", len(text)-i-len("Stopped because: "))
	}
	// 3-byte runes: 500 is not a multiple of 3, the cut backs up to 498.
	if got := gqCause(strings.Repeat("€", 300)); len(got) != 498 || strings.ContainsRune(got, '�') {
		t.Fatalf("rune cut: %d bytes %q", len(got), got)
	}
	if gqCause("  ") != "" || gqCause("short") != "short" {
		t.Fatal("trim or short cause changed")
	}
}

func TestGqCauseText(t *testing.T) {
	for raw, want := range map[string]string{"": "", `"a b"`: "a b", `2`: "2", `-7`: "-7", `null`: "", `true`: ""} {
		if got := gqCauseText(json.RawMessage(raw)); got != want {
			t.Errorf("gqCauseText(%q) = %q, want %q", raw, got, want)
		}
	}
}
