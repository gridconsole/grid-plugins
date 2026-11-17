package guard

import (
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// configuredLevel is shared by the direct guard adapters and hook dispatch.
// Unreadable settings never silently turn a denial off. Every guard reads its
// level here, and the default of each is block, so a guard with no setting keeps
// its decision.
func configuredLevel(name string, in hookio.Input, root string, env map[string]string, d hookio.Decision) hookio.Decision {
	if d.Behavior == "" {
		return d
	}
	resolver := callerResolver(env)
	home := resolver.GridHome
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
	level, err := settings.ResolveHookLevel(name, resolver,
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

// callerResolver is shared by hook settings and automatic folder permissions.
func callerResolver(env map[string]string) where.Resolver {
	home := gridhome.Resolve(func(key string) string { return env[key] }, env["HOME"]).Dir
	machine := env["GRID_MACHINE"]
	if machine == "" {
		machine = gridhome.Machine(home)
	}
	return where.Resolver{GridHome: home, Home: env["HOME"], Machine: machine, Caller: where.CallerFrom(func(key string) string { return env[key] })}
}
