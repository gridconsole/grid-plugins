package hookio

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestDecode(t *testing.T) {
	for _, tt := range []struct {
		text string
		ok   bool
	}{{`{}`, true}, {`{"cwd":2,"tool_name":"Write"}`, true}, {`null`, false}, {`[]`, false}, {`{`, false}, {`{} {}`, false}} {
		if got := Decode([]byte(tt.text)); (got != nil) != tt.ok {
			t.Errorf("Decode(%s)=%v", tt.text, got)
		}
	}
	if got := Read(strings.NewReader(`{"cwd":"/example"}`)); got == nil || got.Cwd != "/example" {
		t.Fatal(got)
	}
}
func TestReadCap(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	go func() { _, _ = w.Write([]byte(`{"tool_name":"Read"}`)) }()
	start := time.Now()
	got := readWithin(r, 25*time.Millisecond)
	if got == nil || got.ToolName != "Read" {
		t.Fatal(got)
	}
	if time.Since(start) > time.Second {
		t.Fatal("read exceeded cap")
	}
}
func TestDecisions(t *testing.T) {
	for _, tt := range []struct {
		d    Decision
		want string
	}{
		{Decision{}, ""},
		{Deny("a < b & c"), `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"a < b & c"}}`},
		{Allow("ok"), `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow","permissionDecisionReason":"ok"}}`},
		{Ask("check"), `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"ask","permissionDecisionReason":"check"}}`},
		{PermissionAllow("/grid"), `{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"allow","updatedPermissions":[{"type":"addDirectories","directories":["/grid"],"destination":"session"}]}}}`},
	} {
		if got := string(tt.d.Bytes()); got != tt.want {
			t.Errorf("got %s; want %s", got, tt.want)
		}
	}
	if got := Strongest(Allow("a"), Ask("b"), Deny("c"), Deny("d")); got.Reason != "c" {
		t.Fatal(got)
	}
}

func TestDecisionUnicodeSeparators(t *testing.T) {
	for _, reason := range []string{"\u2028\u2029", `\u2028`, `\u2029`, "\\\u2028", `\\u2028`, "<>&\n\t"} {
		data := Deny(reason).Bytes()
		var got struct {
			Output struct {
				Reason string `json:"permissionDecisionReason"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("%q: %s: %v", reason, data, err)
		}
		if got.Output.Reason != reason {
			t.Fatalf("got %q, want %q", got.Output.Reason, reason)
		}
	}
}
