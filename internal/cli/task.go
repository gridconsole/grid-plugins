package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

const taskUsage = `Usage:
  grid task new "<words>" [--project <name-or-id>] [--size auto|small|normal|large] [--workflow <name>] [--json]
  grid task show [T-NN] [--json]
  grid task attach <T-NN> [--session ID] [--runtime claude] [--json]
  grid task list [--project <name-or-id>] [--state <state>] [--json]
  grid task move <machine> [--task <task>] [--project <project>] [--who <caller>] [--json]
  grid task stop|close|reopen <T-NN> [--reason <text>] [--json]
  grid task status <T-NN> <state> [--json]
  grid task history <T-NN> [--json]
`

func runTask(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(env.Stdout, taskUsage)
		return 0
	}
	sub := args[0]
	if sub == "move" {
		return runTaskIntent(args, env)
	}
	switch sub {
	case "attach":
		return runTaskAttach(args[1:], env)
	case "stop", "close", "reopen", "status", "history":
		return runTaskWrite(args, env)
	}
	if sub != "new" && sub != "show" && sub != "list" {
		fmt.Fprintf(env.Stderr, "grid task: unknown command %q\n%s", sub, taskUsage)
		return 2
	}
	fs := newFlags("task "+sub, env)
	asJSON := fs.Bool("json", false, "print JSON")
	var proj, size, workflow, state string
	if sub == "new" || sub == "list" {
		fs.StringVar(&proj, "project", "", "project name or id; personal selects Personal")
	}
	if sub == "new" {
		fs.StringVar(&size, "size", "auto", "auto, small, normal or large")
		fs.StringVar(&workflow, "workflow", "", "workflow name; omitted leaves selection to the rules")
	}
	if sub == "list" {
		fs.StringVar(&state, "state", "", "include only tasks in this state")
	}
	fs.Usage = func() {
		fmt.Fprint(env.Stderr, taskUsage)
		fs.PrintDefaults()
	}
	pos, err := taskArgs(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if (sub == "new" && len(pos) != 1) || (sub == "show" && len(pos) > 1) || (sub == "list" && len(pos) != 0) {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	store := task.Store{Resolver: resolver(env)}
	fail := func(err error) int {
		fmt.Fprintf(env.Stderr, "grid task %s: %v\n", sub, err)
		return 1
	}
	switch sub {
	case "new":
		// GRID_ACTOR is what the other task writers read, so a host that sets only it still names the owner.
		who := env.getenv("GRID_WHO")
		if who == "" {
			who = env.getenv("GRID_ACTOR")
		}
		o := task.NewOptions{Cwd: env.Cwd, Words: pos[0], Project: proj, Size: size, Workflow: workflow,
			Who: who, Role: env.getenv("GRID_ROLE")}
		if err := o.Validate(); err != nil {
			fmt.Fprintf(env.Stderr, "grid task new: %v\n", err)
			return 2
		}
		t, err := store.New(o)
		if err != nil {
			return fail(err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				ID   string `json:"id"`
				Path string `json:"path"`
			}{t.ID, t.Path})
		}
		fmt.Fprintf(env.Stdout, "%s created\n%s\n", t.ID, gridhome.Tilde(t.Path, env.Home))
	case "show":
		id := ""
		if len(pos) == 1 {
			id = pos[0]
		}
		t, err := store.Show(env.Cwd, id)
		if err != nil {
			return fail(err)
		}
		// The hash a person passes to grid approve --plan-hash; absent before a plan exists.
		hash, err := taskPlanHash(t.Path)
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid task show: no plan hash: %v\n", err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				task.Task
				PlanHash string `json:"plan_hash,omitempty"`
			}{t, hash})
		}
		fmt.Fprintf(env.Stdout, "id: %s\ntitle: %s\nproject: %s\nsize: %s\nworkflow: %s\nstate: %s\ncreated: %s\npath: %s\n",
			t.ID, t.Title, t.Project, t.Size, t.Workflow, t.State, t.Created, gridhome.Tilde(t.Path, env.Home))
		if hash != "" {
			fmt.Fprintf(env.Stdout, "plan_hash: %s\n", hash)
		}
		fmt.Fprintf(env.Stdout, "intent:\n%s", t.Intent)
		if !strings.HasSuffix(t.Intent, "\n") {
			fmt.Fprintln(env.Stdout)
		}
	case "list":
		tasks, err := store.List(env.Cwd, proj, state)
		if err != nil {
			return fail(err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, tasks)
		}
		for _, t := range tasks {
			fmt.Fprintf(env.Stdout, "%s %s %s\n", t.ID, t.State, t.Title)
		}
	}
	return 0
}

func runTaskAttach(args []string, env Env) int {
	fs := newFlags("task attach", env)
	id := fs.String("session", "", "session id; defaults to GRID_SESSION_ID, then CLAUDE_SESSION_ID, then a generated id")
	runtime := fs.String("runtime", "claude", "session runtime (claude)")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	if *runtime != "claude" {
		fmt.Fprintln(env.Stderr, "grid task attach: --runtime must be claude")
		return 2
	}
	if *id == "" {
		*id = env.getenv("GRID_SESSION_ID")
	}
	if *id == "" {
		*id = env.getenv("CLAUDE_SESSION_ID")
	}
	pid := os.Getppid()
	result, err := taskWriter(env).Attach(task.AttachOptions{
		Cwd: env.Cwd, ID: pos[0], SessionID: *id, Runtime: *runtime,
		PID: pid, PIDStart: migrate.ProcessStart(pid), Who: writeOptions(env, pos[0], "attach", "").Who,
		Agent: agentShell(env), BoundTask: env.getenv("GRID_TASK"),
	})
	if err != nil {
		return commandError(env, "grid task attach", err)
	}
	advice := env.Stdout
	if *asJSON {
		advice = env.Stderr
	}
	for _, holder := range result.Holders {
		fmt.Fprintf(advice, "Live holder: %s. Use grid lease take-over to take over; attach does not change leases.\n", holder)
	}
	if *asJSON {
		return writeJSON(env.Stdout, result)
	}
	fmt.Fprintf(env.Stdout, "id: %s\ntitle: %s\nstate: %s\napproved: %t\nopen_questions: %d\nAttached. %s %s\n",
		result.ID, result.Title, result.State, result.Approved, result.OpenQuestions, result.ID, result.Title)
	return 0
}

// taskPlanHash is the hash of plan.md as grid plan hash prints it, or "" when
// the task has no plan yet. Reading takes no lock: plan.md is replaced atomically.
func taskPlanHash(dir string) (string, error) {
	text, err := plan.Store{TaskDir: dir}.Read()
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return plan.Hash(text), nil
}

// taskArgs accepts flags before or after the single words/id argument. Values
// belonging to string flags stay attached; -- protects words starting with '-'.
func taskArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			pos = append(pos, arg)
			continue
		}
		flags = append(flags, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil || hasValue {
			continue // flag.Parse reports unknown flags and handles --help.
		}
		b, isBool := f.Value.(interface{ IsBoolFlag() bool })
		if (!isBool || !b.IsBoolFlag()) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return pos, fs.Parse(flags)
}
