package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/doctor"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func runDoctorChecks(args []string, env Env) int {
	fs := newFlags("doctor", env)
	asJSON := fs.Bool("json", false, "print JSON")
	bundle := fs.Bool("bundle", false, "write a support bundle")
	root := os.Getenv("CLAUDE_PLUGIN_ROOT")
	if root == "" {
		root = env.Cwd
	}
	hooks := fs.String("hooks-file", filepath.Join(root, "hooks", "hooks.json"), "plugin hooks file (defaults to CLAUDE_PLUGIN_ROOT/hooks/hooks.json or ./hooks/hooks.json)")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if *bundle {
		fmt.Fprintln(env.Stderr, "doctor --bundle: not built yet")
		return 2
	}
	rep := doctor.Run(doctor.Options{GridHome: env.GridHome, Label: gridhome.Tilde(env.GridHome, env.Home), GitVersion: env.GitVersion, HooksFile: *hooks})
	if *asJSON {
		if code := writeJSON(env.Stdout, rep); code != 0 {
			return code
		}
	} else {
		for _, c := range rep.Checks {
			status := "ok"
			if !c.OK {
				status = "fail"
			}
			fmt.Fprintf(env.Stdout, "%s %s: %s\n", status, c.Name, c.Detail)
			if !c.OK {
				fmt.Fprintf(env.Stdout, "fix: %s\n", c.Fix)
			}
		}
	}
	if !rep.OK {
		return 1
	}
	return 0
}
