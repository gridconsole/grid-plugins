package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/routine"
	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runRoutine(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid routine add <name> --every <cron or duration> --run <grid command or skill> [--project p]\n       grid routine list|schedule|unschedule [--project p] [--json]\n       grid routine remove|run <name> [--project p] [--json]")
		return 0
	}
	verb := args[0]
	fs := newFlags("routine "+verb, env)
	project := fs.String("project", "", "project name or id; defaults to home")
	asJSON := fs.Bool("json", false, "print JSON")
	var every, run, homeOverride string
	var due bool
	switch verb {
	case "add":
		fs.StringVar(&every, "every", "", "duration of at least one minute or numeric five-field UTC cron")
		fs.StringVar(&run, "run", "", "grid command or skill")
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
	if fs.NArg() != count {
		fmt.Fprintf(env.Stderr, "grid routine %s: expected %d arguments\n", verb, count)
		return 2
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
			return failure(env, err)
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
			return failure(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, entries)
		}
		for _, entry := range entries {
			printRoutine(env, entry)
		}
		return 0
	case "add":
		entry, err := store.Add(fs.Arg(0), every, run)
		if err != nil {
			return failure(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, entry)
		}
		printRoutine(env, entry)
		return 0
	case "run":
		binary, err := env.executable()
		if err != nil {
			return failure(env, err)
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
			return failure(env, err)
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
			return failure(env, err)
		}
		scheduler := routine.Scheduler{Home: env.Home, GridHome: env.GridHome, Binary: binary, Path: env.getenv("PATH"), Backend: routine.NativeBackend(), Runner: routine.ExecOSRunner{}}
		if routine.NativePreviewOnly() && (verb == "schedule" || verb == "unschedule") {
			plans, err := scheduler.Preview(store, verb == "unschedule")
			if err != nil {
				return failure(env, err)
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
					fmt.Fprintf(env.Stdout, "file: %s\n%s", file.Path, file.Content)
				}
				for _, command := range append(plan.Install, plan.Remove...) {
					fmt.Fprintf(env.Stdout, "would run: %s %q\n", command.Program, command.Args)
				}
			}
			return 0
		}

		switch verb {
		case "schedule":
			err = scheduler.Schedule(context.Background(), store)
		case "unschedule":
			err = scheduler.Unschedule(context.Background(), store.Project, "")
		case "remove":
			// Unregister before deleting config so failed OS removal is retryable.
			err = scheduler.RemoveRoutine(context.Background(), store, fs.Arg(0))
		}
		if err != nil {
			return failure(env, err)
		}
		result := struct {
			Result  string `json:"result"`
			Project string `json:"project"`
		}{Result: verb, Project: store.Project}
		if *asJSON {
			return writeJSON(env.Stdout, result)
		}
		fmt.Fprintf(env.Stdout, "result: %s\n", verb)
		return 0
	}
}

func printRoutine(env Env, e routine.Entry) {
	fmt.Fprintf(env.Stdout, "name: %s\nevery: %s\nrun: %s\nmachine: %s\n", e.Name, e.Every, e.Run, e.Machine)
	if !e.LastRun.IsZero() {
		fmt.Fprintf(env.Stdout, "last_run: %s\noutcome: %s\n", e.LastRun.Format(time.RFC3339), e.Outcome)
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
		_, err := (signal.Store{GridHome: env.GridHome, Project: gridhome.Personal}).Add(signal.Input{
			Source: "routine", SourceID: project + "/" + entry.Name + ":" + entry.LastRun.Format(time.RFC3339Nano), Title: "Routine failed: " + entry.Name, Body: body,
		})
		return err
	}
}
