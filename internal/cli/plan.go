package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const planUsage = "Usage: grid plan hash|items [--json] [--task <task>]\n       grid plan write <file> [--json] [--task <task>]\n       grid plan diff <rev-a> <rev-b> [--json] [--task <task>]"

func runPlan(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, planUsage)
		return 2
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, planUsage)
		return 0
	}
	action := args[0]
	want, ok := map[string]int{"hash": 0, "items": 0, "write": 1, "diff": 2}[action]
	if !ok {
		fmt.Fprintf(env.Stderr, "plan: unknown action %q; use hash, items, write or diff\n", action)
		return 2
	}
	fs := newFlags("plan "+action, env)
	fs.Usage = func() { fmt.Fprintln(env.Stderr, planUsage); fs.PrintDefaults() }
	task := fs.String("task", "", "task folder or id; otherwise use the current task")
	project := fs.String("project", "", "project name or id to disambiguate a task id")
	asJSON := fs.Bool("json", false, "print JSON")
	operands, err := planArguments(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(operands) != want {
		fmt.Fprintln(env.Stderr, planUsage)
		return 2
	}
	scope, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *task, Project: *project})
	if err != nil {
		return planError(env, err)
	}
	if scope.TaskPath == "" {
		return planError(env, errors.New("no current task; use --task <task folder or id>"))
	}
	store := plan.Store{TaskDir: scope.TaskPath}
	switch action {
	case "hash":
		text, err := store.Read()
		if err != nil {
			return planError(env, err)
		}
		hash := plan.Hash(text)
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Hash string `json:"hash"`
			}{hash})
		}
		_, err = fmt.Fprintln(env.Stdout, hash)
		if err != nil {
			return 1
		}
	case "items":
		items, err := store.Items()
		if err != nil {
			return planError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, items)
		}
		var out strings.Builder
		for _, item := range items {
			fmt.Fprintf(&out, "%s: %s: %s\n", item.ID, item.State, item.Title)
			for _, field := range []struct {
				name  string
				value []byte
			}{
				{"write", item.Write}, {"after", item.After}, {"review", item.Review}, {"risk", item.Risk}, {"verify", item.Verify},
			} {
				if field.value != nil {
					fmt.Fprintf(&out, "  %s: %s\n", field.name, field.value)
				}
			}
		}
		if len(items) == 0 {
			out.WriteString("No plan items.\n")
		}
		if _, err := fmt.Fprint(env.Stdout, out.String()); err != nil {
			return 1
		}
	case "write":
		file := operands[0]
		if !filepath.IsAbs(file) {
			file = filepath.Join(env.Cwd, file)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return planError(env, err)
		}
		written, err := store.Write(string(data))
		if err != nil {
			return planError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, written)
		}
		if _, err := fmt.Fprintf(env.Stdout, "plan.md: revision %d, hash %s\n", written.Revision, written.Hash); err != nil {
			return 1
		}
	case "diff":
		diff, err := store.Diff(operands[0], operands[1])
		if err != nil {
			return planError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, diff)
		}
		var out strings.Builder
		fmt.Fprintf(&out, "Plan revisions %d to %d\n", diff.From, diff.To)
		for _, item := range diff.Added {
			fmt.Fprintf(&out, "added: %s: %s\n", item.ID, item.Title)
		}
		for _, item := range diff.Removed {
			fmt.Fprintf(&out, "removed: %s: %s\n", item.ID, item.Title)
		}
		for _, change := range diff.Changed {
			fmt.Fprintf(&out, "changed: %s: %s (position %d to %d)\nbefore:\n%safter:\n%s", change.After.ID, change.After.Title, change.BeforePosition, change.AfterPosition, change.BeforeText, change.AfterText)
		}
		if len(diff.Added)+len(diff.Removed)+len(diff.Changed) == 0 {
			out.WriteString("No item changes.\n")
		}
		if _, err := fmt.Fprint(env.Stdout, out.String()); err != nil {
			return 1
		}
	}
	return 0
}

// Accept flags before or after operands, including between revision numbers.
// A literal -- ends option parsing, allowing a source filename starting with -.
func planArguments(fs *flag.FlagSet, args []string) ([]string, error) {
	var options, operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			operands = append(operands, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			operands = append(operands, arg)
			continue
		}
		options = append(options, arg)
		name := strings.TrimLeft(arg, "-")
		if (name == "task" || name == "project") && i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	return operands, fs.Parse(options)
}

func planError(env Env, err error) int {
	fmt.Fprintf(env.Stderr, "plan: %v\n", err)
	return 1
}
