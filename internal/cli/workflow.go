package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

func runWorkflow(args []string, env Env) int {
	const usage = "usage: grid workflow show [--project <name-or-id>] [--json]"
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(env.Stdout, usage)
		return 0
	}
	if len(args) == 0 || args[0] != "show" {
		fmt.Fprintln(env.Stderr, usage)
		return 2
	}
	fs := newFlags("workflow show", env)
	project := fs.String("project", "", "project name or ID")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, usage)
		return 2
	}
	emptyProject := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "project" && *project == "" {
			emptyProject = true
		}
	})
	if emptyProject {
		fmt.Fprintln(env.Stderr, "grid workflow show: --project requires a name or ID")
		return 2
	}
	answer, err := workflow.Read(env.GridHome, *project)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid: %v\n", err)
		return exitCode(err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, answer)
	}
	for _, file := range answer.Files {
		label := "workflows.md"
		if file.Project != nil {
			label = *file.Project + "/" + label
		}
		names := []string{}
		for _, template := range answer.Templates {
			if template.File == label {
				names = append(names, template.Name)
			}
		}
		count := 0
		for _, problem := range answer.Errors {
			if problem.File == label {
				count++
			}
		}
		listed := "(none)"
		if len(names) > 0 {
			listed = strings.Join(names, ", ")
		}
		if _, err := fmt.Fprintf(env.Stdout, "file: %s\ntemplates: %s\nerrors: %d\n", label, listed, count); err != nil {
			return exitCode(err)
		}
	}
	return 0
}
