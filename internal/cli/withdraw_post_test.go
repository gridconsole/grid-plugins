package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func withdrawalProbe(t *testing.T) (*harness, string, feed.Store, feed.Record, string) {
	t.Helper()
	h, task := feedHarness(t)
	t.Setenv("GRID_AGENT", "builder")
	t.Setenv("GRID_WORKER_ID", "")
	t.Setenv("GRID_ROLE", "")
	t.Setenv("GRID_FEED_TASK", task)
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	q, err := store.Post(feed.Input{Type: "question", Body: map[string]any{"question": "Proceed?", "who": "member", "gate": map[string]any{"id": "Q-1", "kind": "person"}}}, feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{"question_event": "withdrawn", "target": q.ID, "text": "No longer needed"})
	return h, task, store, q, string(body)
}

// S6b8: a withdrawal post is not refused because GRID_AGENT or GRID_WORKER_ID is
// set. The role (member here) and the lease rules still decide. These replace
// TestFeedPostAgentWithdrawalRefused, TestJobPostAgentWithdrawalRefused,
// TestMCPFeedPostAgentWithdrawalRefused and TestAgentWithdrawalPostVariants.
func TestAgentEnvWithdrawalPostSucceeds(t *testing.T) {
	for _, path := range []string{"feed", "mcp"} {
		for _, marker := range []string{"GRID_AGENT", "GRID_WORKER_ID"} {
			t.Run(path+"/"+marker, func(t *testing.T) {
				h, task, store, q, body := withdrawalProbe(t)
				t.Setenv("GRID_AGENT", "")
				t.Setenv(marker, "builder")
				if path == "mcp" {
					line := mcpCall(t, h, "feed_post", `{"type":"control","re":"`+q.ID+`","body":`+body+`}`)
					if strings.Contains(line, `"isError":true`) {
						t.Fatalf("MCP withdrawal under %s: %s", marker, line)
					}
				} else if code := h.run(task, "feed", "post", "--type", "control", "--re", q.ID, "--body", body); code != 0 {
					t.Fatalf("withdrawal under %s: exit=%d stdout=%s stderr=%s", marker, code, h.out, h.errb)
				}
				state, err := store.State()
				if err != nil || state.Questions[q.ID].Open || len(state.NeedsYou) != 0 {
					t.Fatalf("withdrawal did not close the question: %+v %v", state, err)
				}
			})
		}
	}
}

func TestAgentWithdrawalWithTaskFence(t *testing.T) {
	for _, path := range []string{"feed", "job", "mcp", "question"} {
		t.Run(path, func(t *testing.T) {
			h, task, store := orchestratorHarness(t)
			q := questionAdd(t, h, task, "member")
			t.Setenv("GRID_AGENT", "builder")
			t.Setenv("GRID_ROLE", "")
			t.Setenv("GRID_FEED_TASK", task)
			if path != "job" {
				t.Setenv("GRID_LEASE", "task")
			}
			// Job controls keep their existing implicit task lease when the fence is supplied.
			body := `{"question_event":"withdrawn","target":"` + q.Post + `","text":"No longer needed"}`
			before, err := store.Read(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if path == "mcp" {
				line := mcpCall(t, h, "feed_post", `{"type":"control","re":"`+q.Post+`","body":`+body+`}`)
				if strings.Contains(line, `"isError":true`) {
					t.Fatalf("leased MCP withdrawal: %s", line)
				}
			} else {
				args := []string{path, "post", "--type", "control", "--re", q.Post, "--body", body}
				if path == "question" {
					args = []string{"question", "withdraw", q.ID}
				}
				if path == "job" {
					args = append(args, "--job", "J-1")
				}
				if code := h.run(task, args...); code != 0 {
					t.Fatalf("leased withdrawal: %d %s", code, h.errb)
				}
			}
			after, err := store.Read(feed.Filter{})
			if err != nil || len(after.Records) != len(before.Records)+1 {
				t.Fatalf("leased append: %+v %v", after, err)
			}
			state, err := store.State()
			if err != nil || state.Questions[q.Post].Open || len(state.NeedsYou) != 0 {
				t.Fatalf("leased gate: %+v %v", state, err)
			}
		})
	}
}
