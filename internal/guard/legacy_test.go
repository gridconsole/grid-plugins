package guard

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/testenv"
)

// Optional differential checks use a read-only checkout named by the caller.
// Normal tests need neither Node nor reference material. No fixtures are written
// into that checkout, and no legacy script being allowed is actually executed.
func legacyCompare(t *testing.T, fn Adapter, in hookio.Input, env map[string]string, out []byte) {
	t.Helper()
	root := os.Getenv("GRID_LEGACY_PLUGIN")
	if root == "" {
		return
	}
	script := ""
	switch reflect.ValueOf(fn).Pointer() {
	case reflect.ValueOf(PlanGuard).Pointer():
		script = "plan-guard"
	case reflect.ValueOf(HomeAllow).Pointer():
		script = "grid-home-allow"
	}
	if script == "" {
		t.Fatal("unknown adapter")
	}
	if script == "grid-home-allow" && in.ToolName == "Bash" && env["CLAUDE_PLUGIN_ROOT"] != root {
		return
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", filepath.Join(root, "hooks", script+".mjs"))
	cmd.Stdin = bytes.NewReader(b)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = testenv.Temp(t, cmd.Env)
	old, err := cmd.Output()
	if err != nil {
		t.Fatalf("legacy %s: %v", script, err)
	}
	if !bytes.Equal(out, old) {
		t.Fatalf("legacy %s output differs\nGo: %s\nJS: %s", script, out, old)
	}
}
func TestLegacyScriptAllowParity(t *testing.T) {
	root := os.Getenv("GRID_LEGACY_PLUGIN")
	if root == "" {
		t.Skip("set GRID_LEGACY_PLUGIN for optional differential checks")
	}
	for _, args := range []string{"", " --print", " plan.md", " --caption 'a/b.md'", " --url https://example.com/proof"} {
		t.Run(args, func(t *testing.T) {
			f := setup(t)
			f.env["CLAUDE_PLUGIN_ROOT"] = root
			i := f.input("Bash", "node "+filepath.Join(root, "skills", "deliver", "scripts", "proof-add.mjs")+args)
			i.Cwd = f.repo
			d := EvaluateHome(i, f.root, f.env)
			checkAdapter(t, HomeAllow, i, f.env, d)
		})
	}
}
