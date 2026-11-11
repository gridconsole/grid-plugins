package hookio

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestReadNothingToRead(t *testing.T) {
	if in, tooLarge := read(nil, time.Second, 10); in != nil || tooLarge {
		t.Fatalf("read(nil) = %v, %v, want nil, false", in, tooLarge)
	}
	if in, tooLarge := read(failingReader{errors.New("boom")}, time.Second, 10); in != nil || tooLarge {
		t.Fatalf("read of a failing reader = %v, %v, want nil, false", in, tooLarge)
	}
}

// The input is cut at byte 60. cwd and agent_type end at byte 40, before the
// cut; the cut falls inside the value of "x" in tool_input, which never
// completes, so prefixFields stops there and keeps the two scalars read before.
func TestReadCutInputKeepsCwdAndAgentType(t *testing.T) {
	text := `{"cwd":"/w","agent_type":"grid-builder","tool_input":{"x":"` + strings.Repeat("a", 100) + `"}}`
	if !strings.HasPrefix(text, `{"cwd":"/w","agent_type":"grid-builder",`) || len(`{"cwd":"/w","agent_type":"grid-builder",`) != 40 {
		t.Fatal("test input no longer puts the two fields in the first 40 bytes")
	}
	in, tooLarge := read(strings.NewReader(text), time.Second, 60)
	if !tooLarge || in == nil {
		t.Fatalf("read = %v, %v, want an Input and tooLarge true", in, tooLarge)
	}
	if in.Cwd != "/w" || in.AgentType != "grid-builder" {
		t.Fatalf("Cwd, AgentType = %q, %q, want %q, %q", in.Cwd, in.AgentType, "/w", "grid-builder")
	}
	if in.ToolInput != nil {
		t.Fatalf("ToolInput = %v, want nil for a cut input", in.ToolInput)
	}
}

func TestDecisionContextOnly(t *testing.T) {
	got := Decision{Context: "c"}.Bytes()
	var out map[string]any
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("Bytes() = %q: %v", got, err)
	}
	want := map[string]any{"hookSpecificOutput": map[string]any{"hookEventName": "PreToolUse", "additionalContext": "c"}}
	if gotJSON, _ := json.Marshal(out); string(gotJSON) != `{"hookSpecificOutput":{"additionalContext":"c","hookEventName":"PreToolUse"}}` {
		t.Fatalf("Bytes() = %s, want %v", got, want)
	}
	if _, has := out["systemMessage"]; has {
		t.Fatalf("Bytes() = %s, want no systemMessage", got)
	}
}

func TestDecisionWrite(t *testing.T) {
	var buf bytes.Buffer
	if err := (Decision{}).Write(&buf); err != nil || buf.Len() != 0 {
		t.Fatalf("empty Decision Write = %v, wrote %q, want nil and nothing", err, buf.String())
	}
	d := Deny("no")
	buf.Reset()
	if err := d.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), d.Bytes()) || buf.Len() == 0 {
		t.Fatalf("Write wrote %q, want exactly Bytes() %q", buf.Bytes(), d.Bytes())
	}
	closed := errors.New("closed")
	if err := d.Write(failingWriter{closed}); err != closed {
		t.Fatalf("Write to a failing writer = %v, want %v", err, closed)
	}
	// An empty Decision never touches the writer, so a failing one is not hit.
	if err := (Decision{}).Write(failingWriter{closed}); err != nil {
		t.Fatalf("empty Decision Write to a failing writer = %v, want nil", err)
	}
}
