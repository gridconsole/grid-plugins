package guard

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// pbRdContext is a guard context for agent whose user home carries a Grid config
// naming vault as the brain folder of the data folder gridHome ("" writes no
// config: the key is absent).
func pbRdContext(t *testing.T, agent, gridHome, vault string) context {
	t.Helper()
	userHome := t.TempDir()
	env := map[string]string{"HOME": userHome, "GRID_HOME": gridHome}
	if vault != "" {
		cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
		if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(k string) string { return env[k] }, userHome), cfg); err != nil {
			t.Fatal(err)
		}
	}
	return newContext(hookio.Input{AgentType: agent}, t.TempDir(), env)
}

// TestPbRdBrainPaths walks the guard row of the reader table (brain-obsidian.md
// 2.2, guard/context.go): with the Grid config's brain key set, a curator and a
// planner may write anywhere in the brain folder (Root); with it absent they may
// write the Grid home's knowledge/ and the projects' knowledge/, as before. The
// brain folder adds nothing to what another agent is refused.
func TestPbRdBrainPaths(t *testing.T) {
	gridHome, vault := t.TempDir(), t.TempDir()
	for _, tc := range []struct {
		name, agent, target string
		vault               bool
		want                bool
	}{
		{"curator, vault note", "grid:grid-curator", filepath.Join(vault, "Ideas", "Cafe plan.md"), true, true},
		{"planner, grid/ topic", "grid-planner", filepath.Join(vault, "grid", "workflows", "ship.md"), true, true},
		{"curator, vault project note", "grid-curator", filepath.Join(vault, "projects", "demo", "grid", "deploy", "lands.md"), true, true},
		{"builder, vault note", "grid:grid-builder", filepath.Join(vault, "Ideas", "Cafe plan.md"), true, false},
		{"no agent, vault note", "", filepath.Join(vault, "Ideas", "Cafe plan.md"), true, false},
		{"curator, outside both", "grid:grid-curator", filepath.Join(t.TempDir(), "x.md"), true, false},
		{"curator, old knowledge with key set", "grid:grid-curator", filepath.Join(gridHome, "knowledge", "decisions", "a.md"), true, true},
		{"curator, old knowledge, key absent", "grid:grid-curator", filepath.Join(gridHome, "knowledge", "decisions", "a.md"), false, true},
		{"curator, old project knowledge, key absent", "grid:grid-curator", filepath.Join(gridHome, "projects", "demo", "knowledge", "deploy", "a.md"), false, true},
		{"curator, vault path, key absent", "grid:grid-curator", filepath.Join(vault, "Ideas", "Cafe plan.md"), false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := ""
			if tc.vault {
				v = vault
			}
			c := pbRdContext(t, tc.agent, gridHome, v)
			if err := os.MkdirAll(filepath.Dir(tc.target), 0o700); err != nil {
				t.Fatal(err)
			}
			if got := c.brain(realNative(tc.target, 0)); got != tc.want {
				t.Errorf("brain(%s) = %t, want %t", tc.target, got, tc.want)
			}
		})
	}
}
