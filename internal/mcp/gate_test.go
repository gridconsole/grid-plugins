package mcp

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestGateReuseMCP(t *testing.T) {
	s, store, writer := fixture(t)
	post(t, store, *writer, feed.Input{Type: "question", Body: map[string]any{
		"question": "Approve release?", "who": "owner", "gate": map[string]any{"id": "Q-1", "kind": "person"},
	}})
	writer.Author = feed.Author{ID: "agent", Role: "member"}
	writer.Worker = "agent@" + machineID
	out := result(t, last(t, exchange(t, s, prelude+request("feed_post", `{"type":"question","body":{"question":"Open now","who":"owner","gate":{"id":"Q-1","kind":"timer","at":"2000-01-01T00:00:00.000Z"}}}`))))
	if !out.IsError || !strings.Contains(out.Content[0].Text, "gate id Q-1 already exists") {
		t.Errorf("reuse: %+v", out)
	}
	r, err := store.Read(feed.Filter{})
	if err != nil || len(r.Records) != 1 {
		t.Fatalf("refusal wrote a record: %+v %v", r, err)
	}
}
