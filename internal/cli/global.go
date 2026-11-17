package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// globalFlags are the flags that go before the command name. Each one is the
// flag form of an environment variable and wins over it; the variable still works
// when the flag is absent.
var globalFlags = map[string]string{
	"grid-home":   "GRID_HOME",
	"member":      "GRID_MEMBER",
	"role":        "GRID_ROLE",
	"lease-epoch": "GRID_LEASE_EPOCH",
}

// applyGlobalFlags strips the global flags from the front of args and returns the
// rest with an Env that carries them: every command reads GRID_MEMBER, GRID_ROLE and
// GRID_LEASE_EPOCH through env.getenv, so the flag shows through there, and
// --grid-home sets the Grid home and the machine id read from it. A non-zero code
// is the exit code for a malformed global flag.
func applyGlobalFlags(args []string, env Env) ([]string, Env, int) {
	over := map[string]string{}
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		name, value, hasValue := strings.Cut(args[0][2:], "=")
		key, ok := globalFlags[name]
		if !ok {
			break
		}
		args = args[1:]
		if !hasValue {
			if len(args) == 0 {
				fmt.Fprintf(env.Stderr, "grid: --%s needs a value\n", name)
				return nil, env, 2
			}
			value, args = args[0], args[1:]
		}
		if value == "" {
			fmt.Fprintf(env.Stderr, "grid: --%s needs a value\n", name)
			return nil, env, 2
		}
		over[key] = value
	}
	if len(over) == 0 {
		return args, env, 0
	}
	if gh, ok := over["GRID_HOME"]; ok {
		if !filepath.IsAbs(gh) {
			gh = filepath.Join(env.Cwd, gh)
		}
		gh = gridhome.Real(gh)
		over["GRID_HOME"] = gh
		env.GridHome = gh
		env.Resolved.Dir, env.Resolved.Source = gh, gridhome.SourceFlag
		env.Machine = gridhome.Machine(gh)
	}
	env.typedGlobal = map[string]bool{}
	for key := range over {
		env.typedGlobal[key] = true
	}
	base := env.getenv
	env.Getenv = func(key string) string {
		if v, ok := over[key]; ok {
			return v
		}
		return base(key)
	}
	return args, env, 0
}
