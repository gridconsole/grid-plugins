package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

func taskWriter(env Env) task.Store { return task.Store{Resolver: resolver(env)} }

// writeJobWaiting prints the job_waiting object on stdout when err holds a
// *orchestrator.WaitingError, and reports whether it did. grid approve --json and
// grid job result --json on a person Job that still waits both use it.
func writeJobWaiting(env Env, err error) bool {
	var waiting *orchestrator.WaitingError
	if !errors.As(err, &waiting) {
		return false
	}
	_ = writeJSON(env.Stdout, struct {
		Error      string `json:"error"`
		Code       string `json:"code"`
		Job        string `json:"job"`
		WaitingFor string `json:"waiting_for"`
		Reason     string `json:"reason"`
	}{"job_waiting", "job_waiting", waiting.Job, waiting.Target, waiting.Reason})
	return true
}

func writeOptions(env Env, id, action, note string) task.WriteOptions {
	who, _ := writeAuthor(env)
	return task.WriteOptions{Cwd: env.Cwd, ID: id, Action: action, Note: note, Who: who}
}

// writeAuthor names who a task write records, and reports whether that is an
// agent. It only records: no command admits or refuses anyone because of it.
//
//   - A person's shell (no role, owner, member or the plain cli role) records the
//     host's member id (GRID_ACTOR) or the local person, as owner or member. With
//     no GRID_ACTOR and a worker mark (GRID_AGENT, GRID_WORKER_ID, GRID_WORKER) the
//     shell is an agent's and records the worker, role "agent", as deliver does.
//   - lead, guest and external record themselves (GRID_ACTOR else local), whatever
//     marks the shell carries: a guest with a worker id is still a guest.
//   - Any other role is an agent's (contract.TierFor): the worker, else the host's
//     member id, in the role the host gave.
func writeAuthor(env Env) (contract.Author, bool) {
	role := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	actor := env.getenv("GRID_ACTOR")
	person := func(role string) contract.Author {
		// A host can supply its authenticated member id; a terminal without an
		// account records a local person without copying an OS login into task files.
		if actor == "" {
			return contract.Author{ID: "local", Role: role}
		}
		return contract.Author{ID: actor, Role: role}
	}
	switch role {
	case "", "cli", "owner", "member":
		if actor == "" && workerMarked(env) {
			return contract.Author{ID: workerAuthorID(env), Role: "agent"}, true
		}
		// A member's write is recorded as the member's; "owner" is only the
		// no-account default (also for the plain "cli" role of a person's shell).
		if role == "member" {
			return person("member"), false
		}
		return person("owner"), false
	}
	if contract.TierFor(role) != contract.Agent {
		return person(role), false
	}
	if actor != "" {
		return contract.Author{ID: actor, Role: role}, true
	}
	return contract.Author{ID: workerAuthorID(env), Role: role}, true
}

