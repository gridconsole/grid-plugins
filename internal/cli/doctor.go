package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

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
	projects := env.getenv("GRID_PROJECTS")
	if projects != "" && !filepath.IsAbs(projects) {
		projects = filepath.Join(env.Cwd, projects)
	}
	rep := doctor.Run(doctor.Options{Home: env.Home, SettingsPath: settingsPath, GridHome: env.GridHome, Projects: projects, Label: gridhome.Tilde(env.GridHome, env.Home), GitVersion: env.GitVersion, HooksFile: *hooks, GridVersion: gridVersionAt, Resolved: env.Resolved})
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

// gridVersionAt runs `<path> version --json` (no shell, no network) and returns
// its `version`. Any failure is an error: doctor then says the version was not read.
func gridVersionAt(path string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "version", "--json").Output()
	if err != nil {
		return "", err
	}
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return "", err
	}
	if doc.Version == "" {
		return "", errors.New("no version in the answer")
	}
	return doc.Version, nil
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
