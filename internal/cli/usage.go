package cli

import (
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/usage"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const usageHelp = `Usage: grid usage [--task T-NN] [--json]
       grid usage post --job J-N|lead [--dollars N] [--minutes N] [--session ID] [--model NAME]
       grid usage show
       grid usage threshold set [--dollars PERCENT ...] [--minutes PERCENT ...]
       grid usage threshold show | clear [--dollars] [--minutes]
       grid usage budget set [--dollars N] [--minutes N]
       grid usage budget show | clear [--dollars] [--minutes]
All commands accept --task T-NN and --json.
Flags alone fold the token totals observed by the session hooks; they are not Job usage posts.
Posts are deltas; at least one amount is required. Thresholds are percentages (0-100).
Default thresholds: 50%, 80%, 100%. No budget is set by default, so no notice fires.
Set replaces the supplied units. Clear without unit flags clears both units.
`

type usageNumbers struct {
	values []float64
	repeat bool
}

func (n *usageNumbers) String() string { return "" }
func (n *usageNumbers) Set(value string) error {
	if !n.repeat && len(n.values) > 0 {
		return fmt.Errorf("supply this amount only once")
	}
	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("expected a number: %w", err)
	}
	n.values = append(n.values, v)
	return nil
}

func (n *usageNumbers) amount() *float64 {
	if len(n.values) == 0 {
		return nil
	}
	return &n.values[0]
}

func runUsage(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(env.Stdout, usageHelp)
		return 0
	}
	if strings.HasPrefix(args[0], "-") {
		return runUsageObserved(args, env)
	}
	switch args[0] {
	case "threshold", "budget":
		return runUsageConfig(args[0], args[1:], env)
	case "post", "show":
	default:
		fmt.Fprintf(env.Stderr, "usage: unknown command %q\n", args[0])
		return 2
	}
	op := args[0]
	fs := newFlags("usage "+op, env)
	task := fs.String("task", "", "task folder or id; defaults to the current task")
	asJSON := fs.Bool("json", false, "print JSON")
	var dollars, minutes usageNumbers
	var in usage.Input
	if op == "post" {
		fs.StringVar(&in.Job, "job", "", "Job id or lead")
		fs.StringVar(&in.Session, "session", "", "optional session identifier")
		fs.StringVar(&in.Model, "model", "", "optional model name")
		fs.Var(&dollars, "dollars", "dollar delta (non-negative and finite)")
		fs.Var(&minutes, "minutes", "minute delta (non-negative and finite)")
	}
	if code, stop := parse(fs, args[1:], env); stop {
		return code
	}
	res, err := taskFor(env, *task)
	if err != nil {
		return commandError(env, "usage", err)
	}
	s := usage.Store{TaskPath: res.TaskPath, Writer: func() (feed.Writer, error) {
		if env.GridHome == "" {
			return feed.Writer{}, fmt.Errorf("cannot find the Grid home; set GRID_HOME")
		}
		w, err := feedWriter(env, "", "")
		w.Author.Role = "cli"
		return w, err
	}}
	if op == "show" {
		out, err := s.Show()
		if err != nil {
			return commandError(env, "usage", err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, out)
		}
		printUsage(env, out)
		return 0
	}
	in.Dollars, in.Minutes = dollars.amount(), minutes.amount()
	out, err := s.Post(in)
	if err != nil {
		return commandError(env, "usage", err)
	}
	for _, warning := range out.Warnings {
		fmt.Fprintf(env.Stderr, "usage: warning: %s\n", warning)
	}
	if *asJSON {
		return writeJSON(env.Stdout, out)
	}
	printUsage(env, out.Summary)
	for _, notice := range out.Notices {
		unit := "dollars"
		if _, ok := notice.Body["minutes"]; ok {
			unit = "minutes"
		}
		fmt.Fprintf(env.Stdout, "usage-notice %s  %g%% of %s budget  %g %s\n", notice.ID, notice.Body["threshold"], unit, notice.Body[unit], unit)
	}
	return 0
}

func printUsage(env Env, summary usage.Summary) {
	for _, row := range summary.Jobs {
		fmt.Fprintf(env.Stdout, "%s  dollars %g  minutes %g  posts %d\n", row.Job, row.Dollars, row.Minutes, row.Posts)
	}
	t := summary.Totals
	fmt.Fprintf(env.Stdout, "total  dollars %g  minutes %g  posts %d\n", t.Dollars, t.Minutes, t.Posts)
}

func runUsageConfig(kind string, args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(env.Stdout, usageHelp)
		return 0
	}
	op := args[0]
	if op != "set" && op != "show" && op != "clear" {
		fmt.Fprintf(env.Stderr, "usage %s: unknown command %q\n", kind, op)
		return 2
	}
	fs := newFlags("usage "+kind+" "+op, env)
	task := fs.String("task", "", "task folder or id; defaults to the current task")
	asJSON := fs.Bool("json", false, "print JSON")
	dollars, minutes := usageNumbers{repeat: kind == "threshold"}, usageNumbers{repeat: kind == "threshold"}
	var clearDollars, clearMinutes bool
	if op == "set" {
		label := "positive budget amount"
		if kind == "threshold" {
			label = "percentage of budget, 0-100; may be repeated"
		}
		fs.Var(&dollars, "dollars", label)
		fs.Var(&minutes, "minutes", label)
	} else if op == "clear" {
		fs.BoolVar(&clearDollars, "dollars", false, "clear dollars only")
		fs.BoolVar(&clearMinutes, "minutes", false, "clear minutes only")
	}
	if code, stop := parse(fs, args[1:], env); stop {
		return code
	}
	res, err := taskFor(env, *task)
	if err != nil {
		return commandError(env, "usage", err)
	}
	s := usage.Store{TaskPath: res.TaskPath}
	var config usage.Configuration
	if op == "show" {
		config, err = s.Config()
	} else {
		change := usage.Change{Budget: kind == "budget", Dollars: dollars.values, Minutes: minutes.values}
		if op == "clear" {
			// With no selectors, clear both. Explicit false is a usage error:
			// it must not silently turn an intended single-unit clear into both.
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "dollars" && !clearDollars || f.Name == "minutes" && !clearMinutes {
					err = fmt.Errorf("clear selectors cannot be false")
				}
			})
			if err != nil {
				return commandError(env, "usage", err)
			}
			if !clearDollars && !clearMinutes {
				clearDollars, clearMinutes = true, true
			}
			if clearDollars {
				change.Dollars = []float64{}
			}
			if clearMinutes {
				change.Minutes = []float64{}
			}
		}
		config, err = s.Configure(change)
	}
	if err != nil {
		return commandError(env, "usage", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, config)
	}
	for _, unit := range []string{"dollars", "minutes"} {
		budget := "none"
		if n, ok := config.Budget[unit]; ok {
			budget = strconv.FormatFloat(n, 'g', -1, 64)
		}
		values := config.Dollars
		if unit == "minutes" {
			values = config.Minutes
		}
		percentages := make([]string, 0, len(values))
		for _, n := range values {
			percentages = append(percentages, strconv.FormatFloat(n, 'g', -1, 64)+"%")
		}
		fmt.Fprintf(env.Stdout, "%s  budget %s  thresholds [%s]\n", unit, budget, strings.Join(percentages, ", "))
	}
	return 0
}

// runUsageObserved folds the token totals the session hooks recorded; see docs/lanes/L29.md.
func runUsageObserved(args []string, env Env) int {
	fs := newFlags("usage", env)
	id := fs.String("task", "", "task id or folder (otherwise the current task or home)")
	asJSON := fs.Bool("json", false, "print recorded totals as JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "Usage: grid usage [--task T-NN] [--json]")
		return 2
	}
	r, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *id})
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	dir := r.TaskPath
	if dir == "" {
		dir = env.GridHome
	}
	rows, err := (session.Store{TaskDir: dir}).Activities()
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	total := session.FoldUsage(rows)
	if *asJSON {
		return writeJSON(env.Stdout, total)
	}
	fmt.Fprintf(env.Stdout, "Recorded usage: %.0f input tokens, %.0f output tokens, %.0f cache read tokens, %.0f cache write tokens, %.0f total tokens, %.0f ms\n", total.Input, total.Output, total.CacheRead, total.CacheWrite, total.Total, total.Milliseconds)
	return 0
}
