// Package evalcmd implements grid eval without depending on the CLI dispatcher.
package evalcmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gridconsole/grid-plugins/internal/eval"
)

type Env struct {
	Cwd      string
	GridHome string // "" when unknown; the default results folder then falls back to evals/runs under Cwd
	Stdout   io.Writer
	Stderr   io.Writer
}

// stringList collects a repeated flag.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, ",") }
func (l *stringList) Set(v string) error { *l = append(*l, v); return nil }

// Run returns 0 on success, 1 for failed checks or execution/storage errors,
// and 2 for invalid command usage. Reading a failed run with show returns 0.
func Run(args []string, env Env) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid eval list [flags]\n       grid eval run [task] [flags]\n       grid eval show <run-id> [flags]\n\nRun defaults to all tasks and the fake agent. Use --agent with a JSON argv array for a real agent; the agent sees only a minimal environment plus variables named with --env.")
		return 0
	}
	verb := args[0]
	if verb != "list" && verb != "run" && verb != "show" {
		fmt.Fprintf(env.Stderr, "eval: unknown command %q\n", verb)
		return 2
	}
	fs := flag.NewFlagSet("grid eval "+verb, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	asJSON := fs.Bool("json", false, "print JSON")
	var suite, results, tier, size, agent string
	var timeout time.Duration
	if verb != "show" {
		fs.StringVar(&suite, "suite", "evals/tasks", "task suite directory")
		fs.StringVar(&tier, "tier", "", "filter tier b or c")
		fs.StringVar(&size, "size", "", "filter tiny, small, normal or large tasks")
	}
	if verb != "list" {
		fs.StringVar(&results, "results", "", "result directory (default <grid home>/evals/runs)")
	}
	var allowEnv stringList
	if verb == "run" {
		fs.Var(&allowEnv, "env", "name of a login variable to pass to the agent, such as ANTHROPIC_API_KEY (repeatable)")
		fs.StringVar(&agent, "agent", "", `agent argv as JSON, for example '["claude","-p"]' (default: fake)`)
		fs.DurationVar(&timeout, "timeout", 5*time.Minute, "timeout per task, including setup and checks")
	}
	// Accept either task/id before flags or a final positional after flags.
	positional := ""
	rest := args[1:]
	if verb != "list" && len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		positional, rest = rest[0], rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if results == "" {
		results = filepath.Join("evals", "runs")
		if env.GridHome != "" {
			results = filepath.Join(env.GridHome, "evals", "runs")
		}
	}
	if fs.NArg() == 1 && positional == "" && verb != "list" {
		positional = fs.Arg(0)
	} else if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "eval: unexpected arguments")
		return 2
	}
	if verb == "show" {
		if positional == "" {
			fmt.Fprintln(env.Stderr, "eval show: a run id is required")
			return 2
		}
		result, err := eval.LoadResult(resolve(env.Cwd, results), positional)
		if err != nil {
			return fail(env, err)
		}
		if *asJSON {
			return writeJSON(env, result)
		}
		if err := eval.WriteSummary(env.Stdout, []eval.Result{result}); err != nil {
			return fail(env, err)
		}
		return showDetails(env, result)
	}
	if (tier != "" && tier != "b" && tier != "c") || (size != "" && size != "tiny" && size != "small" && size != "normal" && size != "large") {
		fmt.Fprintln(env.Stderr, "eval: invalid tier or size")
		return 2
	}
	var argv []string
	if verb == "run" {
		if timeout <= 0 {
			fmt.Fprintln(env.Stderr, "eval run: timeout must be positive")
			return 2
		}
		if err := eval.ValidateAllowEnv(allowEnv); err != nil {
			fmt.Fprintf(env.Stderr, "eval run: --env: %v\n", err)
			return 2
		}
		if agent != "" {
			if err := json.Unmarshal([]byte(agent), &argv); err != nil || len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
				fmt.Fprintln(env.Stderr, "eval run: --agent must be a nonempty JSON argv array")
				return 2
			}
		}
	}
	tasks, err := eval.LoadSuite(resolve(env.Cwd, suite))
	if err != nil {
		return fail(env, err)
	}
	selected := []eval.Task{}
	for _, task := range tasks {
		if (positional == "" || task.ID == positional) && (tier == "" || task.Tier == tier) && (size == "" || task.Size == size) {
			selected = append(selected, task)
		}
	}
	if verb == "list" {
		if *asJSON {
			return writeJSON(env, selected)
		}
		tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "TASK\tTIER\tSIZE\tCHECKS\tTITLE")
		for _, task := range selected {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", task.ID, task.Tier, task.Size, len(task.Checks), task.Title)
		}
		if err := tw.Flush(); err != nil {
			return fail(env, err)
		}
		return 0
	}
	if len(selected) == 0 {
		return fail(env, errors.New("no tasks match the selection"))
	}
	runner := eval.Runner{Agent: argv, ResultsDir: resolve(env.Cwd, results), Timeout: timeout, AllowEnv: allowEnv}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	records := []eval.Result{}
	code := 0
	for _, task := range selected {
		result, err := runner.Run(ctx, task)
		if err != nil {
			fail(env, fmt.Errorf("task %s: %w", task.ID, err))
			code = 1
		} else {
			records = append(records, result)
			if result.Status != "passed" {
				code = 1
			}
			if result.CleanupError != "" {
				fail(env, fmt.Errorf("task %s: %s", task.ID, result.CleanupError))
				code = 1
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	if *asJSON {
		if writeJSON(env, records) != 0 {
			return 1
		}
	} else if err := eval.WriteSummary(env.Stdout, records); err != nil {
		return fail(env, err)
	}
	return code
}

func resolve(cwd, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}

func writeJSON(env Env, value any) int {
	enc := json.NewEncoder(env.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return fail(env, err)
	}
	return 0
}

func fail(env Env, err error) int {
	fmt.Fprintf(env.Stderr, "eval: %v\n", err)
	return 1
}

func showDetails(env Env, result eval.Result) int {
	tw := tabwriter.NewWriter(env.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "\nCHECK\tTYPE\tSCORER\tSTATUS\tREASON\tEVIDENCE")
	for _, check := range result.Checks {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", check.ID, check.Type, check.Scorer, check.Status, check.Reason, check.Evidence)
	}
	if err := tw.Flush(); err != nil {
		return fail(env, err)
	}
	if result.CleanupError != "" {
		if _, err := fmt.Fprintf(env.Stdout, "cleanup: %s\n", result.CleanupError); err != nil {
			return fail(env, err)
		}
	}
	for _, phase := range []struct {
		name string
		data eval.ProcessResult
	}{{"setup", result.Setup}, {"agent", result.AgentResult}} {
		if _, err := fmt.Fprintf(env.Stdout, "%s: %s (exit %d, %dms)\n", phase.name, phase.data.Status, phase.data.ExitCode, phase.data.DurationMS); err != nil {
			return fail(env, err)
		}
		if phase.data.Error != "" {
			if _, err := fmt.Fprintln(env.Stdout, phase.data.Error); err != nil {
				return fail(env, err)
			}
		}
	}
	return 0
}
