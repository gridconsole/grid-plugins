package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/usage"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const usageHelp = `Usage: grid usage [--task T-NN] [--json]
       grid usage post --job J-N|lead [--dollars N] [--minutes N] [--tokens N] [--session ID] [--model NAME]
       grid usage show
All commands accept --json; task-scoped commands accept --task T-NN.
Flags alone fold the token totals observed by the session hooks; they are not Job usage posts.
Posts are deltas; at least one amount is required.
Units are dollars, minutes and tokens; token amounts are non-negative integers.
Usage is a number the window shows: nothing stops, holds or warns about work at a level.
`

type usageNumbers struct {
	values []float64
}

func (n *usageNumbers) String() string { return "" }
func (n *usageNumbers) Set(value string) error {
	if len(n.values) > 0 {
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
	var result string
	if op == "post" {
		fs.StringVar(&result, "result", "", "book the cost, minutes and tokens of the last result line of this Claude attempt log, once per session; not with --dollars, --minutes or --tokens")
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
	var parsed usage.Input
	if result != "" {
		if dollars.amount() != nil || minutes.amount() != nil || tokens.amount() != nil {
			fmt.Fprintln(env.Stderr, "usage: --result books the amounts in the log; do not combine it with --dollars, --minutes or --tokens")
			return 2
		}
		var err error
		if parsed, err = readResultLog(result); err != nil {
			return commandError(env, "usage", err)
		}
	}
	res, err := taskFor(env, *task)
	if err != nil {
		return commandError(env, "usage", err)
	}
	s := usage.Store{TaskPath: res.TaskPath, GridHome: env.GridHome}
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
	if result != "" {
		// Typed --session and --model override what the log says; the key stays the log's own.
		in.Dollars, in.Minutes, in.Tokens, in.Key = parsed.Dollars, parsed.Minutes, parsed.Tokens, parsed.Key
		if in.Session == "" {
			in.Session = parsed.Session
		}
		if in.Model == "" {
			in.Model = parsed.Model
		}
	}
	out, err := s.Post(in)
	if err != nil {
		return commandError(env, "usage", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, out)
	}
	if out.AlreadyBooked {
		fmt.Fprintf(env.Stdout, "already booked: %s\n", in.Key)
	}
	printUsage(env, out.Summary)
	return 0
}

// readResultLog parses the attempt log at path; every error names the file.
func readResultLog(path string) (usage.Input, error) {
	f, err := os.Open(path)
	if err != nil {
		return usage.Input{}, err
	}
	defer f.Close()
	in, _, err := usage.ParseResult(f)
	if err != nil {
		return usage.Input{}, fmt.Errorf("%s: %w", path, err)
	}
	return in, nil
}

func printUsage(env Env, summary usage.Summary) {
	for _, row := range summary.Jobs {
		fmt.Fprintf(env.Stdout, "%s  dollars %g  minutes %g  tokens %d  posts %d\n", row.Job, row.Dollars, row.Minutes, row.Tokens, row.Posts)
	}
	t := summary.Totals
	fmt.Fprintf(env.Stdout, "total  dollars %g  minutes %g  tokens %d  posts %d\n", t.Dollars, t.Minutes, t.Tokens, t.Posts)
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
		env.recordError(err)
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
