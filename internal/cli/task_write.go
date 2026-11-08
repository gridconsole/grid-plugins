package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func taskWriter(env Env) task.Store { return task.Store{Resolver: resolver(env)} }

func writeOptions(env Env, id, action, note string) task.WriteOptions {
	// A host can supply its authenticated member id; a terminal without an
	// account records a local person without copying an OS login into task files.
	who := env.getenv("GRID_ACTOR")
	if who == "" {
		who = "local"
	}
	return task.WriteOptions{Cwd: env.Cwd, ID: id, Action: action, Note: note, Who: contract.Author{ID: who, Role: "owner"}}
}

func runTaskWrite(args []string, env Env) int {
	sub := args[0]
	fs := newFlags("task "+sub, env)
	asJSON := fs.Bool("json", false, "print JSON")
	reason := ""
	if sub == "stop" || sub == "close" || sub == "reopen" {
		fs.StringVar(&reason, "reason", "", "reason for the move")
	}
	pos, err := taskArgs(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	count := 1
	if sub == "status" {
		count = 2
	}
	if len(pos) != count {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	store := taskWriter(env)
	if sub == "history" {
		lines, err := store.History(env.Cwd, pos[0])
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid task history: %v\n", err)
			return 1
		}
		if *asJSON {
			return writeJSON(env.Stdout, lines)
		}
		for _, line := range lines {
			fmt.Fprintln(env.Stdout, line)
		}
		return 0
	}
	o := writeOptions(env, pos[0], sub, reason)
	if sub == "status" {
		o.State = pos[1]
	}
	event, err := store.Change(o)
	return taskWriteResult(env, event, err, *asJSON)
}

const approveUsage = "Usage: grid approve <T-NN> [--plan-hash <hash>] [--note <text>] [--json]\n"

func runApprove(args []string, env Env) int {
	fs := newFlags("approve", env)
	hash := fs.String("plan-hash", "", "opaque hash of the reviewed plan")
	note := fs.String("note", "", "approval note")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 1 {
		fmt.Fprint(env.Stderr, approveUsage)
		return 2
	}
	if !personWrite(env) {
		return 1
	}
	event, err := taskWriter(env).Approve(writeOptions(env, pos[0], "approve", *note), *hash)
	return taskWriteResult(env, event, err, *asJSON)
}

const refusalUsage = "Usage: grid refusal decide <T-NN> <R-N> allow|deny [--note <text>] [--json]\n"

func runRefusal(args []string, env Env) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help" || args[0] == "-h") {
		fmt.Fprint(env.Stdout, refusalUsage)
		return 0
	}
	if len(args) == 0 || args[0] != "decide" {
		fmt.Fprint(env.Stderr, refusalUsage)
		return 2
	}
	fs := newFlags("refusal decide", env)
	note := fs.String("note", "", "decision note")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 3 || (pos[2] != "allow" && pos[2] != "deny") {
		fmt.Fprint(env.Stderr, refusalUsage)
		return 2
	}
	if !personWrite(env) {
		return 1
	}
	event, err := taskWriter(env).Decide(writeOptions(env, pos[0], "refusal decide", *note), pos[1], pos[2])
	return taskWriteResult(env, event, err, *asJSON)
}

func personWrite(env Env) bool {
	// This is a cooperative guard, not authentication. Host tool guards and
	// worker deny settings enforce the boundary; a person's shell is supported.
	for _, key := range []string{"GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER"} {
		if strings.TrimSpace(env.getenv(key)) != "" {
			fmt.Fprintln(env.Stderr, "Approval and refusal decisions require a person or the app; run this command in your own shell.")
			return false
		}
	}
	role := strings.TrimSpace(env.getenv("GRID_ROLE"))
	if role != "" && role != "owner" && role != "member" && role != "cli" {
		fmt.Fprintln(env.Stderr, "Approval and refusal decisions require a person or the app; run this command in your own shell.")
		return false
	}
	return true
}

func taskWriteResult(env Env, event task.Event, err error, asJSON bool) int {
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid: %v\n", err)
		return 1
	}
	if asJSON {
		return writeJSON(env.Stdout, event)
	}
	fmt.Fprintf(env.Stdout, "%s %s: %s -> %s\n", event.Task, event.What, event.FromState, event.ToState)
	return 0
}
