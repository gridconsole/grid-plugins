package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"github.com/gridconsole/grid-plugins/internal/land"
)

func runLand(args []string, env Env) int {
	f := newFlags("land", env)
	dry := f.Bool("dry-run", false, "print the plan without writes or execution")
	asJSON := f.Bool("json", false, "print JSON")
	operands, code, stop := parseMixed(f, args)
	if stop {
		return code
	}
	if len(operands) != 1 {
		fmt.Fprintln(env.Stderr, "Usage: grid land <task> [--dry-run] [--json]")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	result, err := (land.Store{Resolver: resolver(env), Who: writeOptions(env, "", "", "").Who}).Run(ctx, env.Cwd, operands[0], *dry)
	if err != nil && result.Execution == nil {
		return landError(env, err)
	}
	if *asJSON {
		if code := writeJSON(env.Stdout, result); code != 0 {
			return code
		}
	} else {
		if _, writeErr := fmt.Fprintf(env.Stdout, "task: %s\nproject: %s\nplan_hash: %s\nstatus: %s\ntarget: %s\nbranch: %s\ncommand: %s\nneeds_approve: %t\n", result.Plan.Task, result.Plan.Project, result.Plan.Hash, result.Status, result.Plan.Rule.Target, result.Plan.Rule.Branch, result.Plan.Rule.Command, result.Plan.Rule.NeedsApprove); writeErr != nil {
			return commandError(env, "land", writeErr)
		}
	}
	if err != nil {
		return landError(env, err)
	}
	return 0
}

func landError(env Env, err error) int {
	fmt.Fprintf(env.Stderr, "land: %v\n", err)
	if errors.Is(err, land.ErrRejected) {
		return 3
	}
	return 1
}
