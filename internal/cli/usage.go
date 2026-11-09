package cli

import (
	"errors"
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
       grid usage post --job J-N|lead [--dollars N] [--minutes N] [--tokens N] [--session ID] [--model NAME]
       grid usage show
       grid usage threshold set [--dollars PERCENT ...] [--minutes PERCENT ...] [--tokens PERCENT ...]
       grid usage threshold show | clear [--dollars] [--minutes] [--tokens]
       grid usage budget set [--dollars N] [--minutes N] [--tokens N]
       grid usage budget show | clear [--dollars] [--minutes] [--tokens]
All commands accept --json; task-scoped commands accept --task T-NN.
Flags alone fold the token totals observed by the session hooks; they are not Job usage posts.
Posts are deltas; at least one amount is required. Thresholds are percentages (0-100).
Units are dollars, minutes and tokens; token amounts are non-negative integers.
Default thresholds: tokens 80%, 100%; dollars and minutes 50%, 80%, 100%.
Per unit: a threshold set on the task wins, then the workflow's usage thresholds, then the default.
A task token budget overrides the workspace
default tokens_per_task in <GRID_HOME>/workspace.md front matter (positive digits).
Without a budget no notice fires. The hold at the limit is not enforced yet;
worker and session hooks will enforce it in a follow-up.
Set replaces the supplied units. Clear without unit flags clears all three units.
Clearing a task token budget restores the workspace default when present.
Budget show|set|clear --workspace accesses that default without a task.
Workspace set requires --tokens N (integer, at least 1); clear [--tokens] removes it.
Workspace mode rejects --task, --dollars and --minutes. Show prints tokens_per_task.
Workspace reads allow the local role, owner and member; writes allow local or owner.
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

// usageTokens parses before task resolution or any storage writes.
type usageTokens struct{ values []uint64 }

func (n *usageTokens) String() string { return "" }
func (n *usageTokens) Set(value string) error {
	if len(n.values) != 0 {
		return fmt.Errorf("supply this amount only once")
	}
	v, err := usage.ParseTokens(value)
	if err != nil {
		return err
	}
	n.values = append(n.values, v)
	return nil
}
func (n *usageTokens) amount() *uint64 {
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
	var tokens usageTokens
	var in usage.Input
	if op == "post" {
		fs.StringVar(&in.Job, "job", "", "Job id or lead")
		fs.StringVar(&in.Session, "session", "", "optional session identifier")
		fs.StringVar(&in.Model, "model", "", "optional model name")
		fs.Var(&dollars, "dollars", "dollar delta (non-negative and finite)")
		fs.Var(&minutes, "minutes", "minute delta (non-negative and finite)")
		fs.Var(&tokens, "tokens", "token delta (non-negative integer)")
	}
	if code, stop := parse(fs, args[1:], env); stop {
		return code
	}
	res, err := taskFor(env, *task)
	if err != nil {
		return commandError(env, "usage", err)
	}
	s := usage.Store{TaskPath: res.TaskPath, GridHome: env.GridHome, WorkflowWarning: usageWorkflowWarning(env), Writer: func() (feed.Writer, error) {
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
	in.Dollars, in.Minutes, in.Tokens = dollars.amount(), minutes.amount(), tokens.amount()
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
		if _, ok := notice.Body["tokens"]; ok {
			unit = "tokens"
		}
		fmt.Fprintf(env.Stdout, "usage-notice %s  %g%% of %s budget  %v %s\n", notice.ID, notice.Body["threshold"], unit, notice.Body[unit], unit)
	}
	return 0
}

func usageWorkflowWarning(env Env) func(error) {
	return func(err error) {
		fmt.Fprintf(env.Stderr, "usage: warning: %s\n", err)
	}
}

func printUsage(env Env, summary usage.Summary) {
	for _, row := range summary.Jobs {
		fmt.Fprintf(env.Stdout, "%s  dollars %g  minutes %g  tokens %d  posts %d\n", row.Job, row.Dollars, row.Minutes, row.Tokens, row.Posts)
	}
	t := summary.Totals
	fmt.Fprintf(env.Stdout, "total  dollars %g  minutes %g  tokens %d  posts %d\n", t.Dollars, t.Minutes, t.Tokens, t.Posts)
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
	var workspace bool
	if kind == "budget" {
		fs.BoolVar(&workspace, "workspace", false, "show or change the workspace token budget without resolving a task")
	}
	dollars, minutes := usageNumbers{repeat: kind == "threshold"}, usageNumbers{repeat: kind == "threshold"}
	var tokenBudget usageTokens
	tokenThresholds := usageNumbers{repeat: true}
	var clearDollars, clearMinutes, clearTokens bool
	if op == "set" {
		label := "positive budget amount"
		if kind == "threshold" {
			label = "percentage of budget, 0-100; may be repeated"
		}
		fs.Var(&dollars, "dollars", label)
		fs.Var(&minutes, "minutes", label)
		if kind == "budget" {
			fs.Var(&tokenBudget, "tokens", "positive integer token budget")
		} else {
			fs.Var(&tokenThresholds, "tokens", label)
		}
	} else if op == "clear" {
		fs.BoolVar(&clearDollars, "dollars", false, "clear dollars only")
		fs.BoolVar(&clearMinutes, "minutes", false, "clear minutes only")
		fs.BoolVar(&clearTokens, "tokens", false, "clear tokens only; budget clear restores the workspace default")
	}
	if code, stop := parse(fs, args[1:], env); stop {
		return code
	}
	if workspace {
		invalid := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "task" || f.Name == "dollars" || f.Name == "minutes" || op == "clear" && f.Name == "tokens" && !clearTokens {
				invalid = true
			}
		})
		if invalid || op == "set" && (tokenBudget.amount() == nil || *tokenBudget.amount() == 0) {
			fmt.Fprintln(env.Stderr, "usage budget: --workspace accepts only tokens; set requires --tokens of at least 1")
			return 2
		}
		return runWorkspaceBudget(op, tokenBudget.amount(), *asJSON, env)
	}
	res, err := taskFor(env, *task)
	if err != nil {
		return commandError(env, "usage", err)
	}
	s := usage.Store{TaskPath: res.TaskPath, GridHome: env.GridHome, WorkflowWarning: usageWorkflowWarning(env)}
	var config usage.Configuration
	if op == "show" {
		config, err = s.Config()
	} else {
		change := usage.Change{Budget: kind == "budget", Dollars: dollars.values, Minutes: minutes.values, Tokens: tokenThresholds.values, BudgetTokens: tokenBudget.values}
		if op == "clear" {
			// With no selectors, clear all units. Explicit false is a usage error:
			// it must not silently turn an intended single-unit clear into all units.
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "dollars" && !clearDollars || f.Name == "minutes" && !clearMinutes || f.Name == "tokens" && !clearTokens {
					err = fmt.Errorf("clear selectors cannot be false")
				}
			})
			if err != nil {
				return commandError(env, "usage", err)
			}
			if !clearDollars && !clearMinutes && !clearTokens {
				clearDollars, clearMinutes, clearTokens = true, true, true
			}
			if clearDollars {
				change.Dollars = []float64{}
			}
			if clearTokens {
				if change.Budget {
					change.BudgetTokens = []uint64{}
				} else {
					change.Tokens = []float64{}
				}
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
	for _, unit := range []string{"dollars", "minutes", "tokens"} {
		budget := "none"
		if n, ok := config.Budget[unit]; ok {
			budget = string(n)
		}
		values := config.Dollars
		if unit == "minutes" {
			values = config.Minutes
		}
		if unit == "tokens" {
			values = config.Tokens
		}
		percentages := make([]string, 0, len(values))
		for _, n := range values {
			percentages = append(percentages, strconv.FormatFloat(n, 'g', -1, 64)+"%")
		}
		source := ""
		if unit == "tokens" {
			source = "  source " + config.TokenBudgetSource
		}
		fmt.Fprintf(env.Stdout, "%s  budget %s  thresholds [%s]%s\n", unit, budget, strings.Join(percentages, ", "), source)
	}
	return 0
}

func runWorkspaceBudget(op string, value *uint64, asJSON bool, env Env) int {
	allowed := usage.WorkspaceBudgetReadAllowed
	if op != "show" {
		allowed = usage.WorkspaceBudgetWriteAllowed
	}
	if !allowed(env.getenv("GRID_ROLE")) {
		fmt.Fprintln(env.Stderr, "usage budget: caller role is not allowed to access the workspace budget")
		return 3
	}
	s := usage.Store{GridHome: env.GridHome}
	if op == "show" {
		value = s.WorkspaceTokens()
	} else if err := s.SetWorkspaceTokens(value); err != nil {
		fmt.Fprintf(env.Stderr, "usage budget: %v\n", err)
		if errors.Is(err, usage.ErrWorkspaceTokensInput) {
			return 2
		}
		return 1
	}
	if asJSON {
		return writeJSON(env.Stdout, struct {
			TokensPerTask *uint64 `json:"tokens_per_task"`
		}{value})
	}
	amount := "none"
	if value != nil {
		amount = strconv.FormatUint(*value, 10)
	}
	fmt.Fprintf(env.Stdout, "tokens_per_task %s\n", amount)
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
