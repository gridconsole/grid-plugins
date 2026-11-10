package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A task with no plan.md renders a research Job that was posted with grid job
// post, and grid job show returns the saved brief.
func TestRenderFeedPostedJobWithoutPlan(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	h.write(filepath.Join(path, "intent.md"), "# Intent\n\nUnderstand the request.\n")
	if _, err := os.Stat(filepath.Join(path, "plan.md")); err == nil {
		t.Fatal("fresh task already has plan.md")
	}
	body := firstMinuteJob(t, "J-2", "researcher", []string{})
	body.Body["intent"] = map[string]any{"owner": "intent.md#L1", "item": "intent.md#L1", "goal": "intent.md#L1"}
	body.Body["read"] = []string{}
	raw, err := json.Marshal(body.Body)
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(path, "job", "post", "J-2", "--type", "job", "--body", string(raw)); code != 0 {
		t.Fatalf("job post %d %s", code, h.errb)
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(path, "job", "render", path, "J-2", "--json"); code != 0 {
		t.Fatalf("job render %d %s %s", code, h.out, h.errb)
	}
	if _, err := os.Stat(filepath.Join(path, "jobs", "J-2", "brief.md")); err != nil {
		t.Fatalf("brief.md not saved: %v", err)
	}
	h.out.Reset()
	if code := h.run(path, "job", "show", path, "J-2", "--json"); code != 0 || !strings.Contains(h.out.String(), "Build example feature") {
		t.Fatalf("job show %d %s %s", code, h.out, h.errb)
	}
}