// workerAuthorID is the worker a shell names: GRID_WORKER_ID, else GRID_AGENT,
// else GRID_WORKER, else "grid".
func workerAuthorID(env Env) string {
	for _, key := range []string{"GRID_WORKER_ID", "GRID_AGENT", "GRID_WORKER"} {
		if id := strings.TrimSpace(env.getenv(key)); id != "" {
			return id
		}
	}
	return "grid"
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
	// The project named by --project is resolved before the task operand, so an
	// unregistered project answers no_such_project, not no_such_task. A guest keeps
	// the answer it gets without the flag.
	if (sub == "rename" || sub == "decline") && project != "" && env.GridHome != "" && !resolver(env).Caller.IsGuest() {
		if _, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: project}); errors.Is(err, where.ErrProjectUnregistered) {
			return taskOperandError(env, "grid task "+sub, fmt.Errorf("scope: %w", err))
		}
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
			// An agent's write keeps the worker writeOptions named.
			if _, agent := writeAuthor(env); !agent {
				o.Who.ID = env.getenv("GRID_ACTOR")
				if o.Who.ID == "" {
					o.Who.ID = env.getenv("GRID_WHO")
				}
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
	// A reopen posts a new plan Job for a person (Q8). The writer is built first,
	// so a reopen never leaves the old result approving the plan.
	var postPlanJob func() error
	var cont *reopenContinuation
	if sub == "reopen" {
		var perr error
		if postPlanJob, perr = reopenPlanJob(env, resolved.Path); perr != nil {
			fmt.Fprintf(env.Stderr, "grid task reopen: %v\n", perr)
			return 1
		}
		// With no plan Job to answer again, a task that a stop left with no open Job
		// gets an orchestrator Job the worker starts (reopen_continue.go).
		if postPlanJob == nil {
			if cont, perr = reopenContinueJob(env, resolved.Path); perr != nil {
				fmt.Fprintf(env.Stderr, "grid task reopen: %v\n", perr)
				return 1
			}
		}
	}
	var event task.Event
	if cont != nil && cont.fromRunning {
		// A running task whose Jobs a stop cancelled goes through closed: the state
		// table moves a task to queued only from closed.
		stop := o
		stop.Action = "stop"
		if _, err = store.Change(stop); err != nil {
			cont.drop()
			if errors.Is(err, task.ErrStateMove) {
				env.recordError(err)
			}
			return taskWriteResult(env, event, err, *asJSON, resolved.Path)
		}
	}
	event, err = store.Change(o)
	if err == nil && postPlanJob != nil {
		err = postPlanJob()
	}
	if cont != nil {
		if err == nil {
			err = cont.post()
		} else {
			cont.drop()
		}
	}
	// Only this path records the code: taskWriteResult is also the exit of
	// grid approve, which keeps what it printed before.
	if errors.Is(err, task.ErrStateMove) {
		env.recordError(err)
	}
	return taskWriteResult(env, event, err, *asJSON, resolved.Path)
}

const approveUsage = `Usage: grid approve <task> [--plan-hash <hash>] [--note <text>] [--json]
       grid approve <task> --plan-hash <hash> --send-back <words> [--json]
<task> is an id, exact folder name, or folder path inside a project tasks directory.
--plan-hash accepts the 6-hex plan_hash or 64-hex plan_digest from grid plan hash --json or grid task show --json.
workflow.md with plan_hash_token: digest requires the 64-hex digest.
--send-back sends the open plan approve Job back with your words instead of approving
the plan; it needs the current --plan-hash and takes no --note.
`

func runApprove(args []string, env Env) int {
	fs := newFlags("approve", env)
	fs.Usage = func() { fmt.Fprint(fs.Output(), approveUsage); fs.PrintDefaults() }
	helpToStdout(fs, args, env)
	hash := fs.String("plan-hash", "", "6-hex plan_hash or 64-hex plan_digest from grid plan hash --json or grid task show --json; workflow.md plan_hash_token: digest requires the digest")
	note := fs.String("note", "", "approval note")
	sendBack := fs.String("send-back", "", "send the open plan approve Job back with these words instead of approving; needs --plan-hash, takes no --note")
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
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	if given["send-back"] {
		var usage error
		switch {
		case strings.TrimSpace(*sendBack) == "":
			usage = errors.New("--send-back needs the words that say why")
		case *hash == "":
			usage = errors.New("--send-back needs --plan-hash with the current plan hash")
		case given["note"]:
			usage = errors.New("--note is not taken with --send-back: the words are the answer")
		}
		if usage != nil {
			return taskWriteResult(env, task.Event{}, fmt.Errorf("%w: %v", task.ErrEditUsage, usage), false, "")
		}
	}
	resolved, err := resolveTaskOperand(env, pos[0])
	if err != nil {
		return taskOperandError(env, "grid approve", err)
	}
	pos[0] = resolved.ID
	o := writeOptions(env, pos[0], "approve", *note)
	o.PlanHashToken, err = workflow.PlanHashToken(resolved.Path)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
		return 3
	}
	// The approval is recorded only as the result of the approve person Job of the
	// task. An old approval receipt file is moved into that Job first; with no Job, grid
	// approve posts one.
	w, err := approvalWriter(env)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
		return 3
	}
	if given["send-back"] {
		return sendBackPlan(env, resolved.Path, o, w, *hash, *sendBack, *asJSON)
	}
	if _, _, err := migrate.ReceiptToJob(resolved.Path, w); err != nil {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
		return 1
	}
	store, job := planPersonJob(env, resolved.Path)
	if job != "" {
		// The person Job waits for its after list like any other answer.
		snap, err := loadFeedSnapshot(store)
		if err == nil {
			err = orchestrator.PersonReady(snap, job, time.Now())
		}
		// The ship Job also waits for the task's delivery to be accepted (shipWait),
		// except for the review of the delivery: that answer is recorded and the land
		// follows the acceptance.
		if err == nil && isShipJobID(snap, job) {
			if err = shipWait(snap, job); shipReviewing(snap, err) {
				fmt.Fprintf(env.Stderr, "grid approve: %s is answered; it lands when the review accepts the delivery\n", job)
				err = nil
			}
		}
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
			if *asJSON {
				writeJobWaiting(env, err)
			}
			return 3
		}
	}
	event, err := (land.Store{Resolver: resolver(env)}).Approve(o, *hash)
	// Approve has returned, so .task.lock and .plan.lock are released before
	// the post takes .orchestrator.lock. A stale hash posts nothing.
	if err == nil {
		env.afterApproveHook()
		err = recordApproval(env, resolved.Path, store, w, job, "approved", event)
		if err == nil && store.TaskDir != "" {
			refoldAfterWrite(env, store)
		}
		if err == nil {
			// A leased answer accepts the ship person Job here: the task lands now.
			landAfterShip(env, resolved.Path, o.Who)
		}
	}
	return taskWriteResult(env, event, err, *asJSON, resolved.Path)
}

// sendBackPlan is grid approve --send-back (D-ig): it answers the open plan
// approve Job of the task in dir with the person's words, send_back and the plan
// hash they read, and the holder's pass rejects the Job with them; the plan is
// not approved. It checks everything before it writes: the words fit an answer,
// an open approve Job exists and may be answered, and hash is the current plan.
// Then the answer goes to the Feed and the send_back line to history, which holds
// who and the hash, never the words. o and w are those of the approval.
func sendBackPlan(env Env, dir string, o task.WriteOptions, w feed.Writer, hash, words string, asJSON bool) int {
	words = strings.TrimSpace(words)
	if err := orchestrator.CheckPersonAnswer(words); err != nil {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
		return 3
	}
	store, job := planPersonJob(env, dir)
	open := false
	if job != "" {
		snap, err := loadFeedSnapshot(store)
		if err == nil {
			if j := snap.State.Jobs[job]; j != nil && j.Status != contract.Accepted {
				open = true
				err = orchestrator.PersonReady(snap, job, time.Now())
			}
		}
		if err != nil {
			fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
			if asJSON {
				writeJobWaiting(env, err)
			}
			return 3
		}
	}
	if !open {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", fmt.Errorf("%w: no open plan approve Job to send back", task.ErrEditRejected))
		return 3
	}
	current, err := task.SendBackHash(dir, o.PlanHashToken, hash)
	if err != nil {
		return taskWriteResult(env, task.Event{}, err, false, dir)
	}
	if _, err := orchestrator.AnswerPersonWith(store, w, job, words, map[string]any{"send_back": true, "sent_back_hash": current}, time.Now()); err != nil {
		fmt.Fprintf(env.Stderr, "grid approve: %v\n", err)
		return 1
	}
	event, err := taskWriter(env).SendBack(o, current)
	return taskWriteResult(env, event, err, asJSON, dir)
}

// workerMarked reports whether the environment names a worker.
func workerMarked(env Env) bool {
	for _, key := range []string{"GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER"} {
		if strings.TrimSpace(env.getenv(key)) != "" {
			return true
		}
	}
	return false
}

// agentShell reports whether the environment says a worker or a non-person role
// runs this command. Only task attach uses it, to bind an agent to its assigned
// task (a scope rule); no command refuses a caller because of it.
func agentShell(env Env) bool {
	if workerMarked(env) {
		return true
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
	// The policy reads o.Role; the audit role is the host's, unless an agent wrote.
	if _, agent := writeAuthor(env); !agent {
		o.Who.Role = role
	}
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
