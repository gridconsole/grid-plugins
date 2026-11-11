package hook

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// CountHomeAllow appends one small record per winning fallback. No prompt,
// command, path, session id, or identity enters the counter. Append failures do
// not change a permission decision. Pure guard evaluators never call this.
func CountHomeAllow(env map[string]string, in hookio.Input) {
	// A counter is a record: whatever goes wrong here stays here.
	defer func() { _ = recover() }()
	root := env["CLAUDE_PLUGIN_DATA"]
	if root == "" {
		if home := env["GRID_HOME"]; home != "" {
			root = filepath.Join(home, ".hook-counts")
		}
	}
	if root == "" {
		return
	}
	if os.MkdirAll(root, 0o700) != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(root, "home-allow.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	b, err := json.Marshal(struct {
		Event string `json:"event"`
		Tool  string `json:"tool"`
	}{in.HookEventName, in.ToolName})
	if err == nil {
		_, _ = f.Write(append(b, '\n'))
	}
}
