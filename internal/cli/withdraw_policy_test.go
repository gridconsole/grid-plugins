package cli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestWithdrawUnreadablePolicy(t *testing.T) {
	for _, failure := range []string{"permissions", "directory", "io error"} {
		for _, path := range []string{"question", "leased question", "feed", "job", "mcp", "host"} {
			t.Run(failure+"/"+path, func(t *testing.T) {
				h, task, store := orchestratorHarness(t)
				q := questionAdd(t, h, task, "member")
				if path == "question" {
					t.Setenv("GRID_LEASE_EPOCH", "")
				}
				t.Setenv("GRID_FEED_TASK", task)
				if path != "question" && path != "host" {
					t.Setenv("GRID_LEASE", "task")
					t.Setenv("GRID_AGENT", "builder")
				}
				unreadableWorkflow(t, h, task, failure)
				before := withdrawalFiles(t, task)
				body := `{"question_event":"withdrawn","target":"` + q.Post + `","text":"No longer needed"}`
				switch path {
				case "mcp":
					line := mcpCall(t, h, "feed_post", `{"type":"control","re":"`+q.Post+`","body":`+body+`}`)
					if !strings.Contains(line, `"isError":true`) || !strings.Contains(line, `\"code\":1`) || !strings.Contains(line, "cannot read task withdrawal policy") {
						t.Errorf("MCP refusal: %s", line)
					}
				case "host":
					_, err := store.Post(feed.Input{Type: "control", Re: q.Post, Body: map[string]any{"question_event": "withdrawn", "target": q.Post, "text": "No longer needed"}}, feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine})
					if err == nil || exitCode(err) != 1 || !strings.Contains(err.Error(), "cannot read task withdrawal policy") || strings.Contains(err.Error(), "\n") {
						t.Errorf("host refusal: %v", err)
					}
				default:
					args := []string{path, "post", "--type", "control", "--re", q.Post, "--body", body}
					if path == "question" || path == "leased question" {
						args = []string{"question", "withdraw", q.ID}
					} else if path == "job" {
						args = append(args, "--job", "J-1")
					}
					code := h.run(task, args...)
					if code != 1 || h.out.Len() != 0 || strings.Count(h.errb.String(), "\n") != 1 || !strings.Contains(h.errb.String(), "cannot read task withdrawal policy") {
						t.Errorf("refusal: exit=%d stdout=%s stderr=%s", code, h.out, h.errb)
					}
				}
				if after := withdrawalFiles(t, task); !reflect.DeepEqual(before, after) {
					t.Error("refusal changed task files")
				}
			})
		}
	}
}

func TestQuestionWriteUnreadablePolicy(t *testing.T) {
	commands := []struct {
		name      string
		action    string
		withdrawn bool
		empty     bool
	}{
		{name: "answer open", action: "answer"},
		{name: "answer withdrawn", action: "answer", withdrawn: true},
		{name: "add empty", action: "add", empty: true},
		{name: "add withdrawn", action: "add", withdrawn: true},
		{name: "tick open", action: "tick"},
		{name: "tick withdrawn", action: "tick", withdrawn: true},
		{name: "sync withdrawn", action: "sync", withdrawn: true},
	}
	for _, failure := range []string{"directory", "permissions", "io error"} {
		for _, command := range commands {
			t.Run(failure+"/"+command.name, func(t *testing.T) {
				h, task, _ := orchestratorHarness(t)
				t.Setenv("GRID_LEASE_EPOCH", "")
				if !command.empty {
					questionAdd(t, h, task, "member")
					if command.withdrawn {
						if code := h.run(task, "question", "withdraw", "Q-1"); code != 0 {
							t.Fatalf("setup withdrawal: exit=%d", code)
						}
					}
				}
				if command.action != "answer" {
					t.Setenv("GRID_LEASE", "task")
				}
				unreadableWorkflow(t, h, task, failure)
				before := withdrawalFiles(t, task)
				args := []string{"question", command.action, "--json"}
				switch command.action {
				case "answer":
					args = []string{"answer", task, "Q-1", "Ship it", "--json"}
				case "add":
					args = append(args, "Another question?", "--default", "Wait", "--applies", "2099-01-01T00:00:00Z")
				}
				code := h.run(task, args...)
				if code != 1 {
					t.Errorf("write: exit=%d; want exit=1", code)
				}
				if h.out.Len() != 0 {
					t.Error("refusal produced stdout")
				}
				if strings.Count(h.errb.String(), "\n") != 1 || !strings.Contains(h.errb.String(), "cannot read task withdrawal policy") || strings.Contains(h.errb.String(), task) || strings.Contains(h.errb.String(), "warning:") {
					t.Error("want one path-free policy error without fallback warning")
				}
				if after := withdrawalFiles(t, task); !reflect.DeepEqual(before, after) {
					t.Error("refusal changed task files")
				}
			})
		}
	}
}

