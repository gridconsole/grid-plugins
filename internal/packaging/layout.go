// Package packaging generates the installable plugin from the Go dispatcher.
package packaging

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hook"
)

//go:embed grid.sh
var launcher []byte

// launcherCmd is the Windows entry, bin/grid.cmd. It ships with CRLF line endings, which cmd.exe needs.
//
//go:embed grid.cmd
var launcherCmd []byte

// HookConfig is shared by the Claude Code and Codex manifests.
type HookConfig struct {
	Hooks map[string][]HookGroup `json:"hooks"`
}

type HookGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookCommand `json:"hooks"`
}

type HookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

// Files returns deterministic generated metadata, relative to the plugin root.
func Files(version string) (map[string][]byte, error) {
	manifest := map[string]any{
		"name": "grid", "version": version,
		"description": "Grid: task folders, a planner per task, the Feed and the brain, all written through one grid command.",
		"author":      map[string]string{"name": "Grid"},
		"homepage":    "https://github.com/gridconsole/grid-plugins",
		"repository":  "https://github.com/gridconsole/grid-plugins", "license": "MIT",
	}
	config := HookConfig{Hooks: make(map[string][]HookGroup)}
	for _, e := range hook.Events() {
		config.Hooks[e.Name] = []HookGroup{{Matcher: e.Matcher, Hooks: []HookCommand{{
			Type: "command", Command: `"${CLAUDE_PLUGIN_ROOT}/bin/grid" hook ` + strings.ToLower(e.Name), Timeout: e.Timeout,
		}}}}
	}
	// Both hosts discover skills/, hooks/hooks.json and .mcp.json at the plugin root.
	values := map[string]any{
		".claude-plugin/plugin.json": manifest,
		".codex-plugin/plugin.json":  manifest,
		"hooks/hooks.json":           config,
		".mcp.json": map[string]any{"mcpServers": map[string]any{"grid-feed": map[string]any{
			"command": "${CLAUDE_PLUGIN_ROOT}/bin/grid", "args": []string{"mcp"},
		}}},
		".claude-plugin/marketplace.json": map[string]any{
			"name": "grid", "owner": map[string]string{"name": "Grid"},
			"plugins": []any{map[string]string{"name": "grid", "source": "./", "version": version}},
		},
	}
	files := make(map[string][]byte, len(values))
	for path, value := range values {
		b, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, err
		}
		files[path] = append(b, '\n')
	}
	files["bin/grid"] = launcher
	files["bin/grid.cmd"] = bytes.ReplaceAll(launcherCmd, []byte("\n"), []byte("\r\n"))
	return files, nil
}

// Write generates only the owned package files; skills and agents are untouched.
func Write(root, version string) error {
	files, err := Files(version)
	if err != nil {
		return err
	}
	for name, b := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if name == "bin/grid" {
			mode = 0o755
		}
		if err := os.WriteFile(path, b, mode); err != nil {
			return err
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
	}
	return nil
}
