package eval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
)

// DefaultTraceLimit is eval_trace_stdout_limit: the most agent stdout kept in the
// trace file. It is separate from outputLimit, which bounds the stored result.
const DefaultTraceLimit int64 = 64 << 20

// traceFile is the trace's name beside the workspace in the attempt folder.
const traceFile = "trace.jsonl"

// ErrNoTrace means the agent's stdout held no line the parser could read. It is
// never an empty trace, which would let a negative check pass.
var ErrNoTrace = errors.New("no trace")

// ToolCall is one tool use by the agent, numbered from zero in call order.
type ToolCall struct {
	Index int            `json:"index"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input,omitempty"`
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
			Type    string `json:"type"`
			Message struct {
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
		for _, item := range items {
			if item.Type == "tool_use" {
				calls = append(calls, ToolCall{Index: len(calls), Name: item.Name, Input: item.Input})
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

// cappedFile keeps the first limit bytes written and reports the rest as dropped.
// It never fails a Write, so a full trace cannot stop the agent.
type cappedFile struct {
	mu        sync.Mutex
	file      *os.File
	limit     int64
	written   int64
	truncated bool
}

func (c *cappedFile) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(p)
	if room := c.limit - c.written; int64(len(p)) > room {
		p, c.truncated = p[:room], true
	}
	if len(p) > 0 {
		if _, err := c.file.Write(p); err != nil {
			c.truncated = true // a lost write must never read as a complete trace
			return n, nil
		}
		c.written += int64(len(p))
	}
	return n, nil
}
