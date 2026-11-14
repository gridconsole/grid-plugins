package cli

import (
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// d10OtFill lists each open task of project under the template it runs, in
// numeric id order. A task is open when it is readable and its state is not
// closed (the only terminal state in task.StateMoves, internal/task/write.go:36).
// A task whose template does not resolve, or that runs an inline list, is listed
// under no template; that is not an error of the read. A project the task store
// cannot list leaves every openTasks as it was.
func d10OtFill(env Env, project string, answer *workflow.Answer) {
	if project == "" {
		return
	}
	r := resolver(env)
	tasks, err := (task.Store{Resolver: r}).List(env.Cwd, project, "")
	if err != nil {
		return
	}
	for _, t := range tasks {
		if t.Unreadable || t.State == "closed" {
			continue
		}
		text, err := os.ReadFile(filepath.Join(t.Path, "task.md"))
		if err != nil {
			continue
		}
		tmpl, _, err := workflow.TaskWorkflow(r, t.Project, string(text))
		if err != nil || tmpl == nil {
			continue
		}
		for i := range answer.Templates {
			if answer.Templates[i].Name == tmpl.Name && answer.Templates[i].File == tmpl.File {
				answer.Templates[i].OpenTasks = append(answer.Templates[i].OpenTasks, t.ID)
				break
			}
		}
	}
}
