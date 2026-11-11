package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

// pluginRoot finds the plugin folder: CLAUDE_PLUGIN_ROOT, else the root above the running binary.
func pluginRoot(env Env) string {
	root := env.getenv("CLAUDE_PLUGIN_ROOT")
	if root != "" {
		return root
	}
	exe, err := env.executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	root = filepath.Dir(exe)
	if base := filepath.Base(root); base == "bin" || base == "dist" {
		root = filepath.Dir(root)
	}
	return root
}

// runAgentSync renders the plugin's agent templates into the person's agents folder.
func runAgentSync(args []string, env Env) int {
	fs := newFlags("agent sync", env)
	asJSON := fs.Bool("json", false, "print JSON")
	templates := fs.String("templates", "", "agent template folder; defaults to agent-templates in the plugin root")
	dir := fs.String("dir", "", "agents folder to write; defaults to agents beside the Claude settings.json")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if env.GridHome == "" {
		return commandError(env, "agent sync", errors.New("cannot find the Grid home; set GRID_HOME"))
	}
	if resolver(env).Caller.IsGuest() {
		return commandError(env, "agent sync", ilGuestRefused)
	}
	if *templates == "" {
		root := pluginRoot(env)
		if root == "" {
			return commandError(env, "agent sync", errors.New("cannot find the plugin folder; set CLAUDE_PLUGIN_ROOT or use --templates"))
		}
		*templates = settings.AgentTemplatesPath(root)
	}
	if *dir == "" {
		d, err := settings.ClaudeAgentsDir(env.Home, env.getenv("CLAUDE_CONFIG_DIR"))
		if err != nil {
			return commandError(env, "agent sync", err)
		}
		*dir = d
	}
	results, err := settings.SyncAgents(settings.SyncOptions{Home: env.GridHome, TemplatesDir: *templates, AgentsDir: *dir})
	if err != nil {
		return commandError(env, "agent sync", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Agents []settings.SyncResult `json:"agents"`
		}{results})
	}
	for _, r := range results {
		if _, err := fmt.Fprintf(env.Stdout, "%s %s %s\n", r.Agent, r.State, r.Path); err != nil {
			return exitCode(err)
		}
	}
	return 0
}
