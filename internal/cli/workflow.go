package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

func runWorkflow(args []string, env Env) int {
	const usage = "usage: grid workflow show [--project <name-or-id>] [--json]"
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(env.Stdout, usage)
		fmt.Fprintln(env.Stdout, workflowWriteUsage)
		return 0
	}
	if len(args) > 0 && args[0] == "write" {
		return runWorkflowWrite(args[1:], env)
	}
	if len(args) == 0 || args[0] != "show" {
		fmt.Fprintln(env.Stderr, usage)
		fmt.Fprintln(env.Stderr, workflowWriteUsage)
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
		if errors.Is(err, localdata.ErrUnsafePath) {
			fmt.Fprintln(env.Stderr, "grid workflow show: workflow path is unsafe or inaccessible")
			return 2
		}
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

const workflowWriteUsage = "usage: grid workflow write [--project <name-or-id>] --rev <12-hex|none> --file <path|-> [--json]"

func runWorkflowWrite(args []string, env Env) int {
	fs := newFlags("workflow write", env)
	// Flag errors may quote paths supplied as arguments. Use fixed diagnostics;
	// help has a separate, complete usage line.
	fs.SetOutput(io.Discard)
	project := fs.String("project", "", "project name or ID")
	rev := fs.String("rev", "", "expected revision or none")
	file := fs.String("file", "", "input path or - for stdin")
	asJSON := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(env.Stdout, workflowWriteUsage)
			return 0
		}
		fmt.Fprintln(env.Stderr, workflowWriteUsage)
		return 2
	}
	emptyProject := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "project" && *project == "" {
			emptyProject = true
		}
	})
	if fs.NArg() != 0 || emptyProject || *file == "" || !workflow.ValidWriteRev(*rev) {
		fmt.Fprintln(env.Stderr, workflowWriteUsage)
		return 2
	}
	if !workflow.WriteDefaultAllowed(env.getenv("GRID_ROLE")) {
		fmt.Fprintln(env.Stderr, "grid workflow write: caller role is not allowed to write workflows")
		return 3
	}
	input := env.Stdin
	if *file != "-" {
		path := *file
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(env.Stderr, "grid workflow write: cannot open input file")
			return exitCode(err)
		}
		defer f.Close()
		input = f
	}
	answer, err := workflow.Write(resolver(env), *project, *rev, input)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid workflow write: %v\n", err)
		switch {
		case errors.Is(err, workflow.ErrChanged):
			return 3
		case errors.Is(err, workflow.ErrInvalid):
			return 2
		default:
			return exitCode(err)
		}
	}
	if *asJSON {
		return writeJSON(env.Stdout, answer)
	}
	_, err = fmt.Fprintf(env.Stdout, "%s %s\n", answer.File, answer.Rev)
	returnCode := 0
	if err != nil {
		returnCode = exitCode(err)
	}
	return returnCode
}
