package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

const nhWfUsage = "usage: grid workflow show --task <task folder or id> [--project <name-or-id>] [--json] [--policy]"

// nhWfNoDefault is the text of TaskWorkflow's answer for a home whose workspace
// has no default workflow (internal/workflow/tasksteps.go defaultTemplate). That
// answer, and only that one, gives the built-in flow. It has no sentinel error,
// so its text is matched whole.
const nhWfNoDefault = "the workspace has no default workflow"

// nhWfAnswer is the workflow show answer with --task: the read of the task's
// project, unchanged, and the workflow the task runs.
type nhWfAnswer struct {
	workflow.Answer
	Task nhWfTask `json:"task"`
}

// nhWfTask is the workflow a task runs. Workflow and File are null for an inline
// list and for the built-in flow; Rounds is null for an inline list, which sets none.
type nhWfTask struct {
	ID       string     `json:"id"`
	Workflow *string    `json:"workflow"`
	Source   string     `json:"source"`
	File     *string    `json:"file"`
	Rounds   *int       `json:"rounds"`
	Steps    []nhWfStep `json:"steps"`
}

// nhWfStep is one step the task runs: its name and kind as TaskWorkflow gives
// them, and the template step's agent and approvers (null when it has none).
type nhWfStep struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Agent     *string  `json:"agent"`
	Approvers []string `json:"approvers"`
}

// nhWfShowTask answers workflow show --task. The task is resolved as grid plan
// resolves --task, its task.md goes to workflow.TaskWorkflow with the task's
// project, and the answer is that project's read with the task's workflow.
// Nothing is written to stdout before every read has answered.
func nhWfShowTask(env Env, task, project string, asJSON, withPolicy bool) int {
	r := resolver(env)
	scope, err := r.Resolve(where.Query{Cwd: env.Cwd, Task: task, Project: project})
	if err == nil && scope.TaskPath == "" {
		err = errors.New("no task; use --task <task folder or id>")
	}
	if err != nil {
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid workflow show: %v\n", err)
		return exitCode(err)
	}
	data, err := os.ReadFile(filepath.Join(scope.TaskPath, "task.md"))
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid workflow show: task %s: task.md cannot be read\n", scope.Task)
		return exitCode(err)
	}
	text := string(data)
	answerTask, err := nhWfResolve(r, scope.Project, nhWfTaskID(scope.Task, text), text)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid workflow show: %v\n", err)
		return 1
	}
	read := workflow.Read
	if withPolicy {
		read = workflow.ReadPolicy
	}
	answer, err := read(r, scope.Project)
	if err != nil {
		if errors.Is(err, workflow.ErrPolicy) {
			fmt.Fprintf(env.Stderr, "grid workflow show: %v\n", err)
			return 1
		}
		if errors.Is(err, localdata.ErrUnsafePath) {
			fmt.Fprintln(env.Stderr, "grid workflow show: workflow path is unsafe or inaccessible")
			return 2
		}
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid: %v\n", err)
		return exitCode(err)
	}
	if asJSON {
		return writeJSON(env.Stdout, nhWfAnswer{Answer: answer, Task: answerTask})
	}
	if _, err := fmt.Fprint(env.Stdout, nhWfText(answerTask)); err != nil {
		return exitCode(err)
	}
	return 0
}

// nhWfTaskID is the id task.md names, else the task's folder name.
func nhWfTaskID(folder, text string) string {
	if fm := taskfile.ParseFrontmatter(text); fm != nil {
		if id, ok := fm.Data["id"].(string); ok && strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id)
		}
	}
	return folder
}

