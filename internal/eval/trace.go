package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// DefaultTraceLimit is eval_trace_stdout_limit: the most agent stdout kept in
// memory for the trace checks. It is separate from outputLimit, which bounds the stored result.
const DefaultTraceLimit int64 = 64 << 20

// ErrNoTrace means the agent's stdout held no line the parser could read. It is
// never an empty trace, which would let a negative check pass.
var ErrNoTrace = errors.New("no trace")

// ToolCall is one tool use by the agent, numbered from zero in call order.
type ToolCall struct {
	Index int            `json:"index"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input,omitempty"`
	// Parent is the tool-use id of the call that started the subagent making
	// this call; empty for the run's own calls.
	Parent string `json:"parent,omitempty"`
}

// TraceParser turns the agent's stdout into its tool calls. An agent that does
// not print claude-stream-json supplies its own through Runner.TraceParser.
type TraceParser func([]byte) ([]ToolCall, error)

// DefaultTraceParser reads eval_trace_format "claude-stream-json": one JSON object
// per line, tool calls being tool_use items in the content of assistant messages.
var DefaultTraceParser TraceParser = parseStreamJSON

func parseStreamJSON(data []byte) ([]ToolCall, error) {
	calls := []ToolCall{}
	parsed := false
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var event struct {
			Type            string  `json:"type"`
			ParentToolUseID *string `json:"parent_tool_use_id"`
			Message         struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		parsed = true
		if event.Type != "assistant" {
			continue
		}
		var items []struct {
			Type  string         `json:"type"`
			Name  string         `json:"name"`
			Input map[string]any `json:"input"`
		}
		// The content may be a plain string; only an array holds tool uses.
		if json.Unmarshal(event.Message.Content, &items) != nil {
			continue
		}
		parent := ""
		if event.ParentToolUseID != nil {
			parent = *event.ParentToolUseID
		}
		for _, item := range items {
			if item.Type == "tool_use" {
				calls = append(calls, ToolCall{Index: len(calls), Name: item.Name, Input: item.Input, Parent: parent})
			}
		}
	}
	if !parsed {
		return nil, ErrNoTrace
	}
	return calls, nil
}

// ToolMatch selects tool calls by exact name and, optionally, by substrings of
// string input fields (all must match).
type ToolMatch struct {
	Tool          string         `json:"tool"`
	InputContains map[string]any `json:"input_contains,omitempty"`
}

func (m ToolMatch) matches(call ToolCall) bool {
	if call.Name != m.Tool {
		return false
	}
	for field, want := range m.InputContains {
		got, ok := call.Input[field].(string)
		wantText, _ := want.(string)
		if !ok || !strings.Contains(got, wantText) {
			return false
		}
	}
	return true
}

func (m ToolMatch) validate() error {
	if strings.TrimSpace(m.Tool) == "" {
		return fmt.Errorf("tool must not be empty")
	}
	for field, want := range m.InputContains {
		if text, ok := want.(string); !ok || text == "" || field == "" {
			return fmt.Errorf("input_contains %q must map a field to a nonempty string", field)
		}
	}
	return nil
}

// topLevel keeps the run's own calls and numbers them again from zero.
func topLevel(calls []ToolCall) []ToolCall {
	out := make([]ToolCall, 0, len(calls))
	for _, call := range calls {
		if call.Parent == "" {
			call.Index = len(out)
			out = append(out, call)
		}
	}
	return out
}

func (m ToolMatch) first(calls []ToolCall) int {
	for _, call := range calls {
		if m.matches(call) {
			return call.Index
		}
	}
	return -1
}

// traceInput is what a trace check reads: the agent's captured stdout, whether it
// was cut at the trace limit, and the parser. It is empty for a run without an agent.
type traceInput struct {
	data      []byte
	truncated bool
	parser    TraceParser
}

func evaluateTrace(r CheckResult, c Check, t traceInput) CheckResult {
	if t.truncated {
		r.Reason = "trace truncated"
		return r
	}
	parse := t.parser
	if parse == nil {
		parse = DefaultTraceParser
	}
	calls, err := parse(t.data)
	if err != nil {
		r.Reason = err.Error()
		return r
	}
	if c.TopLevel {
		calls = topLevel(calls)
	}
	switch c.Type {
	case "tool_used":
		m := ToolMatch{Tool: c.Tool, InputContains: c.InputContains}
		count, first := 0, -1
		for _, call := range calls {
			if m.matches(call) {
				if count == 0 {
					first = call.Index
				}
				count++
			}
		}
		if c.Absent {
			mark(&r, count == 0)
			r.Reason = fmt.Sprintf("%s called %d times", c.Tool, count)
			return r
		}
		need := c.CountMin
		if need == 0 {
			need = 1
		}
		mark(&r, count >= need)
		r.Reason = fmt.Sprintf("%s called %d times, need %d", c.Tool, count, need)
		if count > 0 {
			r.Reason += fmt.Sprintf(" (first at call %d)", first)
		}
	case "tool_order":
		before, after := c.Before.first(calls), c.After.first(calls)
		switch {
		case before < 0:
			mark(&r, false)
			r.Reason = "no call matches before"
		case after < 0:
			mark(&r, false)
			r.Reason = "no call matches after"
		default:
			mark(&r, before < after)
			r.Reason = fmt.Sprintf("before at call %d, after at call %d", before, after)
		}
	}
	return r
}

// cappedBuffer keeps the first limit bytes written in memory and reports the
// rest as dropped. It never fails a Write, so a full trace cannot stop the agent.
type cappedBuffer struct {
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int64
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(p)
	if room := c.limit - int64(c.buf.Len()); int64(len(p)) > room {
		p, c.truncated = p[:room], true
	}
	c.buf.Write(p)
	return n, nil
}

func (c *cappedBuffer) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Bytes()
}
