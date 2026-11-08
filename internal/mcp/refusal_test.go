package mcp_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mcp"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func TestFeedPostJobSideRefusal(t *testing.T) {
	testgit.Isolate(t)
	for _, key := range []string{"GRID_LEASE", "GRID_LEASE_EPOCH", "GRID_WAKE", "GRID_WORKER", "GRID_FEED_TASK", "GRID_JOB", "GRID_ATTEMPT"} {
		t.Setenv(key, "")
	}
	t.Setenv("GRID_MEMBER", "member")
	t.Setenv("GRID_ROLE", "member")
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, "grid")
	task := filepath.Join(home, "projects", "example", "tasks", "T-01-example")
	if err := os.MkdirAll(task, 0o700); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]string{
		filepath.Join(home, "VERSION"): "1\n",
		filepath.Join(home, "machine"): "01ARZ3NDEKTSV4RRFFQ69G5FAW\n",
		filepath.Join(task, "task.md"): "---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n---\n",
	} {
		if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var stdout, stderr bytes.Buffer
	env := cli.Env{Home: root, GridHome: home, Cwd: task, Ceiling: root, Stdout: &stdout, Stderr: &stderr}
	if code := cli.Run([]string{"feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"seed"}`}, env); code != 0 {
		t.Fatalf("seed: exit %d: %s", code, &stderr)
	}
	feedBytes := func() map[string]string {
		t.Helper()
		dir := filepath.Join(task, "feed")
		files := map[string]string{}
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			data, err := os.ReadFile(path)
			files[path] = string(data)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return files
	}
	before := feedBytes()
	// Exercise the real CLI session callback, which owns refuseJobSide.
	const prelude = `{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n"
	for _, tc := range []struct{ kind, message string }{
		{"result", "Job-side writes require grid job post or grid job result"},
		{"post", "$.body: oneOf matched 0 branches: $.body: missing required ref"},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			env.Stdin = strings.NewReader(prelude + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"feed_post","arguments":{"type":"` + tc.kind + `","body":{}}}}` + "\n")
			if code := cli.Run([]string{"mcp"}, env); code != 0 || stderr.Len() != 0 {
				t.Fatalf("MCP exit %d: %s", code, &stderr)
			}
			lines := bytes.Split(bytes.TrimSpace(stdout.Bytes()), []byte{'\n'})
			var response struct {
				ID     int             `json:"id"`
				Error  json.RawMessage `json:"error"`
				Result mcp.Result      `json:"result"`
			}
			if err := json.Unmarshal(lines[len(lines)-1], &response); err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(map[string]string{"error": tc.message})
			if response.ID != 1 || response.Error != nil || !response.Result.IsError || len(response.Result.Content) != 1 || response.Result.Content[0].Type != "text" || response.Result.Content[0].Text != string(want) {
				t.Fatalf("tool rejection: %+v", response)
			}
			if !reflect.DeepEqual(before, feedBytes()) {
				t.Fatal("MCP refusal changed Feed bytes")
			}
		})
	}
}
