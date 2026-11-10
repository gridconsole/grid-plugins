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

func assertWithdrawalUnchanged(t *testing.T, store feed.Store, q feed.Record) {
	t.Helper()
	read, err := store.Read(feed.Filter{})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("refusal wrote a record: %+v %v", read, err)
	}
	state, err := store.State()
	if err != nil || !state.Questions[q.ID].Open || len(state.NeedsYou) != 1 {
		t.Fatalf("refusal closed gate: %+v %v", state, err)
	}
}

func TestFeedPostAgentWithdrawalRefused(t *testing.T) {
	h, task, store, q, body := withdrawalProbe(t)
	code := h.run(task, "feed", "post", "--type", "control", "--re", q.ID, "--body", body)
	if code != 3 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "agent shell requires a task lease fence to withdraw") {
		t.Fatalf("withdrawal: exit=%d stdout=%s stderr=%s", code, h.out, h.errb)
	}
	assertWithdrawalUnchanged(t, store, q)
}

func TestJobPostAgentWithdrawalRefused(t *testing.T) {
	h, task, store, q, body := withdrawalProbe(t)
	code := h.run(task, "job", "post", "--type", "control", "--re", q.ID, "--body", body)
	if code != 3 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "agent shell requires a task lease fence to withdraw") {
		t.Fatalf("withdrawal: exit=%d stdout=%s stderr=%s", code, h.out, h.errb)
	}
	assertWithdrawalUnchanged(t, store, q)
}

func TestMCPFeedPostAgentWithdrawalRefused(t *testing.T) {
	h, _, store, q, body := withdrawalProbe(t)
	line := mcpCall(t, h, "feed_post", `{"type":"control","re":"`+q.ID+`","body":`+body+`}`)
	if !strings.Contains(line, `"isError":true`) || !strings.Contains(line, `\"code\":3`) || !strings.Contains(line, "agent shell requires a task lease fence to withdraw") {
		t.Fatalf("withdrawal: %s", line)
	}
	assertWithdrawalUnchanged(t, store, q)
}

func TestAgentWithdrawalPostVariants(t *testing.T) {
	for _, path := range []string{"feed", "job", "mcp"} {
		for _, tc := range []struct {
			name, key, value, lease, epoch string
			stdin                          bool
		}{
			{name: "worker marker", key: "GRID_WORKER", value: "unrelated-worker"},
			{name: "worker id", key: "GRID_WORKER_ID", value: "builder"},
			{name: "agent role", key: "GRID_ROLE", value: "agent"},
			{name: "lease without epoch", key: "GRID_AGENT", value: "builder", lease: "task"},
			{name: "zero fence", key: "GRID_AGENT", value: "builder", lease: "task", epoch: "0.0"},
			{name: "epoch without lease", key: "GRID_AGENT", value: "builder", epoch: "0.1"},
			{name: "stdin body", key: "GRID_AGENT", value: "builder", stdin: true},
		} {
			if path == "job" && tc.name == "epoch without lease" {
				continue
			} // Job controls infer the task lease.
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				h, task, store, q, body := withdrawalProbe(t)
				t.Setenv("GRID_AGENT", "")
				t.Setenv(tc.key, tc.value)
				t.Setenv("GRID_LEASE", tc.lease)
				t.Setenv("GRID_LEASE_EPOCH", tc.epoch)
				if path == "mcp" {
					line := mcpCall(t, h, "feed_post", `{"type":"control","re":"`+q.ID+`","body":`+body+`}`)
					if !strings.Contains(line, `"isError":true`) || !strings.Contains(line, `\"code\":3`) {
						t.Fatalf("MCP refusal: %s", line)
					}
				} else {
					if tc.stdin {
						h.env.Stdin = strings.NewReader(body)
						body = "-"
					}
					if code := h.run(task, path, "post", "--type", "control", "--re", q.ID, "--body", body); code != 3 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "agent shell requires a task lease fence") {
						t.Fatalf("refusal: %d %s %s", code, h.out, h.errb)
					}
				}
				assertWithdrawalUnchanged(t, store, q)
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
