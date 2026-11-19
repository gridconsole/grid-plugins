package cli

import (
	"strings"
	"testing"
)

func refusalJob(id, text string, write []string) map[string]any {
	body := findingsJob(id, findingsGoodRead)
	body["role"] = "builder"
	body["write"] = write
	body["acceptance"] = []map[string]any{{"id": "A1", "text": text, "traces": "item"}}
	return body
}

func TestPostRefusesLintWarningStillPosts(t *testing.T) {
	h, path, store := findingsTask(t)
	if code := postFindingsJob(t, h, path, refusalJob("J-2", "The panel shows on the live dev UI", []string{"main.go"})); code != 0 {
		t.Fatalf("exit %d, want 0: %s", code, h.errb)
	}
	if !strings.Contains(h.errb.String(), "job.live-ui-row") {
		t.Fatalf("stderr lacks the warning: %s", h.errb)
	}
	if st := storedJob(t, store, "J-2"); st.Type != "job" {
		t.Fatalf("stored: %+v", st)
	}
}
