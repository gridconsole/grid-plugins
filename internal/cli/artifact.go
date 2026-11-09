package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/artifact"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// parseMixed accepts flags before or after operands, as in artifact import file --json.
func parseMixed(fs *flag.FlagSet, args []string) ([]string, int, bool) {
	var flags, operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			operands = append(operands, arg)
			continue
		}
		flags = append(flags, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil {
			break
		}
		boolean, ok := f.Value.(interface{ IsBoolFlag() bool })
		if !hasValue && !(ok && boolean.IsBoolFlag()) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	if err := fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil, 0, true
		}
		return nil, 2, true
	}
	return operands, 0, false
}

func taskFor(env Env, task string) (where.Result, error) {
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: task})
	if err != nil {
		return res, err
	}
	if res.TaskPath == "" {
		return res, errors.New("run inside a task folder or supply --task")
	}
	return res, nil
}

func commandError(env Env, name string, err error) int {
	env.recordError(err)
	fmt.Fprintf(env.Stderr, "%s: %v\n", name, err)
	return 1
}

func runArtifact(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid artifact import <file> | link <https-url> | list | publish <id> [--task <task>] [--title <title>] [--json]")
		return 0
	}
	op := args[0]
	if op != "import" && op != "link" && op != "list" && op != "publish" {
		fmt.Fprintf(env.Stderr, "artifact: unknown command %q\n", op)
		return 2
	}
	fs := newFlags("artifact "+op, env)
	task := fs.String("task", "", "task folder or id; defaults to the current task")
	asJSON := fs.Bool("json", false, "print JSON")
	title := new(string)
	if op == "import" || op == "link" {
		title = fs.String("title", "", "artifact title")
	}
	operands, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	count := 1
	if op == "list" {
		count = 0
	}
	if len(operands) != count {
		fmt.Fprintf(env.Stderr, "artifact %s: expected %d operands\n", op, count)
		return 2
	}
	res, err := taskFor(env, *task)
	if err != nil {
		return commandError(env, "artifact", err)
	}
	store := artifact.Store{TaskPath: res.TaskPath, ProjectFolder: res.Folder}
	if op == "list" {
		records, err := store.List()
		if err != nil {
			return commandError(env, "artifact", err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, records)
		}
		for _, r := range records {
			printArtifact(env, r)
		}
		return 0
	}
	var r artifact.Record
	switch op {
	case "import":
		path := operands[0]
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
		r, err = store.Import(path, *title)
	case "link":
		r, err = store.Link(operands[0], *title)
	case "publish":
		r, err = store.Publish(operands[0])
	}
	if err != nil {
		return commandError(env, "artifact", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, r)
	}
	printArtifact(env, r)
	return 0
}

func printArtifact(env Env, r artifact.Record) {
	state := "unpublished"
	if r.Published {
		state = "published"
	}
	fmt.Fprintf(env.Stdout, "%s  %s  %s  %s\n", r.ID, r.Kind, r.Title, state)
}
