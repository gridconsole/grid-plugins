package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/worker"
)

func runWorker(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid worker list [--json]")
		return 0
	}
	if args[0] != "list" {
		fmt.Fprintf(env.Stderr, "grid worker: unknown command %q\n", args[0])
		return 2
	}
	fs := newFlags("worker list", env)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "grid worker list: unexpected argument")
		return 2
	}
	entries, err := (worker.Store{GridHome: env.GridHome}).List()
	if err != nil {
		return failure(env, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, entries)
	}
	for _, entry := range entries {
		fmt.Fprintf(env.Stdout, "machine: %s\npid: %d\nstate: %s\npresence: %s\n", entry.Machine, entry.PID, entry.State, entry.Presence)
	}
	return 0
}
