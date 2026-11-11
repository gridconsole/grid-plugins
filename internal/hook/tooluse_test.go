package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestGuardDispatch(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, tt := range []struct{ name, tool, rel, want string }{
			{"read home", "Read", "projects/example/notes.md", "allow"},
			{"write own task", "Write", "projects/example/tasks/T-01/notes.md", "deny"},
			{"write brain", "Edit", "knowledge/note.md", "deny"},
			{"write feed", "Write", "projects/example/tasks/T-01/feed/record.jsonl", "deny"},
			{"project record", "Write", "projects/example/project.md", "deny"},
			{"ordinary home write", "Write", "notes.md", "allow"},
		} {
			t.Run(event+"/"+tt.name, func(t *testing.T) {
				root := t.TempDir()
				home := filepath.Join(root, "home")
				grid := filepath.Join(home, ".grid")
				cwd := filepath.Join(root, "repo")
				data := filepath.Join(root, "data")
				// only-writer warns by default (verdicts-11nov v-4); these rows pin the deny it gives at block.
				if err := (settings.Store{Home: grid}).SetHook("only-writer", "block"); err != nil {
					t.Fatal(err)
				}
				in := hookio.Input{ToolName: tt.tool, Cwd: cwd, ToolInput: map[string]any{"file_path": filepath.Join(grid, tt.rel), "content": "text"}}
				stdin, _ := json.Marshal(in)
				var out bytes.Buffer
				code := Run(Options{
					Event: event, Stdin: bytes.NewReader(stdin), Stdout: &out, Cwd: cwd,
					Resolver: where.Resolver{GridHome: grid, Home: home},
					Getenv: func(k string) string {
						return map[string]string{"GRID_PLAN_GUARD": "off", "CLAUDE_PLUGIN_DATA": data}[k]
					},
				})
				if code != 0 {
					t.Fatal(code)
				}
				var decoded struct {
					Output struct {
						Event    string `json:"hookEventName"`
						Behavior string `json:"permissionDecision"`
						Decision struct {
							Behavior string `json:"behavior"`
						} `json:"decision"`
					} `json:"hookSpecificOutput"`
				}
				if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
					t.Fatalf("%s: %v", out.Bytes(), err)
				}
				got := decoded.Output.Behavior
				if event == "permissionrequest" {
					got = decoded.Output.Decision.Behavior
					if decoded.Output.Event != "PermissionRequest" {
						t.Fatal(decoded.Output.Event)
					}
				}
				if got != tt.want {
					t.Fatalf("got %s, want %s: %s", got, tt.want, out.Bytes())
				}
				// Only a fallback grant that won arbitration is counted.
				counted, _ := os.ReadFile(filepath.Join(data, "home-allow.jsonl"))
				wantCount := 0
				if tt.want == "allow" {
					wantCount = 1
				}
				if n := strings.Count(string(counted), "\n"); n != wantCount {
					t.Fatalf("fallback count %d, want %d", n, wantCount)
				}
			})
		}
	}
}

// With the default only-writer level, a task-folder write through the dispatcher
// is not denied and the warning goes out as a systemMessage.
func TestGuardDispatchOnlyWriterDefaultWarns(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		t.Run(event, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			grid := filepath.Join(home, ".grid")
			cwd := filepath.Join(root, "repo")
			in := hookio.Input{ToolName: "Write", Cwd: cwd, ToolInput: map[string]any{"file_path": filepath.Join(grid, "projects/example/tasks/T-01/notes.md"), "content": "text"}}
			stdin, _ := json.Marshal(in)
			var out bytes.Buffer
			code := Run(Options{
				Event: event, Stdin: bytes.NewReader(stdin), Stdout: &out, Cwd: cwd,
				Resolver: where.Resolver{GridHome: grid, Home: home},
				Getenv: func(k string) string {
					return map[string]string{"GRID_PLAN_GUARD": "off", "CLAUDE_PLUGIN_DATA": filepath.Join(root, "data")}[k]
				},
			})
			if code != 0 {
				t.Fatal(code)
			}
			got := out.String()
			if strings.Contains(got, `"deny"`) || !strings.Contains(got, `"systemMessage":"only-writer: Task folders and the brain are written only by grid`) {
				t.Fatalf("want a warning and no deny: %s", got)
			}
		})
	}
}

func TestGuardMalformedAndUnknown(t *testing.T) {
	for _, event := range []string{"pretooluse", "permissionrequest", "stop", "subagentstop", "unknown"} {
		var out bytes.Buffer
		if code := Run(Options{Event: event, Stdin: strings.NewReader("{"), Stdout: &out}); code != 0 || out.Len() != 0 {
			t.Fatalf("%s: %d, %s", event, code, out.Bytes())
		}
	}
}

