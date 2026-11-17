package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/routine"
	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const routineLockHelp = "Runs hold the entry lock until execution finishes; another run, schedule or remove of that entry waits for it."

// routineTools collects the repeated --tool flag of grid routine add.
type routineTools []string

func (t *routineTools) String() string         { return strings.Join(*t, ", ") }
func (t *routineTools) Set(value string) error { *t = append(*t, value); return nil }

func runRoutine(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid routine add <name> --every <cron or duration> (--run <grid command or skill> | --workflow <workflow>) [--replace] [--project p]\n       grid routine list [--project p] [--json]\n       grid routine schedule|unschedule [<name>] [--project p] [--json]\n       grid routine remove|run <name> [--project p] [--json]")
		fmt.Fprintln(env.Stdout, "\n"+routineLockHelp+"\n"+routine.LimitsHelp())
		return 0
	}
	verb := args[0]
	fs := newFlags("routine "+verb, env)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: grid routine %s [options]\n\n%s\n", verb, routineLockHelp+"\n"+routine.LimitsHelp())
		fs.PrintDefaults()
	}
	helpToStdout(fs, args, env)
	project := fs.String("project", "", "project name or id; defaults to home")
	asJSON := fs.Bool("json", false, "print JSON")
	var every, run, workflow, homeOverride string
	var due, replace bool
	var timeout, turns, budget string
	var tools routineTools
	switch verb {
	case "add":
		fs.Var(&tools, "tool", "a tool of the mail, chat or calendar source a skill routine reads, by its full name; repeat for each; added to the routine's allow list")
		fs.StringVar(&timeout, "routine-timeout", "", "routine_timeout override; see policy defaults above")
		fs.StringVar(&turns, "routine-max-turns", "", "routine_max_turns override; see policy defaults above")
		fs.StringVar(&budget, "routine-budget", "", "routine_budget override; see policy defaults above")
		fs.StringVar(&every, "every", "", "duration of at least one minute or numeric five-field UTC cron")
		fs.StringVar(&run, "run", "", "grid command or skill; or give --workflow")
		fs.StringVar(&workflow, "workflow", "", "workflow each run starts a new task with, named after the routine; or give --run")
		fs.BoolVar(&replace, "replace", false, "rewrite the routine of this name, keeping last_run, outcome and last_task; schedule it again after")
	case "run":
		fs.BoolVar(&due, "due", false, "run only when due (used by OS timers)")
		fs.StringVar(&homeOverride, "grid-home", "", "Grid home for Task Scheduler")
	case "list", "remove", "schedule", "unschedule":
	default:
		fmt.Fprintf(env.Stderr, "grid routine: unknown command %q\n", verb)
		return 2
	}
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	count := 0
	if verb == "add" || verb == "remove" || verb == "run" {
		count = 1
	}
	// schedule and unschedule act on one routine when given its name, else on all.
	named := (verb == "schedule" || verb == "unschedule") && fs.NArg() == 1
	if named {
		count = 1
	}
	if fs.NArg() != count {
		if verb == "schedule" || verb == "unschedule" {
			fmt.Fprintf(env.Stderr, "grid routine %s: expected at most 1 argument, a routine name\n", verb)
		} else {
			fmt.Fprintf(env.Stderr, "grid routine %s: expected %d arguments\n", verb, count)
		}
		return 2
	}
	if verb == "add" {
		// --run and --workflow are exclusive and one is required. A missing --every
		// is left to the store's interval error, exit 1, as before workflows.
		hasRun, hasWorkflow := strings.TrimSpace(run) != "", strings.TrimSpace(workflow) != ""
		if hasRun && hasWorkflow {
			fmt.Fprintln(env.Stderr, "grid routine add: give --run or --workflow, not both")
			return 2
		}
		if !hasRun && !hasWorkflow && strings.TrimSpace(every) != "" {
			fmt.Fprintln(env.Stderr, "grid routine add: --run or --workflow is required")
			return 2
		}
	}
	if verb != "list" && resolver(env).Caller.IsGuest() {
		return failure(env, ilGuestRefused)
	}
	if homeOverride != "" {
		if !filepath.IsAbs(homeOverride) {
			return failure(env, fmt.Errorf("--grid-home must be absolute"))
		}
		env.GridHome = gridhome.Real(homeOverride)
		if msg := formatRefusal(env); msg != "" {
			return failure(env, errors.New(msg))
		}
	}
	if *project != "" {
		res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: *project})
		if err != nil {
			return routineError(env, err)
		}
		if !res.Record {
			return failure(env, fmt.Errorf("project is not registered; run grid project add"))
		}
		*project = res.Project
	}
	store := routine.Store{GridHome: env.GridHome, Project: *project}
	switch verb {
	case "list":
		entries, err := store.List()
		if err != nil {
			return routineError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, entries)
		}
		for _, entry := range entries {
			printRoutine(env, entry)
			next := "none"
			if entry.Next != nil {
				next = entry.Next.UTC().Format(time.RFC3339)
			}
			fmt.Fprintf(env.Stdout, "scheduled: %t\nnext: %s\n", entry.Scheduled, next)
		}
		return 0
	case "add":
		var policy routine.LimitPolicy
		var policyErr error
		fs.Visit(func(f *flag.Flag) {
			var key, value string
			switch f.Name {
			case "routine-timeout":
				key, value = "routine_timeout", timeout
			case "routine-max-turns":
				key, value = "routine_max_turns", turns
			case "routine-budget":
				key, value = "routine_budget", budget
			default:
				return
			}
			if policyErr == nil {
				policyErr = policy.SetText(key, value)
			}
		})
		if policyErr != nil {
			return routineError(env, policyErr)
		}
		entry, err := store.AddWith(fs.Arg(0), every, run, routine.AddOptions{Workflow: workflow, Replace: replace, Policy: policy, Tools: tools})
		if err != nil {
			return routineError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, entry)
		}
		printRoutine(env, entry)
		return 0
	case "run":
		for _, warning := range store.LegacyLimitWarnings() {
			fmt.Fprintf(env.Stderr, "grid: warning: %s\n", warning)
		}
		binary, err := env.executable()
		if err != nil {
			return routineError(env, err)
		}
		// Child output goes to stderr so --json remains exactly one JSON value.
		output := env.Stdout
		if *asJSON {
			output = env.Stderr
		}
		executor := routine.ProcessExecutor{GridBinary: binary, Home: env.Home, Getenv: env.getenv, Stdout: output, Stderr: env.Stderr}
		result, err := store.Execute(context.Background(), fs.Arg(0), due, executor, routineFailureSignal(env, store.Project))
		if *asJSON && result.Name != "" {
			if code := writeJSON(env.Stdout, result); code != 0 {
				return code
			}
		}
		if err != nil {
			return routineError(env, err)
		}
		if !*asJSON {
			if result.Skipped {
				fmt.Fprintln(env.Stdout, "skipped: not due")
			} else {
				printRoutine(env, result.Entry)
			}
		}
		return 0
	default:
		binary, err := env.executable()
		if err != nil {
			return routineError(env, err)
		}
		name := ""
		if named {
			name = fs.Arg(0)
		}
		// Timers start the launcher in the Grid home, which survives a plugin update, when it is there.
		if env.GridHome != "" {
			// Stat follows the link: one whose target an update removed falls back to the executable.
			// The machine folder's launcher comes first, then the older ~/.grid/bin, when it is there.
			launcher := routine.LauncherPath(gridhome.FindLauncherDir(env.getenv, env.Home, env.GridHome))
			if info, statErr := os.Stat(launcher); statErr == nil && info.Mode().IsRegular() {
				binary = launcher
			}
		}
		scheduler := routine.Scheduler{Home: env.Home, GridHome: env.GridHome, Binary: binary, Path: env.getenv("PATH"), Backend: routine.NativeBackend(), Runner: routine.ExecOSRunner{}}
		if routine.NativePreviewOnly() && (verb == "schedule" || verb == "unschedule") {
			var plans []routine.TimerPlan
			if named {
				plans, err = scheduler.PreviewOne(store, name, verb == "unschedule")
			} else {
				plans, err = scheduler.Preview(store, verb == "unschedule")
			}
			if err != nil {
				return routineError(env, err)
			}
			if *asJSON {
				return writeJSON(env.Stdout, struct {
					Result string              `json:"result"`
					Timers []routine.TimerPlan `json:"timers"`
				}{"preview", plans})
			}
			fmt.Fprintln(env.Stdout, "result: preview (native Windows file locking is unavailable)")
			for _, plan := range plans {
				for _, file := range plan.Files {
					fmt.Fprintf(env.Stdout, "file: %s\n", file.Path)
					if file.Encoding != "" {
						fmt.Fprintf(env.Stdout, "encoding: %s with BOM\n", file.Encoding)
					}
					fmt.Fprint(env.Stdout, file.Content)
				}
				for _, command := range append(plan.Install, plan.Remove...) {
					fmt.Fprintf(env.Stdout, "would run: %s %q\n", command.Program, command.Args)
				}
			}
			return 0
		}

		switch verb {
		case "schedule":
			if named {
				err = scheduler.ScheduleOne(context.Background(), store, name)
			} else {
				err = scheduler.Schedule(context.Background(), store)
			}
		case "unschedule":
			err = scheduler.Unschedule(context.Background(), store.Project, name)
		case "remove":
			// Unregister before deleting config so failed OS removal is retryable.
			err = scheduler.RemoveRoutine(context.Background(), store, fs.Arg(0))
		}
		if err != nil {
			return routineError(env, err)
		}
		// Name is set only when schedule or unschedule was given one; without it the
		// reply keeps its two keys byte for byte.
		result := struct {
			Result  string `json:"result"`
			Project string `json:"project"`
			Name    string `json:"name,omitempty"`
		}{Result: verb, Project: store.Project, Name: name}
		if *asJSON {
			return writeJSON(env.Stdout, result)
		}
		fmt.Fprintf(env.Stdout, "result: %s\n", verb)
		return 0
	}
}

