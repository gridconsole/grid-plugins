package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const planUsage = "Usage: grid plan check [T-NN] [--json]\n       grid plan hash|items [--json] [--task <task>]\n       grid plan write <file> [--json] [--task <task>]\n       grid plan migrate [--task <task>] [--dry-run] [--yes] [--json]"

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
	want, ok := map[string]int{"check": 0, "hash": 0, "items": 0, "write": 1, "migrate": 0}[action]
	if !ok {
		fmt.Fprintf(env.Stderr, "plan: unknown action %q; use check, hash, items, write or migrate\n", action)
		return 2
	}
	fs := newFlags("plan "+action, env)
	fs.Usage = func() { fmt.Fprintln(env.Stderr, planUsage); fs.PrintDefaults() }
	task := fs.String("task", "", "task folder or id; otherwise use the current task")
	project := fs.String("project", "", "project name or id to disambiguate a task id")
	asJSON := fs.Bool("json", false, "print JSON")
	var dryRun, yes bool
	if action == "migrate" {
		fs.BoolVar(&dryRun, "dry-run", false, "preview conversion without writing")
		fs.BoolVar(&yes, "yes", false, "allow migration to invalidate the current approval")
	}
	operands, err := planArguments(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if action == "check" && len(operands) == 1 && *task == "" {
		*task = operands[0]
		operands = nil
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
	case "migrate":
		return runPlanMigrate(store, dryRun, yes, *asJSON, env)
	case "check":
		text, err := store.Read()
		if err != nil {
			return planError(env, err)
		}
		return runPlanCheck(text, *asJSON, env)
	case "hash":
		text, err := store.Read()
		if err != nil {
			return planError(env, err)
		}
		hash := plan.Hash(text)
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Hash   string `json:"hash"`
				Digest string `json:"digest"`
			}{hash, plan.Digest(text)})
		}
		_, err = fmt.Fprintln(env.Stdout, hash)
		if err != nil {
			return 1
		}
	case "items":
		items, err := store.Items()
		if err != nil {
			text, _ := store.Read()
			return planError(env, planFormatError(text, err))
		}
		if *asJSON {
			return writeJSON(env.Stdout, items)
		}
		var out strings.Builder
		for _, item := range items {
			fmt.Fprintf(&out, "%s: %s: %s\n", item.ID, item.State, item.Title)
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
			return planError(env, planFormatError(string(data), err))
		}
		if *asJSON {
			return writeJSON(env.Stdout, written)
		}
		if _, err := fmt.Fprintf(env.Stdout, "plan.md: revision %d, hash %s\n", written.Revision, written.Hash); err != nil {
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

// planError prints the failure. A lookup refusal (an unregistered project, an
// unknown task) is recorded so --json also gets the error envelope; any other
// plan failure keeps its stderr line and empty stdout.
func planError(env Env, err error) int {
	if errorCode(err) != "" {
		env.recordError(err)
	}
	fmt.Fprintf(env.Stderr, "plan: %v\n", err)
	return 1
}

// Keep migration advice in the CLI; Items remains the strict shared parser.
func planFormatError(text string, err error) error {
	message := err.Error()
	if !strings.Contains(message, "front matter is refused") && !strings.Contains(message, "step-level") && !strings.Contains(message, "missing jobs list") {
		return err
	}
	if _, form, upgradeErr := plan.Upgrade(text); upgradeErr == nil && form != "current" {
		return fmt.Errorf("%w; this looks like an older plan format; run grid plan migrate --dry-run", err)
	}
	return err
}

func runPlanMigrate(store plan.Store, dryRun, yes, asJSON bool, env Env) int {
	var converted, form string
	var written plan.Written
	var err error
	if dryRun {
		var text string
		text, err = store.Read()
		if err == nil {
			converted, form, err = plan.Upgrade(text)
		}
	} else {
		// The plan Job's result tells whether the installed plan is approved; task
		// imports plan, so the CLI hands the approved hash in.
		approved := func() string {
			a, _ := taskstore.ApprovalFromJob(store.TaskDir)
			return a.PlanHash
		}
		written, form, err = store.Migrate(yes, approved)
	}
	if err != nil {
		return planError(env, err)
	}
	if form == "current" {
		message := "plan.md is already current"
		if asJSON {
			return writeJSON(env.Stdout, struct {
				Form    string `json:"form"`
				Message string `json:"message"`
			}{form, message})
		}
		if _, err := fmt.Fprintln(env.Stdout, message); err != nil {
			return 1
		}
		return 0
	}
	if dryRun {
		if asJSON {
			return writeJSON(env.Stdout, struct {
				Form string `json:"form"`
				Text string `json:"text"`
			}{form, converted})
		}
		if _, err := fmt.Fprintf(env.Stdout, "form: %s\n%s", form, converted); err != nil {
			return 1
		}
		return 0
	}

	if asJSON {
		return writeJSON(env.Stdout, struct {
			Form string `json:"form"`
			plan.Written
		}{form, written})
	}
	if _, err := fmt.Fprintf(env.Stdout, "plan.md: migrated form %s, revision %d, hash %s\n", form, written.Revision, written.Hash); err != nil {
		return 1
	}
	return 0
}
