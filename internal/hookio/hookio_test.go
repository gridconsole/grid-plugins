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
	if got, large := Read(strings.NewReader(`{"cwd":"/example"}`)); got == nil || got.Cwd != "/example" || large {
		t.Fatal(got, large)
	}
}
func TestReadCap(t *testing.T) {
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	go func() { _, _ = w.Write([]byte(`{"tool_name":"Read"}`)) }()
	start := time.Now()
	got, large := read(r, 25*time.Millisecond, InputLimit)
	if got == nil || got.ToolName != "Read" || large {
		t.Fatal(got, large)
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

func TestReadByteLimit(t *testing.T) {
	pad := func(n int) string { return strings.Repeat("x", n) }
	body := func(first string, n int) string {
		return `{"hook_event_name":"PreToolUse",` + first + `"tool_input":{"content":"` + pad(n) + `"}}`
	}
	for _, tt := range []struct {
		name, text string
		limit      int
		tool       string
		large      bool
	}{
		{"under", body(`"tool_name":"Write",`, 10), 1000, "Write", false},
		{"exactly full", body(`"tool_name":"Write",`, 10), len(body(`"tool_name":"Write",`, 10)), "Write", false},
		{"one byte over", body(`"tool_name":"Write",`, 10), len(body(`"tool_name":"Write",`, 10)) - 1, "Write", true},
		{"far over, name first", body(`"tool_name":"Edit",`, 5000), 1000, "Edit", true},
		{"far over, name after the input", `{"tool_input":{"content":"` + pad(5000) + `"},"tool_name":"Write"}`, 1000, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, large := read(strings.NewReader(tt.text), time.Second, tt.limit)
			if large != tt.large || got == nil || got.ToolName != tt.tool {
				t.Fatalf("large=%v input=%+v", large, got)
			}
			if large && len(got.ToolInput) != 0 {
				t.Fatalf("an unreadable event kept a tool input: %+v", got.ToolInput)
			}
		})
	}
}

// A reader that never ends must not be drained past the limit.
func TestReadStopsAtLimit(t *testing.T) {
	var served int
	r := readerFunc(func(p []byte) (int, error) {
		for i := range p {
			p[i] = ' '
		}
		served += len(p)
		return len(p), nil
	})
	if _, large := read(r, time.Second, 100); !large {
		t.Fatal("not reported as too large")
	}
	if served > 100+4096 {
		t.Fatalf("read %d bytes past a 100 byte limit", served)
	}
}

type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }
