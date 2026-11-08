package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/land"
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
	// A member's write is recorded as the member's; "owner" is only the
	// no-account default (also for the plain "cli" role of a person's shell).
	role := "owner"
	if strings.TrimSpace(env.getenv("GRID_ROLE")) == "member" {
		role = "member"
	}
	return task.WriteOptions{Cwd: env.Cwd, ID: id, Action: action, Note: note, Who: contract.Author{ID: who, Role: role}}
}

func runTaskWrite(args []string, env Env) int {
	sub := args[0]
	fs := newFlags("task "+sub, env)
	asJSON := fs.Bool("json", false, "print JSON")
	reason, title, project := "", "", ""
	if sub == "stop" || sub == "close" || sub == "reopen" || sub == "decline" {
		fs.StringVar(&reason, "reason", "", "reason for the move")
	}
	if sub == "rename" {
		fs.StringVar(&title, "title", "", "new title (1-200 characters on one line)")
	}
	if sub == "rename" || sub == "decline" {
		fs.StringVar(&project, "project", "", "project name or id; personal selects Personal")
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
	if sub == "rename" || sub == "decline" {
		// Keep the host role separate from the audit role's local-owner default.
		role := strings.TrimSpace(env.getenv("GRID_ROLE"))
		var event task.Event
		var err error
		if sub == "rename" {
			event, err = store.Rename(task.RenameOptions{WriteOptions: o, Title: title, Project: project, Role: role})
		} else {
			// An absent host person means "you" in next; the receipt still uses
			// the existing local author default.
			o.Who.ID = env.getenv("GRID_ACTOR")
			if o.Who.ID == "" {
				o.Who.ID = env.getenv("GRID_WHO")
			}
			event, err = store.Decline(task.DeclineOptions{WriteOptions: o, Reason: reason, Project: project, Role: role})
		}
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid task %s: %v\n", sub, err)
			switch {
			case errors.Is(err, task.ErrEditUsage):
				return 2
			case errors.Is(err, task.ErrEditRejected):
				return 3
			default:
				return 1
			}
		}
		return taskWriteResult(env, event, nil, *asJSON)
	}
	if sub == "status" {
		o.State = pos[1]
	}
	event, err := store.Change(o)
	return taskWriteResult(env, event, err, *asJSON)
}

const approveUsage = "Usage: grid approve <T-NN> [--plan-hash <hash>] [--note <text>] [--json]\n"

func runApprove(args []string, env Env) int {
	fs := newFlags("approve", env)
	hash := fs.String("plan-hash", "", "hash of the reviewed plan, as grid task show prints it")
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
	event, err := (land.Store{Resolver: resolver(env)}).Approve(writeOptions(env, pos[0], "approve", *note), *hash)
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
	if agentShell(env) {
		fmt.Fprintln(env.Stderr, "Approval and refusal decisions require a person or the app; run this command in your own shell.")
		return false
	}
	return true
}

// agentShell reports whether the environment says a worker or a non-person role runs this command.
func agentShell(env Env) bool {
	for _, key := range []string{"GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER"} {
		if strings.TrimSpace(env.getenv(key)) != "" {
			return true
		}
	}
	role := strings.TrimSpace(env.getenv("GRID_ROLE"))
	return role != "" && role != "owner" && role != "member" && role != "cli"
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
