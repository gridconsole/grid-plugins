// Package mcp serves the Feed over newline-delimited JSON-RPC on stdio.
// It has no process or environment access; its caller supplies trusted context.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// ProtocolVersion is the newest protocol implemented by this server. The two
// earlier versions used by the old plugin share this subset of the protocol.
const ProtocolVersion = "2025-06-18"
const MaxMessage = 8 << 20

// Server calls the Feed directly. Resolve and Session must use trusted process
// context, never identity supplied by tools/call. They run lazily: discovery and
// feed_types work without a Grid home, task, or writer identity. Getenv answers
// the one environment read the server makes (CLAUDE_PLUGIN_ROOT, for the
// stale-plugin notice); nil reads the process.
type Server struct {
	Version string
	Getenv  func(string) string
	Resolve func(task, project string) (feed.Store, error)
	Session func(feed.Input) (feed.Input, feed.Writer, error)
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func failure(id json.RawMessage, code int, message string) *response {
	if id == nil {
		id = json.RawMessage("null")
	}
	return &response{JSONRPC: "2.0", ID: id, Error: &rpcError{code, message}}
}
func success(id json.RawMessage, result any) *response {
	return &response{JSONRPC: "2.0", ID: id, Result: result}
}

type connection struct{ initialized, ready bool }

// Serve handles one connection until EOF. Only JSON-RPC responses go to out.
// A bounded scanner prevents an unterminated input from consuming unbounded
// memory. Transport errors (including oversized lines) end the connection.
func (s Server) Serve(in io.Reader, out io.Writer) error {
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), MaxMessage+1)
	enc := json.NewEncoder(out)
	c := connection{}
	for scan.Scan() {
		line := bytes.TrimSpace(scan.Bytes())
		if len(line) == 0 {
			continue
		}
		if len(line) > MaxMessage {
			return fmt.Errorf("MCP message exceeds %d bytes", MaxMessage)
		}
		if reply := s.handle(line, &c); reply != nil {
			if err := enc.Encode(reply); err != nil {
				return err
			}
		}
	}
	return scan.Err()
}

var integerID = regexp.MustCompile(`^-?[0-9]+$`)

func validID(id json.RawMessage) bool {
	if integerID.Match(id) {
		return true
	}
	var str string
	return len(id) > 0 && id[0] == '"' && json.Unmarshal(id, &str) == nil
}

func (s Server) handle(line []byte, c *connection) *response {
	if !utf8.Valid(line) || !json.Valid(line) {
		return failure(nil, -32700, "Parse error")
	}
	var msg map[string]json.RawMessage
	if json.Unmarshal(line, &msg) != nil || msg == nil {
		return failure(nil, -32600, "Invalid request")
	}
	id, hasID := msg["id"]
	if hasID && !validID(id) {
		return failure(nil, -32600, "Invalid request id")
	}
	var method, version string
	if json.Unmarshal(msg["jsonrpc"], &version) != nil || version != "2.0" || json.Unmarshal(msg["method"], &method) != nil || method == "" || msg["result"] != nil || msg["error"] != nil {
		return failure(id, -32600, "Invalid request")
	}
	// Notifications never execute tools or produce responses.
	if !hasID {
		if method == "notifications/initialized" && c.initialized {
			c.ready = true
		}
		return nil
	}
	params := msg["params"]
	if params == nil {
		params = json.RawMessage("{}")
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(params, &obj) != nil || obj == nil {
		return failure(id, -32602, "Params must be an object")
	}
	switch method {
	case "ping":
		return success(id, struct{}{})
	case "initialize":
		if c.initialized {
			return failure(id, -32600, "Already initialized")
		}
		var requested string
		if json.Unmarshal(obj["protocolVersion"], &requested) != nil || requested == "" {
			return failure(id, -32602, "protocolVersion is required")
		}
		negotiated := ProtocolVersion
		switch requested {
		case ProtocolVersion, "2025-03-26", "2024-11-05":
			negotiated = requested
		}
		c.initialized = true
		return success(id, map[string]any{
			"protocolVersion": negotiated,
			"capabilities":    map[string]any{"tools": struct{}{}},
			"serverInfo":      map[string]string{"name": "grid-feed", "version": s.Version},
		})
	case "tools/list", "tools/call":
		if !c.ready {
			return failure(id, -32000, "Initialize and send notifications/initialized first")
		}
		if method == "tools/list" {
			if cursor, ok := obj["cursor"]; ok && string(cursor) != `""` {
				return failure(id, -32602, "Invalid cursor")
			}
			return success(id, map[string]any{"tools": Tools()})
		}
		var name string
		if json.Unmarshal(obj["name"], &name) != nil {
			return failure(id, -32602, "Tool name is required")
		}
		var tool *Tool
		for _, candidate := range Tools() {
			if candidate.Name == name {
				tool = &candidate
				break
			}
		}
		if tool == nil {
			return failure(id, -32602, "Unknown tool")
		}
		args := obj["arguments"]
		if args == nil {
			args = json.RawMessage("{}")
		}
		result, err := s.call(*tool, args)
		if err != nil {
			return success(id, s.withStaleNotice(toolError(err)))
		}
		return success(id, s.withStaleNotice(result))
	default:
		return failure(id, -32601, "Method not found")
	}
}
