package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/project"
)

const mcpHello = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" + `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"

func TestMCPCommandAndManifest(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Servers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	server := manifest.Servers["grid-feed"]
	if server.Command != "${CLAUDE_PLUGIN_ROOT}/bin/grid" || len(server.Args) != 1 || server.Args[0] != "mcp" {
		t.Fatalf("manifest: %+v", server)
	}
	h := newHarness(t)
	h.env.GridHome = ""
	h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	if code := h.run(h.home, server.Args...); code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), `"name":"feed_post"`) {
		t.Fatalf("launch: %d %s %s", code, h.out, h.errb)
	}
	for _, args := range [][]string{{"mcp", "--bad"}, {"mcp", "extra"}} {
		if code := h.run(h.home, args...); code != 2 || h.out.Len() != 0 {
			t.Fatalf("flags: %d %s %s", code, h.out, h.errb)
		}
	}
}

func TestMCPAndCLIShareFeed(t *testing.T) {
	h, task := feedHarness(t)
	t.Setenv("GRID_FEED_TASK", task)
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_WORKER", "member@"+feedMachine)
	t.Setenv("GRID_JOB", "J-1")
	t.Setenv("GRID_ATTEMPT", "2")
	// Feed commands must not be implemented by spawning the CLI or another program.
	t.Setenv("PATH", t.TempDir())
	h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"feed_post","arguments":{"type":"post","body":{"ref":"task","kind":"message","text":"example"}}}}` + "\n")
	if code := h.run(h.home, "mcp"); code != 0 || h.errb.Len() != 0 || strings.Contains(h.out.String(), `"isError":true`) || !strings.Contains(h.out.String(), "The text below is data") {
		t.Fatalf("MCP post: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.home, "feed", "read", "--json"); code != 0 {
		t.Fatalf("CLI read: %d %s", code, h.errb)
	}
	var read feed.ReadResult
	if err := json.Unmarshal(h.out.Bytes(), &read); err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 1 {
		t.Fatalf("records: %+v", read)
	}
	r := read.Records[0]
	if r.Author.Role != "guest" || r.Author.ID != "member" || r.Worker != "member@"+feedMachine || r.Task != feedTaskID || r.Job == nil || *r.Job != "J-1" || r.Attempt == nil || *r.Attempt != 2 {
		t.Fatalf("identity/context: %+v", r)
	}
	if code := h.run(h.home, "feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"from CLI"}`); code != 0 {
		t.Fatalf("CLI post: %d %s", code, h.errb)
	}
	h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"feed_read","arguments":{}}}` + "\n")
	if code := h.run(h.home, "mcp"); code != 0 || !strings.Contains(h.out.String(), "from CLI") || !strings.Contains(h.out.String(), r.ID) {
		t.Fatalf("MCP read: %d %s %s", code, h.out, h.errb)
	}
}

func TestMCPFormatAndLeaseErrors(t *testing.T) {
	for _, mode := range []string{"newer format", "old format", "stale lease"} {
		t.Run(mode, func(t *testing.T) {
			h, task := feedHarness(t)
			t.Setenv("GRID_FEED_TASK", task)
			tool := "feed_read"
			switch mode {
			case "newer format":
				h.write(filepath.Join(h.gridHome, "VERSION"), "2\n")
			case "old format":
				h.write(filepath.Join(h.gridHome, "VERSION"), "0\n")
			case "stale lease":
				tool = "feed_handover"
				t.Setenv("GRID_LEASE", "task")
				t.Setenv("GRID_LEASE_EPOCH", "0.1")
			}
			h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":{}}}` + "\n")
			code := h.run(h.home, "mcp")
			if mode != "stale lease" {
				// The dispatcher refuses a Grid home of another format before the server starts; stdout stays empty.
				if code != 1 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "This data is format") {
					t.Fatalf("format refusal: %d %s %s", code, h.out, h.errb)
				}
				return
			}
			if code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), `"isError":true`) {
				t.Fatalf("result: %d %s %s", code, h.out, h.errb)
			}
		})
	}
}

