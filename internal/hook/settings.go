package hook

import (
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func (o Options) hookLevel(name string, in hookio.Input) (string, error) {
	cwd := in.Cwd
	if cwd == "" {
		cwd = o.Cwd
	}
	level, err := settings.ResolveHookLevel(name, o.Resolver, where.Query{
		Cwd: cwd, Project: o.Getenv("GRID_PROJECT"), Task: o.Getenv("GRID_TASK"),
	})
	o.log("hook settings", err)
	return level, err
}

func (o Options) hookEnabled(name string, in hookio.Input) bool {
	level, _ := o.hookLevel(name, in)
	return level != "off"
}
