package guard

import (
	"github.com/gridconsole/grid-plugins/internal/contract"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// PlanGuardOffCommand is the command a person runs to turn the plan guard off.
const PlanGuardOffCommand = "grid hook level plan-guard off"

const off = " To turn this guard off, run " + PlanGuardOffCommand + " (or start Claude Code with GRID_PLAN_GUARD=off)."

var oddName = regexp.MustCompile(`[^\x20-\x7e]|:|[. ](?:/|$)`)

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
	owner := taskAt(target, projects)
	text, history := "", ""
	if owner != nil {
		text = read(filepath.Join(owner.dir, "task.md"))
		history = read(filepath.Join(owner.dir, "history.md"))
	}
	// planning only widens what a grid-planner may write (the planner fence below);
	// it never blocks a write.
	planning := owner != nil && held(stepsFor(c.env, owner.dir), owner.dir, text, history)
	slot := strings.Split(relative(projects, target), "/")
	if owner == nil && inside(projects, target) && len(slot) > 2 && slot[1] != "tasks" && (oddName.MatchString(slot[1]) || strings.EqualFold(slot[1], "tasks")) {
		return hookio.Deny("A folder beside a project's tasks folder keeps a plain name (printable ASCII, no colon, no dot or space at the end, not tasks in other letters): on macOS or Windows it can be the tasks folder under another spelling, so " + given + " is not written." + off)
	}
	agent := agentName(in.AgentType)
	// A file the caller's own Job names is its to write: the fence reads the Job
	// only where it would fire, and the other fences below still answer for it.
	if c.set && (contract.NormalizeRole(agent) == "reviewer" || contract.NormalizeRole(agent) == "checker") && !inside(projects, target) && !c.jobNames(given, target) {
		return hookio.Deny("A " + agent + " writes only its Verdict inside a task folder; " + target + " is outside it." + off)
	}
	role := ""
	switch strings.TrimSpace(in.AgentType) {
	case "grid:grid-curator", "grid-curator":
		role = "curator"
	case "grid:grid-planner", "grid-planner":
		role = "planner"
	}
	if strings.TrimSpace(in.AgentType) != "" && role == "" && c.brain(target) {
		return hookio.Deny("Only a grid-curator (or a grid-planner) writes the brain; a " + agent + " writes a proposal in its task's brain-proposals/ folder instead of " + given + "." + off)
	}
	tasks := []string{}
	if t := taskAt(c.path(in.Cwd), projects); t != nil {
		tasks = append(tasks, t.dir)
	}
	tasks = unique(append(tasks, c.agentTasks()...))
	for _, d := range tasks {
		if inside(realNative(d, 0), target) {
			return hookio.Decision{}
		}
	}
	where := projects
	if len(tasks) > 0 {
		where = strings.Join(tasks, " or ")
	}
	if role == "curator" {
		if c.brain(target) {
			return hookio.Decision{}
		}
		return hookio.Deny("A curator writes only the brain (a project's knowledge/ folder or the Grid home's) and its task folder. Write inside " + where + ", not " + given + "." + off)
	}
	if c.set && role == "planner" {
		if planning || (owner == nil && len(tasks) == 0 && inside(projects, target)) || artifact(projects, target, owner) || c.brain(target) {
			return hookio.Decision{}
		}
		return hookio.Deny("A planner writes only inside a task folder (its own, or one whose plan is not accepted yet), an artifacts folder or a brain folder. Write inside " + where + ", not " + given + "." + off)
	}
	// A task whose plan is not accepted yet does not limit where its agents write:
	// whether a plan needs a person's approval is a step of the task's workflow
	// (an approve step), and the workflow's default says who answers and when.
	return hookio.Decision{}
}
func artifact(root, target string, owner *task) bool {
	base := strings.ToLower(filepath.Base(target))
	if !inside(root, target) || base == "history.md" || base == "task.md" {
		return false
	}
	if owner != nil {
		return strings.HasPrefix(owner.rel, "artifacts/")
	}
	slot := strings.Split(relative(root, target), "/")
	return len(slot) > 2 && slot[1] == "artifacts"
}