// nhWfResolve is the task's workflow as TaskWorkflow resolves it: the inline
// list, or the template with its source, rounds and the steps for the task's
// size. TaskWorkflow's no-default answer is the built-in flow; every other
// error is returned as it is.
func nhWfResolve(r where.Resolver, project, id, text string) (nhWfTask, error) {
	t, steps, err := workflow.TaskWorkflow(r, project, text)
	if err != nil {
		if err.Error() == nhWfNoDefault {
			return nhWfBuiltIn(id), nil
		}
		return nhWfTask{}, err
	}
	out := nhWfTask{ID: id, Source: "inline", Steps: make([]nhWfStep, 0, len(steps))}
	if t == nil {
		for _, s := range steps {
			out.Steps = append(out.Steps, nhWfStep{Name: s.Step, Kind: s.Kind})
		}
		return out, nil
	}
	source, err := nhWfSource(t.File)
	if err != nil {
		return nhWfTask{}, err
	}
	name, file, rounds := t.Name, t.File, int(t.Rounds)
	out.Workflow, out.Source, out.File, out.Rounds = &name, source, &file, &rounds
	// The template's steps for the task's size, in order, are the ones
	// TaskWorkflow returned (templateSteps), so the two lists pair up.
	sized := nhWfSized(t.Steps, text)
	if len(sized) != len(steps) {
		return nhWfTask{}, fmt.Errorf("workflow %q: %d steps for the task's size, the resolver gave %d", t.Name, len(sized), len(steps))
	}
	for i, s := range steps {
		if strings.ToLower(sized[i].Name) != s.Step {
			return nhWfTask{}, fmt.Errorf("workflow %q step %q does not pair with the resolver's step %q", t.Name, sized[i].Name, s.Step)
		}
		out.Steps = append(out.Steps, nhWfStep{Name: s.Step, Kind: s.Kind, Agent: sized[i].Agent, Approvers: sized[i].Approvers})
	}
	return out, nil
}

// nhWfSized keeps the template steps the task's size runs, as templateSteps
// does: every step when the size is auto or unset, whose steps all run for
// every size once TaskWorkflow has answered.
func nhWfSized(steps []workflow.TemplateStep, text string) []workflow.TemplateStep {
	letter := ""
	if fm := taskfile.ParseFrontmatter(text); fm != nil {
		size, _ := fm.Data["size"].(string)
		letter = map[string]string{"small": "S", "normal": "N", "large": "L"}[size]
	}
	var out []workflow.TemplateStep
	for _, s := range steps {
		if letter == "" || strings.Contains(s.Sizes, letter) {
			out = append(out, s)
		}
	}
	return out
}

// nhWfSource names where a template came from by its file label: a brain note
// (brain:<level>/<name>), the project's workflows.md (<project>/workflows.md)
// or the workspace's (workflows.md).
func nhWfSource(file string) (string, error) {
	switch {
	case strings.HasPrefix(file, "brain:project/"):
		return "brain:project", nil
	case strings.HasPrefix(file, "brain:home/"):
		return "brain:home", nil
	case file == "workflows.md":
		return "workspace", nil
	case strings.HasSuffix(file, "/workflows.md"):
		return "project", nil
	}
	return "", fmt.Errorf("workflow file %q is not a known source", file)
}

// nhWfBuiltIn is the flow a task runs when the home has no default workflow:
// Run, then Check by grid-reviewer, 2 review rounds, the flow the app's Judge
// uses for such a home, then Ship, the person step: grid deliver posts its
// person Job (postShipPersonJob), the plan approve Job, so the step's kind is
// approve. Its approvers list is empty: the task's person, the owner when
// task.md names none.
func nhWfBuiltIn(id string) nhWfTask {
	reviewer, rounds := "grid-reviewer", 2
	return nhWfTask{ID: id, Source: "built-in", Rounds: &rounds, Steps: []nhWfStep{
		{Name: "Run", Kind: "run"},
		{Name: "Check", Kind: "check", Agent: &reviewer},
		{Name: "Ship", Kind: "approve", Approvers: []string{}},
	}}
}

// nhWfText is the text answer: the workflow, its source, its rounds and one
// line per step.
func nhWfText(t nhWfTask) string {
	name, rounds := "(none)", "(none)"
	if t.Workflow != nil {
		name = *t.Workflow
	}
	if t.Rounds != nil {
		rounds = strconv.Itoa(*t.Rounds)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "workflow: %s\nsource: %s\nrounds: %s\n", name, t.Source, rounds)
	for _, s := range t.Steps {
		if s.Agent != nil {
			fmt.Fprintf(&b, "step: %s (%s, %s)\n", s.Name, s.Kind, *s.Agent)
		} else {
			fmt.Fprintf(&b, "step: %s (%s)\n", s.Name, s.Kind)
		}
	}
	return b.String()
}
