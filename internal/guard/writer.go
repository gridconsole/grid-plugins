package guard

import (
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// EvaluateOnlyWriter enforces the new CLI's ownership of task folders and the
// brain (plugin-design.md:49,90). It is composed at the event boundary so the
// individual legacy guard adapters remain useful for recording comparisons.
// Shell writes still require the worker's settings/sandbox; a text guard cannot
// reliably identify computed paths or arbitrary programs that write files. Its
// level is the only-writer setting, block by default.
func EvaluateOnlyWriter(in hookio.Input, root string, env map[string]string) hookio.Decision {
	return configuredLevel("only-writer", in, root, env, evaluateOnlyWriter(in, root, env))
}

func evaluateOnlyWriter(in hookio.Input, root string, env map[string]string) hookio.Decision {
	if !fileTool(in.ToolName) {
		return hookio.Decision{}
	}
	c := newContext(in, root, env)
	if c.projects == "" || c.target() == "" {
		return hookio.Decision{}
	}
	target := c.path(c.target())
	owner := taskAtFold(strings.ToLower(target), strings.ToLower(realNative(c.projects, 0)))
	personalRoot := realNative(filepath.Join(c.home, gridhome.Personal, "tasks"), 0)
	personalRel := strings.Split(relative(personalRoot, target), "/")
	personal := c.home != "" && len(personalRel) > 0 && taskName.MatchString(strings.ToUpper(personalRel[0]))
	if owner != nil || personal || c.brain(target) {
		return hookio.Deny("Task folders and the brain are written only by grid. Use the corresponding grid command to make this change.")
	}
	parts := strings.Split(relative(realNative(c.projects, 0), target), "/")
	if len(parts) == 2 && parts[0] != ".." && strings.EqualFold(filepath.Base(target), "project.md") {
		return hookio.Deny("Project records are written only by grid. Use grid project to make this change.")
	}
	return hookio.Decision{}
}
