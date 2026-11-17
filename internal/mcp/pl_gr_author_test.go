package mcp

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// plGrAnswerArgs is a feed_post of an answer for dana under her grant whose body
// names a forged by.
func plGrAnswerArgs(extra string) string {
	return `{"type":"answer","re":"` + postID + `","body":{"answer":"Call it Lark.","for":"dana","by":"dana","grant":"[[grid/people/dana/grants/answers]]"}` + extra + `}`
}

// Row 3 (V22): the author of an mcp post is the session's; a by in the body is
// replaced by the session's id, and an author argument changes nothing.
func TestPlGrMcpIgnoresAnAuthorArgument(t *testing.T) {
	s, store, writer := fixture(t)
	post(t, store, *writer, feed.Input{ID: postID, Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": "Which name?"}})
	*writer = feed.Writer{Author: feed.Author{ID: "agent-7", Role: "worker"}, Worker: "agent-7@" + machineID}

	// An author argument is not the session's author: the post is not written.
	out := result(t, last(t, exchange(t, s, prelude+request("feed_post", plGrAnswerArgs(`,"author":{"id":"dana","role":"member"}`)))))
	if !out.IsError || !strings.Contains(out.Content[0].Text, "unknown argument") {
		t.Fatalf("author argument: %+v", out)
	}

	out = result(t, last(t, exchange(t, s, prelude+request("feed_post", plGrAnswerArgs("")))))
	if out.IsError {
		t.Fatalf("the answer for dana was refused: %+v", out)
	}
	read, err := store.Read(feed.Filter{Type: "answer"})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 1 {
		t.Fatalf("want one answer, got %d", len(read.Records))
	}
	r := read.Records[0].Record
	if r.Author.ID != "agent-7" || r.Author.Role != "worker" || r.Worker != "agent-7@"+machineID {
		t.Fatalf("the answer's author is %+v (worker %s), want the session's agent-7", r.Author, r.Worker)
	}
	if r.Body["by"] != "agent-7" || r.Body["for"] != "dana" {
		t.Fatalf("body by=%v for=%v, want by from the session", r.Body["by"], r.Body["for"])
	}
}
