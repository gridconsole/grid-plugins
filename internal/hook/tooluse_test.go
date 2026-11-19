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
			{"write own task", "Write", "projects/example/tasks/T-01/notes.md", "allow"},
			{"write brain", "Edit", "knowledge/note.md", "allow"},
			{"project record", "Write", "projects/example/project.md", "allow"},
			{"ordinary home write", "Write", "notes.md", "allow"},
		} {
			t.Run(event+"/"+tt.name, func(t *testing.T) {
				root := t.TempDir()
				home := filepath.Join(root, "home")
				grid := filepath.Join(home, ".grid")
				cwd := filepath.Join(root, "repo")
				data := filepath.Join(root, "data")
				// A task folder, its Feed, the brain and a project record are not refused by a
				// Grid guard, and `grid setup` writes no deny rule for them: agents write their
				// task state. The Feed is proven free in TestLaHookFeedIsNotDeniedBySetup
				// (la_hook_feed_rule_test.go).
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

// caseAliasHome builds a Grid home with a project, its knowledge folder and the
// home's knowledge folder on a case-folding volume, with plan-guard at block so the
// rows below prove what the strictest level does.
func caseAliasHome(t *testing.T) (grid, home, cwd string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !foldsCase(t, root) {
		t.Skip("case-sensitive filesystem: another spelling names a different path")
	}
	home = filepath.Join(root, "home")
	grid = filepath.Join(home, ".grid")
	cwd = filepath.Join(root, "repo")
	for _, d := range []string{filepath.Join(grid, "projects", "example", "knowledge"), filepath.Join(grid, "knowledge"), cwd} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(grid, "projects", "example", "project.md"), []byte("---\nname: example\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// plan-guard warns by default (verdicts-11nov v-4); these rows run it at block.
	if err := (settings.Store{Home: grid}).SetHook("plan-guard", "block"); err != nil {
		t.Fatal(err)
	}
	return grid, home, cwd
}

// caseAliasWrite is what the hook answers a builder's Write to path.
func caseAliasWrite(t *testing.T, event, grid, home, cwd, path string) string {
	t.Helper()
	in := hookio.Input{AgentType: "grid:grid-builder", ToolName: "Write", Cwd: cwd, ToolInput: map[string]any{"file_path": path, "content": "text"}}
	stdin, _ := json.Marshal(in)
	var out bytes.Buffer
	Run(Options{
		Event: event, Stdin: bytes.NewReader(stdin), Stdout: &out, Cwd: cwd,
		Resolver: where.Resolver{GridHome: grid, Home: home},
		Getenv:   func(string) string { return "" },
	})
	return out.String()
}

// A builder writes the brain and a project record through another spelling of the
// Grid home on a case-folding volume as freely as through the first: agents write
// both, and the spelling is read as the Grid home, not as a path outside it. The
// project record was once refused here; plan-guard no longer refuses it.
func TestCaseAliasesOfTheGridHomeAreAllowed(t *testing.T) {
	grid, home, cwd := caseAliasHome(t)
	for _, event := range []string{"pretooluse", "permissionrequest"} {
		for _, tt := range []struct{ name, path string }{
			{"project knowledge", filepath.Join(grid, "PROJECTS", "example", "knowledge", "n.md")},
			{"project record", filepath.Join(grid, "PROJECTS", "example", "project.md")},
			{"home knowledge", filepath.Join(home, ".GRID", "knowledge", "n.md")},
		} {
			t.Run(event+"/"+tt.name, func(t *testing.T) {
				out := caseAliasWrite(t, event, grid, home, cwd, tt.path)
				if strings.Contains(out, `"deny"`) || !strings.Contains(out, `"allow"`) {
					t.Fatalf("not allowed: %s", out)
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
	// The judged call is a role refusal of the home guard, which no setting turns off: a
	// guest may not write into the Grid home's Personal scope.
	head := `{"tool_name":"Write","tool_input":{"file_path":"` + filepath.Join(grid, "personal", "x") + `","content":"`
	tail := `"}}`
	stdin := head + strings.Repeat("x", hookio.InputLimit-len(head)-len(tail)) + tail
	if len(stdin) != hookio.InputLimit {
		t.Fatal(len(stdin))
	}
	var out bytes.Buffer
	Run(Options{Event: "pretooluse", Stdin: strings.NewReader(stdin), Stdout: &out, Cwd: root,
		Resolver: where.Resolver{GridHome: grid, Home: filepath.Join(root, "home")},
		Getenv: func(k string) string {
			return map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "gus", "GRID_PROJECT": "example"}[k]
		}})
	if !strings.Contains(out.String(), `"deny"`) || !strings.Contains(out.String(), "This caller cannot access this Grid home scope.") {
		t.Fatalf("the guest write into Personal was not judged by the guards: %.300s", out.String())
	}
}
