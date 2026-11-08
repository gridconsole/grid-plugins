package guard

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

const off = " To turn this guard off, start Claude Code with GRID_PLAN_GUARD=off."

var oddName = regexp.MustCompile(`[^\x20-\x7e]|:|[. ](?:/|$)`)
var oddPart = regexp.MustCompile(`:|[. ](?:/|$)`)
var notASCII = regexp.MustCompile(`[^\x20-\x7e]`)
var historyAlias = regexp.MustCompile(`(?i)^history\.md(?::.*|[. ]*)$`)

// EvaluatePlan ports the legacy plan rules. root is the filesystem base used
// when cwd is omitted; all environment reads come from env. It performs no writes.
func EvaluatePlan(in hookio.Input, root string, env map[string]string) (decision hookio.Decision) {
	defer func() { decision = configuredLevel("plan-guard", in, root, env, decision) }()
	c := newContext(in, root, env)
	in = c.in
	if c.projects == "" || strings.EqualFold(strings.TrimSpace(env["GRID_PLAN_GUARD"]), "off") {
		return hookio.Decision{}
	}
	if in.ToolName == "Bash" {
		return c.recallGate()
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
	text, history, top := "", "", ""
	if owner != nil {
		text = read(filepath.Join(owner.dir, "task.md"))
		history = read(filepath.Join(owner.dir, "history.md"))
		top = strings.Split(owner.rel, "/")[0]
	}
	unaccepted := owner != nil && held(owner.dir, text, history)
	slot := strings.Split(relative(projects, target), "/")
	if owner == nil && inside(projects, target) && len(slot) > 2 && slot[1] != "tasks" && (oddName.MatchString(slot[1]) || strings.EqualFold(slot[1], "tasks")) {
		return hookio.Deny("A folder beside a project's tasks folder keeps a plain name (printable ASCII, no colon, no dot or space at the end, not tasks in other letters): on macOS or Windows it can be the tasks folder under another spelling, so " + given + " is not written." + off)
	}
	if owner != nil && gateLike(top) {
		return hookio.Deny(given + " is not written: a name at the top of a task folder is not plan accepted, preview approved or ship approved, as history.md would log it like a person's approval." + off)
	}
	if owner != nil && historyAlias.MatchString(owner.rel) {
		for _, next := range after(in.ToolName, in.ToolInput, history) {
			if addsMark(history, next, previewApproved) {
				return hookio.Deny(given + " is not changed to add a preview approved line: a person approves the preview in Grid, and Grid writes the line." + off)
			}
		}
		for _, next := range after(in.ToolName, in.ToolInput, history) {
			if addsMark(history, next, planReady) && !plan.HasRecall(read(filepath.Join(owner.dir, "plan.md"))) {
				return hookio.Deny(given + " is not changed to add a plan ready line: plan.md has no brain recall line. Add it under ## How we approach it: one `Used: <path>` line for each note of the recall you used, or `Recall: nothing found` when no note bears on the task (the recall hits are in your context, or run `grid-brain recall --task <id>`). Then run plan-hash.mjs again and add the plan ready line." + off)
			}
		}
	}
	if unaccepted && (notASCII.MatchString(top) || oddPart.MatchString(owner.rel)) {
		return hookio.Deny("This task's plan is not accepted yet, so " + given + " is not written: until then its files keep plain names (printable ASCII at the top of the task folder, no colon, no dot or space at the end)." + off)
	}
	forged := ""
	if unaccepted {
		forged = leavesPlan(in.ToolName, in.ToolInput, strings.ToLower(owner.rel), text, history)
	}
	switch forged {
	case "status":
		return hookio.Deny("This task's plan is not accepted yet, so " + given + " is not changed to close or pause the task: the person does that in Grid." + off)
	case "unreadable":
		return hookio.Deny("This task's plan is not accepted yet, so " + given + " is not changed to a task.md whose step or workflow cannot be read for sure: give it a step and a workflow of {step, kind} items with plain lower-case names, each named once." + off)
	case "unclear":
		return hookio.Deny("This task's plan is not accepted yet: Grid records the acceptance, and what " + given + " would hold after this edit cannot be told (an old_string not in the file, or input that is not text), so it is not changed. Read it and edit it again." + off)
	case "":
	default:
		return hookio.Deny("This task's plan is not accepted yet: Grid records the acceptance, so " + given + " is not changed to say so or to move the task past its plan step." + off)
	}
	agent := agentName(in.AgentType)
	if c.set && (agent == "grid-reviewer" || agent == "grid-checker") && !inside(projects, target) {
		return hookio.Deny("A " + agent + " writes only its Verdict inside a task folder; " + target + " is outside it.")
	}
	role := strings.TrimPrefix(strings.TrimSpace(in.AgentType), "grid:")
	if role != "grid-curator" && role != "grid-planner" {
		role = ""
	}
	if strings.TrimSpace(in.AgentType) != "" && role == "" && c.brain(target) {
		return hookio.Deny("Only a grid-curator (or a grid-planner) writes the brain; a " + agent + " writes a proposal in its task's brain-proposals/ folder instead of " + given + ".")
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
	if role == "grid-curator" {
		if c.brain(target) {
			return hookio.Decision{}
		}
		return hookio.Deny("A curator writes only the brain (a project's knowledge/ folder or the Grid home's) and its task folder. Write inside " + where + ", not " + given + "." + off)
	}
	if c.set && role == "grid-planner" {
		if unaccepted || (owner == nil && len(tasks) == 0 && inside(projects, target)) || artifact(projects, target, owner) || c.brain(target) {
			return hookio.Decision{}
		}
		return hookio.Deny("A planner writes only inside a task folder (its own, or one whose plan is not accepted yet), an artifacts folder or a brain folder. Write inside " + where + ", not " + given + "." + off)
	}
	all := append([]string{}, tasks...)
	if pointerKey(in.AgentID) != "" {
		all = append(all, c.mainTasks()...)
	}
	held := []string{}
	for _, d := range unique(all) {
		if isHeld(d) {
			held = append(held, d)
		}
	}
	if len(held) == 0 || inside(projects, target) {
		return hookio.Decision{}
	}
	return hookio.Deny("A task whose plan is not accepted yet writes only inside its task folder or the Grid projects folder. Write inside " + strings.Join(held, " or ") + ", not " + given + ". Writes elsewhere start once the person accepts the plan in Grid." + off)
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
func addsMark(text, next string, markers ...string) bool {
	counts := map[string]int{}
	marked := func(line string) bool {
		for _, m := range markers {
			if strings.Contains(line, m) {
				return true
			}
		}
		return false
	}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if marked(line) {
			counts[line]++
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(next, "\r\n", "\n"), "\n") {
		if marked(line) {
			if counts[line] == 0 {
				return true
			}
			counts[line]--
		}
	}
	return false
}
func leavesPlan(tool string, input map[string]any, file, text, history string) string {
	if file != "task.md" && file != "history.md" {
		return ""
	}
	if tool == "NotebookEdit" {
		return "unclear"
	}
	isTask := file == "task.md"
	before := history
	if isTask {
		before = text
	}
	nexts := after(tool, input, before)
	if len(nexts) == 0 {
		return "unclear"
	}
	unreadable := planStage(text, history) == "unreadable"
	for _, next := range nexts {
		if !isTask && addsMark(history, next, planAcceptedMark, previewApproved) {
			return "accepted"
		}
		stage := planStage(text, next)
		if isTask {
			stage = planStage(next, history)
		}
		if !heldStage(stage) && !(isTask && text == "" && stage == "") {
			return "accepted"
		}
		if isTask && stage == "unreadable" && !unreadable {
			return "unreadable"
		}
		if isTask && inactive(next) && !inactive(text) {
			return "status"
		}
	}
	return ""
}
