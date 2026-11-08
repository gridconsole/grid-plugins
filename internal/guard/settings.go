package guard

import (
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// configuredLevel is shared by the direct guard adapters and hook dispatch.
// Unreadable settings never silently turn a denial off, and a guard that always
// blocks ignores the settings files altogether.
func configuredLevel(name string, in hookio.Input, root string, env map[string]string, d hookio.Decision) hookio.Decision {
	if d.Behavior == "" || settings.AlwaysBlock(name) {
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
	level, err := settings.ResolveHookLevel(name,
		where.Resolver{GridHome: home, Home: env["HOME"], Machine: env["GRID_MACHINE"]},
		where.Query{Cwd: cwd, Project: env["GRID_PROJECT"], Task: env["GRID_TASK"]})
	if err != nil {
		if d.Behavior == "deny" || d.Behavior == "ask" {
			d.Warnings = append(d.Warnings, err.Error())
			return d
		}
		// An unreadable permission fallback must never grant access.
		return hookio.Deny(err.Error())
	}
	switch level {
	case "off":
		return hookio.Decision{}
	case "warn":
		if d.Behavior == "deny" || d.Behavior == "ask" {
			return hookio.Decision{Warnings: []string{fmt.Sprintf("%s: %s", name, d.Reason)}}
		}
	}
	return d
}
