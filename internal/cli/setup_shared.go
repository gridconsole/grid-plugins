package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

const setupSharedUsage = "Usage: grid setup --shared-group <group> [--json]"

// groupName is a group as a shell command can safely spell it.
var groupName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]{0,63}$`)

// machineEnv is what the machine folder reads from the machine: the environment
// of this run and the login's home.
func (e Env) machineEnv() machinefolder.Env {
	return machinefolder.Env{Getenv: e.getenv, Home: e.Home, GOOS: e.machineGOOS}
}

// setupSharedGroupAsked reports that the arguments hold the --shared-group flag,
// which has its own parser: neither the Claude Code form (no operand) nor the
// project form (one operand) of grid setup knows it.
func setupSharedGroupAsked(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return false
		case arg == "--answers" || arg == "-answers":
			i++
		case strings.HasPrefix(arg, "-"):
			name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
			if name == "shared-group" || strings.HasPrefix(name, "shared-group=") {
				return true
			}
		}
	}
	return false
}

// sharedGroupResult is the --json answer of grid setup --shared-group.
type sharedGroupResult struct {
	Group    string   `json:"group"`
	Path     string   `json:"path"`
	Config   string   `json:"config"`
	Ready    bool     `json:"ready"`
	Why      string   `json:"why,omitempty"`
	Commands []string `json:"commands"`
}

// runSetupShared writes the two config keys and, when the folder is missing or the
// login cannot write in it, prints the commands a person runs. It makes no folder,
// changes no group or mode and runs no command: the only thing it writes is the
// config. The exit is 0 once the config is written; the commands are the next step
// of a person, and grid doctor checks them afterwards.
func runSetupShared(args []string, env Env) int {
	fs := newFlags("setup", env)
	fs.Usage = func() { fmt.Fprintln(env.Stdout, setupSharedUsage) }
	group := fs.String("shared-group", "", "group that shares the machine folder")
	asJSON := fs.Bool("json", false, "print JSON")
	operands, code, stop := parseMixed(fs, args)
	if stop {
		return code
	}
	if len(operands) != 0 {
		fmt.Fprintln(env.Stderr, setupSharedUsage)
		return 2
	}
	if !groupName.MatchString(*group) {
		fmt.Fprintf(env.Stderr, "setup: %q is not a group name (letters, digits, _ . -; it starts with a letter or _)\n%s\n", *group, setupSharedUsage)
		return 2
	}

	menv := env.machineEnv()
	folder := machinefolder.Resolve(menv)
	plan := machinefolder.PlanShared(menv, *group, env.machineCheck)
	if plan.Path == "" {
		return commandError(env, "setup", errors.New(plan.Why))
	}
	cfgPath, err := machinefolder.WriteConfig(menv, machinefolder.Config{MachineFolder: plan.Path, SharedGroup: *group})
	if err != nil {
		return commandError(env, "setup", err)
	}
	res := sharedGroupResult{Group: *group, Path: plan.Path, Config: cfgPath, Ready: plan.Ready, Why: plan.Why, Commands: plan.Commands}
	if res.Commands == nil {
		res.Commands = []string{}
	}
	if *asJSON {
		return writeJSON(env.Stdout, res)
	}

	w := env.Stdout
	fmt.Fprintf(w, "Wrote %s and %s to %s\n", machinefolder.KeySharedGroup, machinefolder.KeyMachineFolder, gridhome.Tilde(cfgPath, env.Home))
	fmt.Fprintf(w, "Shared group: %s\nMachine folder: %s\n", *group, plan.Path)
	if folder.Problem != "" {
		fmt.Fprintf(w, "Note: %s; it is replaced by the folder above.\n", folder.Problem)
	}
	if folder.Source == machinefolder.SourceDefault {
		fmt.Fprintf(w, "Note: this is the standard place of this login, which no other login shares. To share one folder, set %s to it (for example /srv/grid) and run this again.\n", machinefolder.EnvDir)
	}
	if plan.Ready {
		fmt.Fprintln(w, "The folder exists and this login can write in it.")
	} else {
		fmt.Fprintf(w, "The folder is not ready: %s.\nGrid runs no sudo. A person with sudo runs these commands:\n", plan.Why)
		for _, c := range plan.Commands {
			fmt.Fprintf(w, "  %s\n", c)
		}
	}
	fmt.Fprintf(w, "For each other login that shares the folder: sudo usermod -aG %s <login>, then that login logs in again.\n", *group)
	fmt.Fprintln(w, "Run grid doctor to check the folder's group and mode.")
	return 0
}
