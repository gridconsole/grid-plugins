package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestGateReuseCLI(t *testing.T) {
	for _, command := range []string{"feed", "job"} {
		t.Run(command, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			h.write(filepath.Join(path, "workflow.md"), "---\ngate_post_roles:\n  timer: [agent]\n---\n")
			if code := h.run(path, "job", "next"); code != 0 {
				t.Fatalf("assign: %d %s", code, h.errb)
			}
			state, err := store.State()
			if err != nil {
				t.Fatal(err)
			}
			t.Setenv("GRID_ROLE", "agent")
			t.Setenv("GRID_LEASE", "J-1")
			t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
			t.Setenv("GRID_JOB", "J-1")
			t.Setenv("GRID_ATTEMPT", "1")
			if code := h.run(path, "job", "ack", "J-1"); code != 0 {
				t.Fatalf("ack: %d %s", code, h.errb)
			}
			owner := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + feedMachine}
			if _, err := store.Post(feed.Input{Type: "question", Body: map[string]any{
				"question": "Approve release?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "person"},
			}}, owner); err != nil {
				t.Fatal(err)
			}
			before, err := store.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if command == "feed" {
				t.Setenv("GRID_LEASE", "")
				t.Setenv("GRID_LEASE_EPOCH", "")
			}
			code := h.run(path, command, "post", "--type", "question", "--body", `{"question":"Open now","who":"owner","gate":{"id":"Q-1","kind":"timer","at":"2000-01-01T00:00:00.000Z"}}`)
			if code != 3 || !strings.Contains(h.errb.String(), "gate id Q-1 already exists") {
				t.Errorf("reuse: exit %d, stderr %s", code, h.errb)
			}
			after, err := store.Read(feed.Filter{})
			if err != nil || len(after.Records) != len(before.Records) {
				t.Fatalf("refusal wrote a record: before=%d after=%d err=%v", len(before.Records), len(after.Records), err)
			}
		})
	}
}
