package guard

import (
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// PlanGuardOffCommand is the command a person runs to turn the plan guard off.
const PlanGuardOffCommand = "grid hook level plan-guard off"

const off = " To turn this guard off, run " + PlanGuardOffCommand + " (or start Claude Code with GRID_PLAN_GUARD=off)."

// EvaluatePlan ports the legacy plan rules. root is the filesystem base used
// when cwd is omitted; all environment reads come from env. It performs no writes.
// Its level is the plan-guard setting, warn by default.
func EvaluatePlan(in hookio.Input, root string, env map[string]string) hookio.Decision {
	return configuredLevel("plan-guard", in, root, env, evaluatePlan(in, root, env))
}

func evaluatePlan(in hookio.Input, root string, env map[string]string) hookio.Decision {
	c := newContext(in, root, env)
	in = c.in
	if c.projects == "" || strings.EqualFold(strings.TrimSpace(env["GRID_PLAN_GUARD"]), "off") {
		return hookio.Decision{}
	}
	if !fileTool(in.ToolName) {
		return hookio.Decision{}
	}
	given := c.target()
	if given == "" {
		return hookio.Decision{}
	}
	target := c.path(given)
	projects := realNative(c.projects, 0)
	agent := agentName(in.AgentType)
	// A file the caller's own Job names is its to write: the fence reads the Job
	// only where it would fire. A reviewer or checker keeps to its Verdict, so it
	// does not edit what it reviews; every other agent writes task folders, the
	// brain and project files freely, and a task whose plan is not accepted yet does
	// not limit where its agents write either (whether a plan needs a person's
	// approval is a step of the task's workflow).
	if c.set && (contract.NormalizeRole(agent) == "reviewer" || contract.NormalizeRole(agent) == "checker") && !inside(projects, target) && !c.jobNames(given, target) {
		return hookio.Deny("A " + agent + " writes only its Verdict inside a task folder; " + target + " is outside it." + off)
	}
	return hookio.Decision{}
}
