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

// laHdRun drives one PreToolUse through Run, as TestGuardDispatch does, with the
// host-guard setting at level, and returns what the hook printed.
func laHdRun(t *testing.T, level, tool, rel string) string {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	grid := filepath.Join(home, ".grid")
	cwd := filepath.Join(root, "repo")
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(filepath.Join(grid, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (settings.Store{Home: grid}).SetHook("host-guard", level); err != nil {
		t.Fatal(err)
	}
	// No agent type: the main agent writes.
	in := hookio.Input{ToolName: tool, Cwd: cwd, ToolInput: map[string]any{"file_path": filepath.Join(grid, rel), "content": "text"}}
	stdin, _ := json.Marshal(in)
	var out bytes.Buffer
	code := Run(Options{
		Event: "pretooluse", Stdin: bytes.NewReader(stdin), Stdout: &out, Cwd: cwd,
		Resolver: where.Resolver{GridHome: grid, Home: home},
		Getenv: func(k string) string {
			return map[string]string{"GRID_PLAN_GUARD": "off", "CLAUDE_PLUGIN_DATA": data}[k]
		},
	})
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out.Bytes())
	}
	return out.String()
}

// The hook no longer refuses a write to the Grid host folder, even where the person
// set the retired host-guard level to block, and `grid setup` writes no deny rule for
// it either (TestSetupWritesNoDenyRulesForAgentState in internal/cli).
func TestLaHostDropHookNoLongerRefuses(t *testing.T) {
	for _, level := range []string{"block", "warn", "off"} {
		for _, rel := range []string{".host/x", ".host/approval/x.json", ".HOST/x"} {
			t.Run(level+"/"+rel, func(t *testing.T) {
				got := laHdRun(t, level, "Write", rel)
				if strings.Contains(got, `"deny"`) {
					t.Fatalf("host-guard %s: the hook refused %s: %s", level, rel, got)
				}
			})
		}
	}
}
