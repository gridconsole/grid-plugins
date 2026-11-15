package mcp

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A reviewer or a dependent Job reads a person Job's outcome through MCP the
// same way as `grid feed read --job J --type result`: the control that closed
// the person Job, with the answer in body.answer (internal/feed/read.go
// personClosings, end review 0412Z S2).
func TestR10bMCPReadsPersonAnswerAsResult(t *testing.T) {
	s, store, writer := fixture(t)
	grant(t, store, writer)
	if _, err := store.Handover(feed.Input{Type: "handover", Body: map[string]any{"reason": "start"}}, *writer); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var job map[string]any
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatal(err)
	}
	job["id"], job["role"] = "J-1", "Person"
	job["intent"] = map[string]any{"owner": "dana", "item": "i-1", "goal": "Choose the name."}
	post(t, store, *writer, feed.Input{Type: "job", Job: "J-1", Body: job})
	said := post(t, store, *writer, feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": "member answered J-1"}})
	closing := post(t, store, *writer, feed.Input{Type: "control", Re: said.ID, Body: map[string]any{"target": "J-1", "status": "accepted", "reason": "answered by member", "answer": "Call it Lark."}})

	out := result(t, last(t, exchange(t, s, prelude+request("feed_read", `{"job":"J-1","type":"result"}`))))
	if out.IsError {
		t.Fatalf("feed_read: %+v", out)
	}
	var read struct {
		Records []feed.Record `json:"records"`
	}
	if err := json.Unmarshal([]byte(out.Content[0].Text), &read); err != nil {
		t.Fatalf("feed_read text is not the read result: %v: %s", err, out.Content[0].Text)
	}
	if len(read.Records) != 1 {
		t.Fatalf("feed_read --job J-1 --type result wants the one closing record, got %d: %s", len(read.Records), out.Content[0].Text)
	}
	got := read.Records[0]
	if got.ID != closing.ID || got.Type != "control" || got.Body["target"] != "J-1" || got.Body["answer"] != "Call it Lark." {
		t.Fatalf("feed_read returned %+v, want the closing control %s with answer %q", got, closing.ID, "Call it Lark.")
	}
	if n := strings.Count(out.Content[0].Text, "Call it Lark."); n != 1 {
		t.Fatalf("the answer is shown %d times, want once: %s", n, out.Content[0].Text)
	}

	// The CLI reads the Feed with the same filter (internal/cli/feed.go); MCP
	// returns the same records.
	cli, err := store.Read(feed.Filter{Job: "J-1", Type: "result"})
	if err != nil {
		t.Fatal(err)
	}
	if len(cli.Records) != 1 || cli.Records[0].ID != got.ID {
		t.Fatalf("CLI read %+v differs from MCP read %+v", cli.Records, read.Records)
	}
}
