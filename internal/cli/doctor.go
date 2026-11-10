package cli

import (
	"fmt"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/doctor"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

func runDoctor(args []string, env Env) int {
	fs := newFlags("doctor", env)
	asJSON := fs.Bool("json", false, "print JSON")
	bundle := fs.Bool("bundle", false, "write a support bundle")
	out := fs.String("out", "", "support zip path (requires --bundle)")
	hooks := fs.String("hooks-file", "", "plugin hooks file (defaults to hooks/hooks.json in the plugin root: CLAUDE_PLUGIN_ROOT, else found from the grid executable)")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *out != "" && !*bundle {
		fmt.Fprintln(env.Stderr, "doctor: --out requires --bundle")
		return 2
	}
	if env.GridHome == "" {
		fmt.Fprintln(env.Stderr, "grid: cannot find the Grid home; set GRID_HOME")
		return 1
	}
	if *hooks == "" {
		*hooks = defaultHooksFile(env)
	}
	settingsPath, _ := settings.ClaudeSettingsPath(env.Home, env.getenv("CLAUDE_CONFIG_DIR"))
	rep := doctor.Run(doctor.Options{Home: env.Home, SettingsPath: settingsPath, GridHome: env.GridHome, Label: gridhome.Tilde(env.GridHome, env.Home), GitVersion: env.GitVersion, HooksFile: *hooks})
	if *bundle {
		path := *out
		if path != "" && !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
		path, err := doctor.Bundle(doctor.BundleOptions{GridHome: env.GridHome, Home: env.Home, SettingsPath: settingsPath, Out: path, Report: rep})
		if err != nil {
			fmt.Fprintf(env.Stderr, "doctor --bundle: %v\n", err)
			return 1
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Path   string        `json:"path"`
				Doctor doctor.Report `json:"doctor"`
			}{path, rep})
		}
		if _, err = fmt.Fprintln(env.Stdout, path); err != nil {
			return 1
		}
		return 0
	}
	if *asJSON {
		if code := writeJSON(env.Stdout, rep); code != 0 {
			return code
		}
	} else {
		if err := doctor.WriteReport(env.Stdout, rep); err != nil {
			return 1
		}
	}
	if !rep.OK {
		return 1
	}
	return 0
}

// defaultHooksFile finds the plugin's hooks.json from the plugin root, never from
// the current folder: CLAUDE_PLUGIN_ROOT, else the root above the running binary.
func defaultHooksFile(env Env) string {
	root := pluginRoot(env)
	if root == "" {
		return ""
	}
	return filepath.Join(root, "hooks", "hooks.json")
}
