// Package hookio implements Claude Code's hook input and decision wire format.
package hookio

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"
)

// Input contains shared event fields. ToolInput keeps unknown and ill-typed
// values so one malformed optional field does not discard other guard evidence.
type Input struct {
	Source                string            `json:"source"`
	Prompt                string            `json:"prompt"`
	Reason                string            `json:"reason"`
	ToolResponse          map[string]any    `json:"tool_response"`
	Args                  []any             `json:"args"`
	SessionID             string            `json:"session_id"`
	TranscriptPath        string            `json:"transcript_path"`
	Cwd                   string            `json:"cwd"`
	PermissionMode        string            `json:"permission_mode"`
	HookEventName         string            `json:"hook_event_name"`
	ToolName              string            `json:"tool_name"`
	ToolInput             map[string]any    `json:"tool_input"`
	ToolUseID             string            `json:"tool_use_id"`
	AgentID               string            `json:"agent_id"`
	AgentType             string            `json:"agent_type"`
	StopHookActive        bool              `json:"stop_hook_active"`
	LastAssistantMessage  string            `json:"last_assistant_message"`
	AgentTranscriptPath   string            `json:"agent_transcript_path"`
	PermissionSuggestions []json.RawMessage `json:"permission_suggestions"`
}
type PreToolUse Input
type PermissionRequest Input
type Stop Input
type SubagentStop Input

// Decode accepts only JSON objects, just like readInput in lib.mjs.
func Decode(data []byte) *Input {
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil || raw == nil {
		return nil
	}
	// Decode each field separately: JavaScript's str ignores non-string values.
	var in Input
	target, _ := json.Marshal(in)
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(target, &fields)
	for k := range fields {
		if v, ok := raw[k]; ok {
			one, _ := json.Marshal(map[string]json.RawMessage{k: v})
			_ = json.Unmarshal(one, &in)
		}
	}
	return &in
}

const (
	InputTimeout = 3 * time.Second
	// InputLimit is the most stdin bytes a hook reads. Claude Code's tool input
	// for an ordinary edit is far smaller; a larger event is refused, not guessed.
	InputLimit = 1 << 20
)

// Read waits at most three seconds and reads at most InputLimit bytes, then
// parses what it has. If stdin held more than the limit, tooLarge is true and
// the Input holds only the top-level fields found before the limit (usually the
// tool name and cwd), never a tool input: the caller decides what to do with an
// event it could not read. If a non-closable Reader blocks forever, its single
// read goroutine remains blocked; hook processes exit after one call. Owned
// pipes should be closed by the caller.
func Read(r io.Reader) (in *Input, tooLarge bool) { return read(r, InputTimeout, InputLimit) }
func read(r io.Reader, cap time.Duration, limit int) (*Input, bool) {
	timer := time.NewTimer(cap)
	defer timer.Stop()
	return readUntil(r, timer.C, limit)
}

// readUntil is read with the cap as a channel: it parses what stdin gave once
// stdin ends or after fires, whichever comes first. A test drives it with its
// own channel instead of racing a timer against the scheduler.
func readUntil(r io.Reader, after <-chan time.Time, limit int) (*Input, bool) {
	if r == nil {
		return nil, false
	}
	var mu sync.Mutex
	var buf bytes.Buffer
	done := make(chan bool, 1)
	go func() {
		// One byte past the limit tells "exactly full" from "too large".
		lr := io.LimitReader(r, int64(limit)+1)
		b := make([]byte, 4096)
		for {
			n, err := lr.Read(b)
			mu.Lock()
			_, _ = buf.Write(b[:n])
			mu.Unlock()
			if err != nil {
				done <- err == io.EOF
				return
			}
		}
	}()
	select {
	case ok := <-done:
		if !ok {
			return nil, false
		}
	case <-after:
	}
	mu.Lock()
	data := bytes.Clone(buf.Bytes())
	mu.Unlock()
	if len(data) > limit {
		return prefixFields(data[:limit]), true
	}
	return Decode(data), false
}

// prefixFields reads the scalar top-level fields of a JSON object cut off at
// some point. It stops at the first value that does not complete.
func prefixFields(data []byte) *Input {
	in := &Input{}
	dec := json.NewDecoder(bytes.NewReader(data))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return in
	}
	for dec.More() {
		t, err := dec.Token()
		key, ok := t.(string)
		if err != nil || !ok {
			break
		}
		var raw json.RawMessage
		if dec.Decode(&raw) != nil {
			break
		}
		var text string
		if json.Unmarshal(raw, &text) != nil {
			continue
		}
		switch key {
		case "tool_name":
			in.ToolName = text
		case "cwd":
			in.Cwd = text
		case "hook_event_name":
			in.HookEventName = text
		case "agent_type":
			in.AgentType = text
		}
	}
	return in
}

