package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

func TestGateSquatCLI(t *testing.T) {
	for _, command := range []string{"feed", "job"} {
		for _, allow := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/override=%t", command, allow), func(t *testing.T) {
				h, path, store := orchestratorHarness(t)
				if code := h.run(path, "job", "next"); code != 0 {
					t.Fatalf("assign: %d %s", code, h.errb)
				}
				state, err := store.State()
				if err != nil {
					t.Fatal(err)
				}
				// Post the waiting Job before any question claims Q-1.
				raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
				if err != nil {
					t.Fatal(err)
				}
				var body map[string]any
				if err := json.Unmarshal(raw, &body); err != nil {
					t.Fatal(err)
				}
				body["id"], body["item"], body["after"] = "J-2", 2, []any{"gate:Q-1"}
				owner := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + feedMachine}
				if _, err := store.Post(feed.Input{Type: "job", Job: "J-2", Body: body}, owner); err != nil {
					t.Fatal(err)
				}
				if allow {
					h.write(filepath.Join(path, "workflow.md"), "---\ngate_post_roles:\n  timer: [agent]\n---\n")
				}
				t.Setenv("GRID_ROLE", "agent")
				t.Setenv("GRID_LEASE", "J-1")
				t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
				t.Setenv("GRID_JOB", "J-1")
				t.Setenv("GRID_ATTEMPT", "1")
				if code := h.run(path, "job", "ack", "J-1"); code != 0 {
					t.Fatalf("ack: %d %s", code, h.errb)
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
				if allow {
					if code != 0 {
						t.Errorf("override: exit %d, stderr %s", code, h.errb)
					}
				} else if code != 3 || !strings.Contains(h.errb.String(), "role agent may not post timer gates") {
					t.Errorf("squat: exit %d, stderr %s", code, h.errb)
				}
				after, err := store.Read(feed.Filter{})
				if err != nil {
					t.Fatal(err)
				}
				want := len(before.Records)
				if allow {
					want++
				}
				if len(after.Records) != want {
					t.Errorf("records: got %d want %d", len(after.Records), want)
				}
				snap, err := orchestrator.Load(store)
				if err != nil {
					t.Fatal(err)
				}
				ready, err := orchestrator.Ready(snap, time.Now())
				if err != nil && (allow || !strings.Contains(err.Error(), "gate gate:Q-1 does not exist")) {
					t.Fatal(err)
				}
				if slices.Contains(ready.Ready, "J-2") != allow {
					t.Errorf("waiting Job readiness: %+v", ready)
				}
			})
		}
	}
}
