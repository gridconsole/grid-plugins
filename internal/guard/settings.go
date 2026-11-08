package guard

import (
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// configuredLevel is shared by the direct guard adapters and hook dispatch.
// Unreadable settings never silently turn a denial off.
func configuredLevel(name string, in hookio.Input, root string, env map[string]string, d hookio.Decision) hookio.Decision {
	if d.Behavior == "" {
		return d
	}
	home := gridhome.Dir(func(key string) string { return env[key] }, env["HOME"])
	if home == "" {
		return d
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = root
	}
	if cwd == "" {
		cwd = "."
	}
	machine := env["GRID_MACHINE"]
	if machine == "" {
		machine = gridhome.Machine(home)
	}
	r, err := (where.Resolver{GridHome: home, Home: env["HOME"], Machine: machine}).Resolve(where.Query{Cwd: cwd, Project: env["GRID_PROJECT"], Task: env["GRID_TASK"]})
	if err != nil {
		return hookio.Deny("Cannot resolve hook settings: " + err.Error())
	}
	project := ""
	if r.Record {
		project = r.Project
	}
	levels, err := settings.ReadHookLevels(home, project)
	if err != nil {
		return hookio.Deny("Cannot read hook settings: " + err.Error())
	}
	switch levels[name] {
	case "off":
		return hookio.Decision{}
	case "warn":
		if d.Behavior == "deny" || d.Behavior == "ask" {
			return hookio.Decision{Warnings: []string{fmt.Sprintf("%s: %s", name, d.Reason)}}
		}
	}
	return d
}