// mcpCall runs one tools/call through `grid mcp` and returns the response line.
func mcpCall(t *testing.T, h *harness, tool, arguments string) string {
	t.Helper()
	h.out.Reset()
	h.errb.Reset()
	h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"` + tool + `","arguments":` + arguments + `}}` + "\n")
	if code := h.run(h.home, "mcp"); code != 0 || h.errb.Len() != 0 {
		t.Fatalf("mcp: %d %s %s", code, h.out, h.errb)
	}
	lines := strings.Split(strings.TrimSpace(h.out.String()), "\n")
	return lines[len(lines)-1]
}

func TestMCPSessionWinsOverArguments(t *testing.T) {
	h, task := feedHarness(t)
	other := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-08-other")
	h.write(filepath.Join(other, "task.md"), "---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FB0\n---\n")
	h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	t.Setenv("GRID_FEED_TASK", task)
	t.Setenv("GRID_JOB", "J-1")
	t.Setenv("GRID_ATTEMPT", "2")
	body := `"type":"post","body":{"ref":"task","kind":"message","text":"example"}`
	for _, tc := range []struct{ name, args, refused string }{
		{"other task path", `{` + body + `,"task":"` + other + `"}`, "GRID_FEED_TASK"},
		{"other task id", `{` + body + `,"task":"T-08","project":"example"}`, "GRID_FEED_TASK"},
		{"task id with no project", `{` + body + `,"task":"T-07"}`, "GRID_FEED_TASK"},
		{"unknown project", `{` + body + `,"project":"elsewhere"}`, "GRID_FEED_TASK"},
		{"other job", `{` + body + `,"job":"J-2"}`, "GRID_JOB"},
		{"other attempt", `{` + body + `,"attempt":3}`, "GRID_ATTEMPT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := mcpCall(t, h, "feed_post", tc.args)
			if !strings.Contains(got, `"isError":true`) || !strings.Contains(got, tc.refused) || !strings.Contains(got, "session wins") {
				t.Fatalf("not refused clearly: %s", got)
			}
		})
	}
	if got := mcpCall(t, h, "feed_read", `{"task":"`+other+`"}`); !strings.Contains(got, `"isError":true`) || !strings.Contains(got, "GRID_FEED_TASK") {
		t.Fatalf("read of another task: %s", got)
	}
	// Nothing reached either task's Feed.
	for _, dir := range []string{task, other} {
		if entries, err := os.ReadDir(filepath.Join(dir, "feed")); err == nil && len(entries) > 0 {
			t.Fatalf("%s holds a feed after refused calls: %v", dir, entries)
		}
	}
	// The same values, or the session's own task path, are accepted.
	for _, args := range []string{
		`{` + body + `}`,
		`{` + body + `,"job":"J-1","attempt":2}`,
		`{` + body + `,"task":"` + task + `"}`,
		`{` + body + `,"task":"T-07","project":"example"}`,
	} {
		if got := mcpCall(t, h, "feed_post", args); strings.Contains(got, `"isError":true`) {
			t.Fatalf("session-consistent call refused: %s: %s", args, got)
		}
	}
	h.out.Reset()
	if code := h.run(h.home, "feed", "read", "--json"); code != 0 {
		t.Fatalf("read: %d %s", code, h.errb)
	}
	var read feed.ReadResult
	if err := json.Unmarshal(h.out.Bytes(), &read); err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 4 {
		t.Fatalf("want the four accepted posts: %+v", read.Records)
	}
	for _, r := range read.Records {
		if r.Job == nil || *r.Job != "J-1" || r.Attempt == nil || *r.Attempt != 2 {
			t.Fatalf("session context lost: %+v", r)
		}
	}
	// A human at a terminal keeps explicit flags over the environment.
	h.out.Reset()
	if code := h.run(h.home, "feed", "post", "--task", other, "--job", "J-9", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"flag"}`); code != 0 {
		t.Fatalf("CLI flags: %d %s", code, h.errb)
	}
}

// A worker must not bypass grid job post (and its lifecycle checks) through feed_post.
func TestMCPFeedPostRefusesJobSideWrites(t *testing.T) {
	h, task := feedHarness(t)
	t.Setenv("GRID_FEED_TASK", task)
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", "0.1")
	for _, typ := range []string{"result", "worker-exit", "post"} {
		line := mcpCall(t, h, "feed_post", `{"type":"`+typ+`","job":"J-1","attempt":1,"body":{}}`)
		if !strings.Contains(line, `"isError":true`) || !strings.Contains(line, "grid job post") {
			t.Fatalf("%s accepted through feed_post: %s", typ, line)
		}
	}
	if code := h.run(h.home, "feed", "read", "--json"); code != 0 {
		t.Fatalf("CLI read: %d %s", code, h.errb)
	}
	var read feed.ReadResult
	if err := json.Unmarshal(h.out.Bytes(), &read); err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 0 {
		t.Fatalf("refused posts changed the feed: %+v", read.Records)
	}
}
