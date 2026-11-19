package cli

import (
	"github.com/gridconsole/grid-plugins/internal/feed"
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

func TestPostRefusesLintAfterMissing(t *testing.T) {
	h, path, store := findingsTask(t)
	if code := postFindingsJob(t, h, path, refusalJob("J-2", "Builds on the change of J-7", []string{"main.go"})); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, h.errb)
	}
	if !strings.Contains(h.errb.String(), "job.after-missing") {
		t.Fatalf("stderr lacks the finding: %s", h.errb)
	}
	if res, err := store.ReadOnly(feed.Filter{Type: "job", Job: "J-2"}); err != nil || len(res.Records) != 0 {
		t.Fatalf("a refused Job was written: %v %+v", err, res.Records)
	}
}

func TestPostRefusesLintRowOutsideWrite(t *testing.T) {
	h, path, _ := findingsTask(t)
	if code := postFindingsJob(t, h, path, refusalJob("J-2", "internal/other/x.go holds it", []string{"main.go"})); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, h.errb)
	}
	if !strings.Contains(h.errb.String(), "job.row-outside-write") {
		t.Fatalf("stderr lacks the finding: %s", h.errb)
	}
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