func withdrawalFiles(t *testing.T, task string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(task, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[path+"/"] = "directory"
			return nil
		}
		if entry.Name() == "workflow.md" {
			return nil
		}
		raw, err := os.ReadFile(path)
		files[path] = string(raw)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestWithdrawPolicyWarningOnce(t *testing.T) {
	h, task := feedHarness(t)
	q := questionAdd(t, h, task, "member")
	h.write(filepath.Join(task, "workflow.md"), "---\ngate_post_roles: [\n---\n")
	stderr, err := os.CreateTemp(t.TempDir(), "stderr-")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	old := os.Stderr
	os.Stderr = stderr
	defer func() { os.Stderr = old }()
	for i := 0; i < 2; i++ {
		code := h.run(task, "question", "withdraw", q.ID)
		os.Stderr = old
		if _, err := stderr.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(stderr)
		if err != nil {
			t.Fatal(err)
		}
		warnings := string(raw) + h.errb.String()
		if code != 0 || strings.Count(warnings, "warning:") != 1 || strings.Count(warnings, "\n") != 1 || !strings.Contains(warnings, "using defaults") {
			t.Fatalf("withdraw: exit=%d warnings=%q", code, warnings)
		}
		if err := stderr.Truncate(0); err != nil {
			t.Fatal(err)
		}
		if _, err := stderr.Seek(0, 0); err != nil {
			t.Fatal(err)
		}
		os.Stderr = stderr
	}
}

func TestAgentWithdrawUnreadablePolicy(t *testing.T) {
	for _, path := range []string{"question", "feed", "job", "mcp"} {
		t.Run(path, func(t *testing.T) {
			h, task, _, q, body := withdrawalProbe(t)
			h.mkdir(filepath.Join(task, "workflow.md"))
			before := withdrawalFiles(t, task)
			if path == "mcp" {
				line := mcpCall(t, h, "feed_post", `{"type":"control","re":"`+q.ID+`","body":`+body+`}`)
				if !strings.Contains(line, `"isError":true`) || !strings.Contains(line, `\"code\":1`) || !strings.Contains(line, "cannot read task withdrawal policy") {
					t.Errorf("MCP refusal: %s", line)
				}
			} else {
				args := []string{path, "post", "--type", "control", "--re", q.ID, "--body", body}
				if path == "question" {
					args = []string{"question", "withdraw", "Q-1"}
				}
				code := h.run(task, args...)
				wantCode, wantText := 1, "cannot read task withdrawal policy"
				if path == "question" {
					wantCode, wantText = 3, "agent_env"
				}
				if code != wantCode || h.out.Len() != 0 || strings.Count(h.errb.String(), "\n") != 1 || !strings.Contains(h.errb.String(), wantText) {
					t.Errorf("refusal: exit=%d stdout=%s stderr=%s", code, h.out, h.errb)
				}
			}
			if after := withdrawalFiles(t, task); !reflect.DeepEqual(before, after) {
				t.Error("refusal changed task files")
			}
		})
	}
}

// Each operation must warn once, including repeat invocations in one process.
func TestReadUnreadablePolicy(t *testing.T) {
	commands := []struct {
		name string
		args []string
		want string
	}{
		{"question list", []string{"question", "list", "--json"}, `"state":"defaulted"`},
		{"feed state", []string{"feed", "state", "--json"}, `"open":false`},
		{"MCP feed_state", []string{"mcp"}, `\"open\":false`},
		{"job tree", []string{"job", "tree", "--json"}, `"id":"J-1"`},
		{"orchestrator tick", []string{"feed", "tick", "--json"}, `"state":"assigned"`},
		{"job post message", []string{"job", "post", "--job", "J-1", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"Still moving"}`, "--json"}, `"type":"post"`},
		{"job post control", []string{"job", "post", "--job", "J-1", "--type", "control", "--body", `{"target":"J-1","status":"ready","reason":"ready"}`, "--json"}, `"type":"control"`},
	}
	for _, failure := range []string{"permissions", "directory", "io error"} {
		for _, command := range commands {
			t.Run(failure+"/"+command.name, func(t *testing.T) {
				h, task, _ := orchestratorHarness(t)
				questionAdd(t, h, task, "member", "--applies", "2000-01-01T00:00:00Z")
				t.Setenv("GRID_FEED_TASK", task)
				t.Setenv("GRID_LEASE", "task")
				unreadableWorkflow(t, h, task, failure)
				for attempt := 0; attempt < 2; attempt++ {
					if command.name == "MCP feed_state" {
						h.env.Stdin = strings.NewReader(mcpHello + `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"feed_state","arguments":{}}}` + "\n")
					}
					code := 0
					warnings := capturePolicyStderr(t, func() { code = h.run(task, command.args...) }) + h.errb.String()
					if code != 0 || strings.Contains(h.out.String(), `"isError":true`) {
						t.Errorf("read: exit=%d isError=%t; want exit=0 isError=false; stderr=%s", code, strings.Contains(h.out.String(), `"isError":true`), warnings)
						return
					}
					if strings.Count(warnings, "warning:") != 1 || strings.Count(warnings, "\n") != 1 || !strings.Contains(warnings, "withdraw: []") || strings.Contains(warnings, task) {
						t.Errorf("want one path-free withdraw: [] warning, got %q", warnings)
					}
					if attempt == 0 && !strings.Contains(h.out.String(), command.want) {
						t.Errorf("missing %s in output: %s", command.want, h.out)
					}
				}
			})
		}
	}
}

func capturePolicyStderr(t *testing.T, run func()) string {
	t.Helper()
	stderr, err := os.CreateTemp(t.TempDir(), "stderr-")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	old := os.Stderr
	os.Stderr = stderr
	defer func() { os.Stderr = old }()
	run()
	if _, err := stderr.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func unreadableWorkflow(t *testing.T, h *harness, task, failure string) {
	t.Helper()
	policy := filepath.Join(task, "workflow.md")
	switch failure {
	case "directory":
		h.mkdir(policy)
	case "permissions":
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("chmod does not deny reads on Windows or as root")
		}
		h.write(policy, "---\ngate_post_roles: {withdraw: [member]}\n---\n")
		if err := os.Chmod(policy, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(policy, 0600) })
	case "io error":
		if runtime.GOOS != "linux" {
			t.Skip("requires Linux process memory reads")
		}
		// Reading unmapped address zero yields a real EIO without a global
		// filesystem hook. The link resolves to the reading process itself.
		if err := os.Symlink(filepath.Join(string(filepath.Separator), "proc", "self", "mem"), policy); err != nil {
			t.Fatal(err)
		}
		if _, err := os.ReadFile(policy); !errors.Is(err, syscall.EIO) {
			t.Skipf("process memory did not provide EIO: %v", err)
		}
	default:
		t.Fatalf("unknown workflow failure %q", failure)
	}
}