// Decision's zero value is silence (defer to normal permission rules).
type Decision struct {
	UpdatedInput map[string]any
	Interrupt    bool
	Event        string
	Behavior     string
	Reason       string
	Directories  []string
	Warnings     []string
	// Context is text a PreToolUse hook adds for the agent; it is not a verdict.
	Context string
}

func Allow(reason string) Decision {
	return Decision{Event: "PreToolUse", Behavior: "allow", Reason: reason}
}
func Deny(reason string) Decision {
	return Decision{Event: "PreToolUse", Behavior: "deny", Reason: reason}
}
func Ask(reason string) Decision {
	return Decision{Event: "PreToolUse", Behavior: "ask", Reason: reason}
}
func PermissionAllow(home string) Decision {
	d := Decision{Event: "PermissionRequest", Behavior: "allow"}
	if home != "" {
		d.Directories = []string{home}
	}
	return d
}

// Bytes emits JSON.stringify-compatible compact JSON without a trailing newline.
func (d Decision) Bytes() []byte {
	if d.Behavior == "" && len(d.Warnings) == 0 && d.Context == "" {
		return nil
	}
	var out any
	if d.Behavior == "" {
		fields := map[string]any{}
		if len(d.Warnings) > 0 {
			fields["systemMessage"] = strings.Join(d.Warnings, "\n")
		}
		if d.Context != "" {
			fields["hookSpecificOutput"] = map[string]string{"hookEventName": "PreToolUse", "additionalContext": d.Context}
		}
		out = fields
	} else if d.Event == "PermissionRequest" {
		type permission struct {
			Type        string   `json:"type"`
			Directories []string `json:"directories"`
			Destination string   `json:"destination"`
		}
		type decision struct {
			UpdatedInput       map[string]any `json:"updatedInput,omitempty"`
			Interrupt          bool           `json:"interrupt,omitempty"`
			Behavior           string         `json:"behavior"`
			Message            string         `json:"message,omitempty"`
			UpdatedPermissions []permission   `json:"updatedPermissions,omitempty"`
		}
		p := decision{Behavior: d.Behavior, Message: d.Reason, UpdatedInput: d.UpdatedInput, Interrupt: d.Interrupt}
		if len(d.Directories) > 0 {
			p.UpdatedPermissions = []permission{{"addDirectories", d.Directories, "session"}}
		}
		out = struct {
			Output any `json:"hookSpecificOutput"`
		}{struct {
			Event    string   `json:"hookEventName"`
			Decision decision `json:"decision"`
		}{d.Event, p}}
	} else {
		out = struct {
			Output any `json:"hookSpecificOutput"`
		}{struct {
			Event    string `json:"hookEventName"`
			Behavior string `json:"permissionDecision"`
			Reason   string `json:"permissionDecisionReason"`
			Context  string `json:"additionalContext,omitempty"`
		}{d.Event, d.Behavior, d.Reason, d.Context}}
	}
	if d.Behavior != "" && len(d.Warnings) > 0 {
		data, _ := json.Marshal(out)
		var fields map[string]any
		_ = json.Unmarshal(data, &fields)
		fields["systemMessage"] = strings.Join(d.Warnings, "\n")
		out = fields
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if enc.Encode(out) != nil {
		return nil
	}
	data := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	// JSON.stringify leaves these valid Unicode separators literal. Walk escape
	// sequences so a literal backslash followed by "u2028" remains escaped.
	outBytes := make([]byte, 0, len(data))
	for i := 0; i < len(data); {
		if data[i] == '\\' && i+1 < len(data) {
			if bytes.HasPrefix(data[i:], []byte(`\u2028`)) {
				outBytes = append(outBytes, []byte("\u2028")...)
				i += 6
				continue
			}
			if bytes.HasPrefix(data[i:], []byte(`\u2029`)) {
				outBytes = append(outBytes, []byte("\u2029")...)
				i += 6
				continue
			}
			outBytes = append(outBytes, data[i:i+2]...)
			i += 2
			continue
		}
		outBytes = append(outBytes, data[i])
		i++
	}
	return outBytes
}
func (d Decision) Write(w io.Writer) error {
	b := d.Bytes()
	if len(b) == 0 {
		return nil
	}
	_, err := w.Write(b)
	return err
}

// Strongest applies deny > ask > allow, preserving the first reason at a tie.
func Strongest(ds ...Decision) Decision {
	rank := map[string]int{"allow": 1, "ask": 2, "deny": 3}
	var best Decision
	var warnings []string
	for _, d := range ds {
		warnings = append(warnings, d.Warnings...)
		if rank[d.Behavior] > rank[best.Behavior] {
			best = d
		}
	}
	best.Warnings = warnings
	return best
}
