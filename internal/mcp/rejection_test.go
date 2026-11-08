package mcp

import "testing"

func TestFeedPostInvalidBodyDiagnostic(t *testing.T) {
	s, _, _ := fixture(t)
	got := result(t, last(t, exchange(t, s, prelude+request("feed_post", `{"type":"post","body":{}}`))))
	const want = "$.body: oneOf matched 0 branches: $.body: missing required ref"
	if !got.IsError || len(got.Content) != 1 || got.Content[0].Text != `{"error":"`+want+`"}` {
		t.Fatalf("%+v", got)
	}
}
