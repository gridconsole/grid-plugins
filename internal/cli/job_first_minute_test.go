package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func firstMinuteJob(t *testing.T, id, role string, after []string) feed.Input {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"], body["role"], body["after"] = id, role, after
	return feed.Input{Type: "job", Job: id, Body: body}
}

// A fresh task has no plan.md and no approval. A research Job and a plan Job
// post anyway, and grid job next offers the research Job first.
func TestResearchAndPlanJobsPostBeforeAnyPlan(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	for _, name := range []string{"plan.md", "approval.json"} {
		if _, err := os.Stat(filepath.Join(path, name)); err == nil {
			t.Fatalf("fresh task already has %s", name)
		}
	}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	w.Fence = 1 // the harness takes the task first: epoch 0.1
	if code := h.run(path, "job", "cancel", "J-1", "--reason", "replaced by research"); code != 0 {
		t.Fatalf("cancel %d %s", code, h.errb)
	}
	if _, err := store.Post(firstMinuteJob(t, "J-2", "researcher", []string{}), w); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(firstMinuteJob(t, "J-3", "planner", []string{"J-2"}), w); err != nil {
		t.Fatal(err)
	}
	h.out.Reset()
	if code := h.run(path, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"J-2"`) {
		t.Fatalf("next did not offer the research Job: %d %s %s", code, h.out, h.errb)
	}
}

// A small task is one Job on the Feed, the same path as any other size.
func TestSmallTaskIsOneJobOnTheFeed(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	b, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(path, "task.md"), strings.Replace(string(b), "---\n", "---\nsize: small\n", 1))
	state, err := store.State()
	if err != nil || len(state.Jobs) != 1 || state.Jobs["J-1"].Status == "" {
		t.Fatalf("one Job expected on the Feed: %+v %v", state.Jobs, err)
	}
	if code := h.run(path, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"J-1"`) {
		t.Fatalf("next on a small task: %d %s %s", code, h.out, h.errb)
	}
}
