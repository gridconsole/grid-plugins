package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/workflow"
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
	if contract.NormalizeRole(env.getenv("GRID_ROLE")) == "member" {
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
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		if sub == "rename" || sub == "decline" {
			err = fmt.Errorf("scope: %w", err)
		}
		return taskOperandError(env, "grid task "+sub, err)
	}
	pos[0] = resolved.ID
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
			env.recordError(err)
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
		return taskWriteResult(env, event, nil, *asJSON, resolved.Path)
	}
	if sub == "status" {
		o.State = pos[1]
	}
	event, err := store.Change(o)
	return taskWriteResult(env, event, err, *asJSON, resolved.Path)
}

const approveUsage = `Usage: grid approve <task> [--plan-hash <hash>] [--gate plan|preview|ship] [--note <text>] [--json]
<task> is an id, exact folder name, or folder path inside a project tasks directory.
--plan-hash accepts the 6-hex plan_hash or 64-hex plan_digest from grid plan hash --json or grid task show --json.
workflow.md with plan_hash_token: digest requires the 64-hex digest.
--gate names the gate the person approved; a task now waiting at another gate is refused (exit 1) and nothing is written.
`

func runApprove(args []string, env Env) int {
	fs := newFlags("approve", env)
	fs.Usage = func() { fmt.Fprint(env.Stderr, approveUsage); fs.PrintDefaults() }
	hash := fs.String("plan-hash", "", "6-hex plan_hash or 64-hex plan_digest from grid plan hash --json or grid task show --json; workflow.md plan_hash_token: digest requires the digest")
	gate := fs.String("gate", "", "the gate the person approved: plan, preview or ship; refused when the task waits at another gate")
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
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		return taskOperandError(env, "grid approve", err)
	}
	pos[0] = resolved.ID
	o := writeOptions(env, pos[0], "approve", *note)
	o.Gate = *gate
	o.PlanHashToken, err = workflow.PlanHashToken(resolved.Path)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
		return 3
	}
	// The approval is also the result of the open person Job of the approve
	// step, when the task has one. Without a Feed writer it stays open and
	// approval.json is written as before.
	var post func(task.Event) error
	if store, job := planPersonJob(resolved.Path); job != "" {
		if w, err := orchestratorWriter(env, "task", env.getenv("GRID_LEASE_EPOCH")); err != nil {
			fmt.Fprintf(env.Stderr, "grid approve: %s stays open: %v\n", job, err)
		} else {
			post = func(e task.Event) error { return personApproval(store, w, job, "approved", e) }
		}
	}
	event, err := (land.Store{Resolver: resolver(env)}).Approve(o, *hash)
	// Approve has returned, so .task.lock and .plan.lock are released before
	// the post takes .orchestrator.lock. A stale hash posts nothing.
	if err == nil && post != nil {
		err = post(event)
	}
	return taskWriteResult(env, event, err, *asJSON, resolved.Path)
}

const refusalUsage = "Usage: grid refusal decide <task> <R-N> allow|deny [--note <text>] [--json]\n       grid decide <task> <R-N> allow|deny [--note <text>] [--json]\n<task> is an id, exact folder name, or folder path inside a project tasks directory.\n"

func runRefusal(args []string, env Env) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "help" || args[0] == "-h") {
		fmt.Fprint(env.Stdout, refusalUsage)
		return 0
	}
	if len(args) == 0 || args[0] != "decide" {
		fmt.Fprint(env.Stderr, refusalUsage)
		return 2
	}
	return runDecide(args[1:], env)
}

// runDecide also serves the top-level short spelling.
func runDecide(args []string, env Env) int {
	fs := newFlags("refusal decide", env)
	fs.Usage = func() { fmt.Fprint(env.Stderr, refusalUsage); fs.PrintDefaults() }
	note := fs.String("note", "", "decision note")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args)
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
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		return taskOperandError(env, "grid refusal decide", err)
	}
	pos[0] = resolved.ID
	event, err := taskWriter(env).Decide(writeOptions(env, pos[0], "refusal decide", *note), pos[1], pos[2])
	return taskWriteResult(env, event, err, *asJSON, resolved.Path)
}

// agentShell reports whether the environment says a worker or a non-person role
// runs this command. Only task attach uses it, to bind an agent to its assigned
// task (a scope rule); no command refuses a caller because of it.
func agentShell(env Env) bool {
	for _, key := range []string{"GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER"} {
		if strings.TrimSpace(env.getenv(key)) != "" {
			return true
		}
	}
	role := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	return role != "" && role != "owner" && role != "member" && role != "cli"
}

func taskWriteResult(env Env, event task.Event, err error, asJSON bool, path string) int {
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid: %v\n", err)
		if errors.Is(err, task.ErrEditUsage) {
			return 2
		}
		return 1
	}
	if asJSON {
		return writeJSON(env.Stdout, struct {
			task.Event
			Folder string `json:"folder"`
		}{event, filepath.Base(path)})
	}
	fmt.Fprintf(env.Stdout, "%s %s: %s -> %s\n", event.Task, event.What, event.FromState, event.ToState)
	return 0
}

func runTaskTrash(args []string, env Env) int {
	sub := args[0]
	fs := newFlags("task "+sub, env)
	project := fs.String("project", "", "project name or id; required for restore")
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) != 1 || (sub == "restore" && *project == "") {
		fmt.Fprint(env.Stderr, taskUsage)
		return 2
	}
	role := strings.TrimSpace(env.getenv("GRID_ROLE"))
	if role == "" || role == "cli" {
		role = "owner"
	}
	o := task.TrashOptions{WriteOptions: writeOptions(env, pos[0], sub, ""), Project: *project, Role: role}
	o.Who.Role = role
	var result task.TrashResult
	if sub == "delete" {
		result, err = taskWriter(env).Delete(o)
	} else {
		result, err = taskWriter(env).Restore(o)
	}
	if err != nil {
		env.recordError(err)
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
	if *asJSON {
		return writeJSON(env.Stdout, result)
	}
	fmt.Fprintln(env.Stdout, result.Folder)
	return 0
}