func TestHomeAllowCounter(t *testing.T) {
	root := t.TempDir()
	env := map[string]string{"CLAUDE_PLUGIN_DATA": root}
	in := hookio.Input{HookEventName: "PreToolUse", ToolName: "Read", Cwd: "/private", SessionID: "not-recorded"}
	CountHomeAllow(env, in)
	CountHomeAllow(env, in)
	got, err := os.ReadFile(filepath.Join(root, "home-allow.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Repeat("{\"event\":\"PreToolUse\",\"tool\":\"Read\"}\n", 2)
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

// foldsCase probes whether dir's volume reads two spellings of a name as one
// entry (the macOS default); it never assumes this from the OS.
func foldsCase(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "CaseProbe")
	if err := os.WriteFile(probe, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Stat(filepath.Join(dir, "caseprobe"))
	return err == nil
}

// A builder must not reach the brain or a project record through another
// spelling of the Grid home on a case-folding volume.
func TestCaseAliasesOfTheGridHomeAreDenied(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !foldsCase(t, root) {
		t.Skip("case-sensitive filesystem: another spelling names a different path")
	}
	home := filepath.Join(root, "home")
	grid := filepath.Join(home, ".grid")
	cwd := filepath.Join(root, "repo")
	for _, d := range []string{filepath.Join(grid, "projects", "example", "knowledge"), filepath.Join(grid, "knowledge"), cwd} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(grid, "projects", "example", "project.md"), []byte("---\nname: example\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// only-writer and plan-guard warn by default (verdicts-11nov v-4); these rows pin the deny each gives at block.
	for _, name := range []string{"only-writer", "plan-guard"} {
		if err := (settings.Store{Home: grid}).SetHook(name, "block"); err != nil {
			t.Fatal(err)
		}
	}
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, tt := range []struct{ name, path string }{
			{"project knowledge", filepath.Join(grid, "PROJECTS", "example", "knowledge", "n.md")},
			{"project record", filepath.Join(grid, "PROJECTS", "example", "project.md")},
			{"home knowledge", filepath.Join(home, ".GRID", "knowledge", "n.md")},
		} {
			t.Run(event+"/"+tt.name, func(t *testing.T) {
				in := hookio.Input{AgentType: "grid:grid-builder", ToolName: "Write", Cwd: cwd, ToolInput: map[string]any{"file_path": tt.path, "content": "text"}}
				stdin, _ := json.Marshal(in)
				var out bytes.Buffer
				Run(Options{
					Event: event, Stdin: bytes.NewReader(stdin), Stdout: &out, Cwd: cwd,
					Resolver: where.Resolver{GridHome: grid, Home: home},
					Getenv:   func(string) string { return "" },
				})
				if !strings.Contains(out.String(), `"deny"`) {
					t.Fatalf("not denied: %s", out.Bytes())
				}
			})
		}
	}
}

func TestOversizedEventIsRefusedForGuardedTools(t *testing.T) {
	huge := strings.Repeat("x", hookio.InputLimit)
	named := func(tool string) string {
		return `{"hook_event_name":"PreToolUse","tool_name":"` + tool + `","tool_input":{"file_path":"/repo/a.go","content":"` + huge + `"}}`
	}
	unnamed := `{"tool_input":{"content":"` + huge + `"},"tool_name":"Write"}`
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, tt := range []struct {
			name, stdin string
			deny        bool
		}{
			{"Write", named("Write"), true},
			{"Edit", named("Edit"), true},
			{"Bash", named("Bash"), true},
			{"Agent", named("Agent"), true},
			{"tool named after its input", unnamed, true},
			{"Read", named("Read"), false},
			{"Grep", named("Grep"), false},
		} {
			t.Run(event+"/"+tt.name, func(t *testing.T) {
				var out bytes.Buffer
				Run(Options{Event: event, Stdin: strings.NewReader(tt.stdin), Stdout: &out, Cwd: t.TempDir(), Getenv: func(string) string { return "" }})
				if got := strings.Contains(out.String(), `"deny"`); got != tt.deny {
					t.Fatalf("deny=%v want %v: %.200s", got, tt.deny, out.String())
				}
				if tt.deny {
					if !strings.Contains(out.String(), "larger than") {
						t.Fatalf("no reason given: %.300s", out.String())
					}
					if event == "permissionrequest" && !strings.Contains(out.String(), `"hookEventName":"PermissionRequest"`) {
						t.Fatalf("wrong envelope: %.300s", out.String())
					}
				} else if out.Len() != 0 {
					t.Fatalf("expected silence: %.200s", out.String())
				}
			})
		}
	}
}

func TestEventAtTheLimitIsStillGuarded(t *testing.T) {
	root := t.TempDir()
	grid := filepath.Join(root, "home", ".grid")
	if err := os.MkdirAll(filepath.Join(grid, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	head := `{"tool_name":"Write","tool_input":{"file_path":"` + filepath.Join(grid, "knowledge", "n.md") + `","content":"`
	tail := `"}}`
	stdin := head + strings.Repeat("x", hookio.InputLimit-len(head)-len(tail)) + tail
	if len(stdin) != hookio.InputLimit {
		t.Fatal(len(stdin))
	}
	var out bytes.Buffer
	Run(Options{Event: "pretooluse", Stdin: strings.NewReader(stdin), Stdout: &out, Cwd: root,
		Resolver: where.Resolver{GridHome: grid, Home: filepath.Join(root, "home")}, Getenv: func(string) string { return "" }})
	if !strings.Contains(out.String(), "written only by grid") {
		t.Fatalf("the brain write was not judged by the guards: %.300s", out.String())
	}
}