func printRoutine(env Env, e routine.Entry) {
	fmt.Fprintf(env.Stdout, "name: %s\nevery: %s\n", e.Name, e.Every)
	if e.Workflow == "" {
		fmt.Fprintf(env.Stdout, "run: %s\n", e.Run)
	} else {
		fmt.Fprintf(env.Stdout, "workflow: %s\n", e.Workflow)
	}
	if len(e.Tools) > 0 {
		fmt.Fprintf(env.Stdout, "tools: %s\n", strings.Join(e.Tools, ", "))
	}
	fmt.Fprintf(env.Stdout, "machine: %s\n", e.Machine)
	if !e.LastRun.IsZero() {
		fmt.Fprintf(env.Stdout, "last_run: %s\noutcome: %s\n", e.LastRun.Format(time.RFC3339), e.Outcome)
	}
	if e.LastTask != "" {
		fmt.Fprintf(env.Stdout, "last_task: %s\n", e.LastTask)
	}
}

func routineFailureSignal(env Env, project string) routine.FailureSignal {
	return func(entry routine.Entry, runErr error) error {
		// Grid's own data folder resolves to Personal. Use the existing project and
		// signal writers; routine failures never manufacture task or Feed records.
		ps := projectStore(env)
		if _, err := ps.Add(env.GridHome, ""); err != nil {
			return fmt.Errorf("record routine failure signal: %w", err)
		}
		body := "Routine " + entry.Name + " failed. Run grid routine list to inspect its outcome."
		// SourceID is an existing deduplication key, not a record timestamp. Keep
		// its original RFC3339Nano bytes so pre-L42 signals still match retries.
		sourceID := project + "/" + entry.Name + ":" + entry.LastRun.Format(time.RFC3339Nano)
		if errors.Is(runErr, routine.ErrScheduleChanged) {
			body = runErr.Error()
			// A refusal leaves last_run alone; without its own id it would fold into
			// the note of the failed run that last_run names, hiding the instruction.
			sourceID += ":changed"
		}
		_, err := (signal.Store{GridHome: env.GridHome, Project: gridhome.Personal, Resolver: projectStore(env).Resolver}).Add(signal.Input{
			Source: "routine", SourceID: sourceID, Title: "Routine failed: " + entry.Name, Body: body,
		})
		return err
	}
}

func routineError(env Env, err error) int {
	var policyErr *routine.WorkflowPolicyError
	if errors.As(err, &policyErr) {
		fmt.Fprintf(env.Stderr, "grid: %s\n", policyErr)
		return 2
	}
	var limitErr *routine.LimitError
	if errors.As(err, &limitErr) {
		fmt.Fprintf(env.Stderr, "grid: %s\n", limitErr)
		return 2
	}
	return failure(env, err)
}
