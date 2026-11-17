package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobcmd"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/session"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// jobCancelUsage is what `grid job cancel --help` says before its flags: how a person
// cancels a Running Job without knowing a lease epoch, and how an agent does.
const jobCancelUsage = `Usage: grid job cancel <job> --reason <text> [--task <task>] [--json]

Cancels a Job, a Running one too.
A person (role owner, member or lead) types no lease and no epoch. When the task lease is
held by your own <member>@<machine> (GRID_MEMBER on this machine), the cancel is written
at once under it. When another holder holds it (the machine's worker, worker@<machine>, by
default), the cancel is saved as your request and the holder writes it under its lease on
its next pass; ` + "`grid job show <job>`" + ` then shows it Cancelled. Both exit 0.
An agent cancels with the GRID_TASK_EPOCH its worker gave it. A --lease-epoch typed on the
command line is used as typed, so a guessed or old one is refused as a stale lease (exit 3).

`

func runOrchestratorJob(args []string, env Env) int {
	if len(args) > 0 {
		switch args[0] {
		case "next", "post", "ack", "result", "cancel", "tree", "blocked":
			return runOrchestrator(args, env)
		}
	}
	return runJob(args, env)
}
func runOrchestratorFeed(args []string, env Env) int {
	if len(args) > 0 && args[0] == "tick" {
		return runOrchestrator(args, env)
	}
	return runFeed(args, env)
}
func runOrchestrator(args []string, env Env) int {
	verb, rest := args[0], args[1:]
	id := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id, rest = rest[0], rest[1:]
	}
	name := "job " + verb
	if verb == "tick" {
		name = "feed tick" // runOrchestratorFeed sends only tick here
	}
	fs := newFlags(name, env)
	if verb == "cancel" {
		fs.Usage = func() { fmt.Fprint(fs.Output(), jobCancelUsage); fs.PrintDefaults() }
	}
	orderFlag := fs.Int("order", -1, "queue order of the Job (lowest first); on job post only")
	task := fs.String("task", "", "task folder or id")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	wait := fs.Bool("wait", false, "wait for an assignment or judgement event")
	named := id // the Job id the writer typed, before the GRID_JOB fallback
	if id == "" {
		id = env.getenv("GRID_JOB")
	}
	job := fs.String("job", id, "Job id")
	kind := fs.String("type", "post", "registered post type")
	body := fs.String("body", "{}", "JSON body, or - for stdin; in a shell put it in single quotes, since double quotes run backticks and $ (write a single quote as \\u0027)")
	reason := fs.String("reason", "", "cancellation reason")
	attempt := fs.Int("attempt", 0, "Job attempt (or GRID_ATTEMPT)")
	leaseFlag := fs.String("lease", env.getenv("GRID_LEASE"), "task or Job lease")
	epoch := fs.String("lease-epoch", env.getenv("GRID_LEASE_EPOCH"), "generation.fence")
	wake := fs.String("wake", env.getenv("GRID_WAKE"), "event/n idempotency key")
	re := fs.String("re", "", "record this follows")
	recordID := fs.String("id", "", "record ULID reused on retries")
	to := fs.String("to", "", "assignee member@machine; defaults to task holder")
	owner := fs.String("owner", env.getenv("GRID_OWNER"), "task owner's member id for crash-loop questions")
	afterWake := fs.Bool("after-wake", false, "suppress clock expiry until process identities and heartbeats are checked after sleep")
	usageReady := fs.Bool("usage-ready", false, "capacity has been restored for the selected assignee")
	person := fs.String("person", "", "post a Job for this person (role Person); other Jobs depend on it with after")
	planHash := fs.String("plan-hash", "", "on job result for a person Job of an approve step: the 6-hex plan_hash or 64-hex plan_digest the person approved")
	fromBrief := fs.String("from-brief", "", "on job post: post a Job built from this brief file, copied into the task's inputs")
	if code, stop := parse(fs, rest, env); stop {
		return code
	}
	if *fromBrief != "" {
		if verb != "post" || *person != "" || *body != "{}" {
			fmt.Fprintln(env.Stderr, "--from-brief applies to job post, without --person or --body")
			return 2
		}
		*kind = "job"
	}
	if *person != "" {
		if verb != "post" {
			fmt.Fprintln(env.Stderr, "--person applies to job post")
			return exitCode(&feed.InvalidRecordError{Err: errors.New("--person applies to job post")})
		}
		*kind = "job"
	}
	fail := func(err error) int {
		fmt.Fprintln(env.Stderr, err)
		// The body parser reports this before a record reaches the shared writer.
		if err.Error() == "--body must be a JSON object" {
			err = &feed.InvalidRecordError{Err: err}
		}
		recordLookupError(env, err)
		return exitCode(err)
	}
	var obj map[string]any
	if verb == "post" {
		if *body == "-" {
			data, err := io.ReadAll(env.Stdin)
			if err != nil {
				return fail(err)
			}
			*body = string(data)
		}
		if err := json.Unmarshal([]byte(*body), &obj); err != nil || obj == nil {
			return fail(errors.New("--body must be a JSON object"))
		}
		if *orderFlag >= 0 {
			if *kind != "job" {
				return fail(&feed.InvalidRecordError{Err: errors.New("--order applies to a Job post")})
			}
			obj["order"] = *orderFlag
		}
		if *person != "" {
			obj["role"] = "Person"
			intent, _ := obj["intent"].(map[string]any)
			if intent == nil {
				intent = map[string]any{}
			}
			intent["owner"] = *person
			obj["intent"] = intent
		}
		// A Job post takes its id from the body; a Job id typed on the command line
		// (positional or --job) must be the same one.
		if *kind == "job" {
			if bodyID, _ := obj["id"].(string); bodyID != "" {
				if *job != id {
					named = *job
				}
				if named != "" && named != bodyID {
					return fail(&feed.InvalidRecordError{Err: fmt.Errorf("Job id %s on the command line differs from the body id %s", named, bodyID)})
				}
				*job = bodyID
			}
		}
	}
	// An agent works in a git worktree outside the task folder, so its folder names no task; the
	// worker gives it the session's task as GRID_FEED_TASK, or as GRID_TASK when the app starts it,
	// which grid feed reads the same way: --task, GRID_FEED_TASK, GRID_TASK, then the cwd.
	if *task == "" {
		*task = env.getenv("GRID_FEED_TASK")
	}
	if *task == "" {
		*task = env.getenv("GRID_TASK")
	}
	scope, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *task, Project: *project})
	if err != nil {
		return fail(err)
	}
	if scope.TaskPath == "" {
		return fail(errors.New("no task selected; use --task"))
	}
	if verb == "post" {
		// A refused withdrawal must leave a legacy task untouched, so check before the identity write.
		if err := (feed.Store{TaskDir: scope.TaskPath, Warn: env.Stderr}).CheckWithdrawalPolicy(feed.Input{Type: *kind, Body: obj}); err != nil {
			return fail(err)
		}
	}
	if verb != "tree" && verb != "blocked" {
		if _, err := feed.EnsureTaskID(scope.TaskPath); err != nil {
			return fail(err)
		}
	}
	taskID, taskOwner, err := taskstore.FeedIdentity(scope.TaskPath)
	if err != nil {
		return fail(err)
	}
	if *owner == "" {
		*owner = taskOwner
	}
	store := feed.Store{TaskDir: scope.TaskPath, TaskID: taskID, Warn: env.Stderr}
	if *fromBrief != "" {
		if obj, err = jobFromBrief(env, store, scope.TaskPath, *fromBrief, *job); err != nil {
			return fail(err)
		}
		*job, _ = obj["id"].(string)
	}
	if verb == "tree" || verb == "blocked" {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return fail(err)
		}
		if verb == "tree" {
			rows := orchestrator.Tree(snap)
			if *asJSON {
				return writeJSON(env.Stdout, rows)
			}
			for _, r := range rows {
				from := ""
				if r.DiscoveredFrom != nil {
					from = "  discovered_from=" + *r.DiscoveredFrom
				}
				fmt.Fprintf(env.Stdout, "%s  %s  %s%s\n", r.ID, r.Role, r.Status, from)
				for _, a := range r.Attempts {
					line := fmt.Sprintf("  attempt %d  fence %d  %s  %s", a.Number, a.Fence, a.State, a.Exit)
					if a.Code != "" {
						line += " code " + a.Code
					}
					if a.ReasonMissing {
						line += " (no reason given)"
					}
					if a.NoResult {
						line += " no result posted"
					}
					fmt.Fprintln(env.Stdout, line)
				}
				if o := r.Orphan; o != nil {
					fmt.Fprintf(env.Stdout, "  orphan %s since %s: %s\n", o.Kind, o.Since, o.Detail)
				}
			}
			return 0
		}
		ready, err := orchestrator.Ready(snap, time.Now())
		if err != nil {
			return fail(err)
		}
		waits := map[string]string{}
		for _, w := range ready.Waiting {
			if waits[w.Job] == "" {
				waits[w.Job] = w.Reason
			}
		}
		rows := []orchestrator.Row{}
		for _, r := range orchestrator.Tree(snap) {
			why := waits[r.ID]
			if r.Status == contract.Parked || why == "" && (r.Status == contract.Posted || r.Status == contract.Ready) && r.Reason != "ready" {
				why = r.Reason
			}
			if why != "" {
				rows = append(rows, orchestrator.Row{ID: r.ID, Role: r.Role, State: r.Status, Why: why})
			}
		}
		return orchestratorRows(env, rows, *asJSON)
	}
	// A person's Job is never assigned, so its result is written under the task lease.
	personResult := false
	var personJobRole, personOwner, personKind string
	if verb == "result" && *job != "" {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return fail(err)
		}
		for _, it := range snap.Items {
			if it.ID == *job && it.IsPerson() && !personResult {
				personResult, personJobRole, personOwner, personKind = true, it.Role, it.PersonName(), it.Kind
			}
		}
	}
	leaseName := "task"
	if verb == "ack" || verb == "result" || verb == "post" {
		leaseName = *job
		if leaseName == "" {
			leaseName = env.getenv("GRID_LEASE")
		}
	}
	if verb == "post" && (*kind == "job" || *kind == "control") || personResult {
		leaseName = "task"
	}
	// A Job agent's GRID_LEASE names its own Job, but posting a Job is a task
	// write. The worker gives every agent the task epoch it is holding as
	// GRID_TASK_EPOCH, so a Job post uses that, unless --lease or --lease-epoch
	// was typed. Cancelling a Job is the same task write, so `cancel` takes the
	// same rule; `post --kind control` does not. The global --lease-epoch, which goes
	// before the command, counts as typed: env.getenv shows it as GRID_LEASE_EPOCH,
	// so the flag set alone cannot tell.
	if (verb == "post" && *kind == "job" || verb == "cancel") && env.getenv("GRID_TASK_EPOCH") != "" {
		typed := map[string]bool{}
		fs.Visit(func(f *flag.Flag) { typed[f.Name] = true })
		typed["lease-epoch"] = typed["lease-epoch"] || env.typedGlobal["GRID_LEASE_EPOCH"]
		if !typed["lease"] {
			*leaseFlag = ""
			if !typed["lease-epoch"] {
				*epoch = env.getenv("GRID_TASK_EPOCH")
			}
		}
	}
	if *leaseFlag != "" {
		leaseName = *leaseFlag
	}
	// A person of the team with no lease epoch posts a Job or answers a person Job
	// as an unleased record, so the worker holding the task lease does not stop it.
	// A plan-hash approval is answered the same way, and every other role keeps the lease path.
	personRole := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	if personRole == "" {
		personRole = "member"
	}
	unleased := *epoch == "" && contract.TierFor(personRole) == contract.PersonTeam && (verb == "post" && *kind == "job" || personResult)
	// A guest the person Job names answers it the same way; every other guest, and a
	// guest with any lease, keeps the lease path and its refusal.
	if !unleased && personResult && *planHash == "" && *epoch == "" && *leaseFlag == "" && env.getenv("GRID_LEASE") == "" {
		unleased = contract.GuestAddressee(personRole, env.getenv("GRID_MEMBER"), false, personJobRole, personOwner, personKind)
	}
	// A person of the team at a terminal holds no lease epoch to cancel a Job with. When the
	// task lease names this member's own <member>@<machine>, the cancel is written under the
	// lease's current generation and fence, with the person as the author. When another
	// holder holds it (by default the machine's worker), the person writes an unleased cancel
	// request instead, as a person posts a Job or answers a person Job, and the holder's next
	// pass writes the leased control (personCancels beside personAnswers). The person never
	// writes under another holder's lease, so store.go's holder check and record.go's
	// author check stay as they are. A typed epoch is used as typed, and an agent keeps the
	// epochs the worker gave it.
	request := false
	if verb == "cancel" && *epoch == "" && (*leaseFlag == "" || *leaseFlag == "task") && contract.TierFor(personRole) == contract.PersonTeam {
		held, readErr := (lease.TaskStore{Feed: store}).Read("task")
		if readErr == nil && held.Holder != nil && *held.Holder == env.getenv("GRID_MEMBER")+"@"+localMachine(env) {
			leaseName, *epoch = "task", fmt.Sprintf("%d.%d", held.Gen, held.Fence)
		} else {
			request, unleased = true, true
		}
	}
	var writer feed.Writer
	if unleased {
		writer, err = feedWriter(env, "", "")
	} else {
		writer, err = orchestratorWriter(env, leaseName, *epoch)
	}
	if err != nil {
		return fail(err)
	}
	if verb == "next" || verb == "tick" {
		s := orchestrator.Store{AfterWake: *afterWake, Feed: store, Writer: writer, Assignee: *to, Owner: *owner}
		configureTickStore(env, &s, scope.Project, taskID)
		if *usageReady {
			s.UsageReady = func(worker string) bool { return worker == *to || *to == "" && worker == writer.Worker }
		}
		ctx, cancel := env.signalContext(os.Interrupt)
		defer cancel()
		out, err := s.Next(ctx, *wait && verb == "next")
		if errors.Is(err, context.Canceled) {
			return 130
		}
		if err != nil {
			return fail(err)
		}
		if out.Wake != nil {
			if *asJSON {
				return writeJSON(env.Stdout, struct {
					Wake *orchestrator.Wake `json:"wake"`
				}{out.Wake})
			}
			fmt.Fprintf(env.Stdout, "Wake %s  %s/%d\n", out.Wake.Job, out.Wake.Event, out.Wake.N)
			return 0
		}
		return orchestratorRows(env, out.Rows, *asJSON)
	}
	if *job == "" {
		return fail(errors.New("Job id is required"))
	}
	if *attempt == 0 {
		*attempt, _ = strconv.Atoi(env.getenv("GRID_ATTEMPT"))
	}
	if verb != "post" {
		if *body == "-" {
			b, err := io.ReadAll(env.Stdin)
			if err != nil {
				return fail(err)
			}
			*body = string(b)
		}
		if err := json.Unmarshal([]byte(*body), &obj); err != nil || obj == nil {
			return fail(errors.New("--body must be a JSON object"))
		}
	}

	if personResult && *planHash != "" {
		code := runPersonApproval(env, store, writer, scope.TaskPath, *job, obj, *planHash, *asJSON)
		if code == 0 {
			refoldAfterWrite(env, store)
		}
		return code
	}
	if personResult {
		answer, _ := obj["answer"].(string)
		record, err := orchestrator.AnswerPerson(store, writer, *job, answer, time.Now())
		if err != nil {
			if *asJSON {
				// A Job that still waits also prints the job_waiting object, as grid approve does.
				writeJobWaiting(env, err)
			}
			return fail(err)
		}
		refoldAfterWrite(env, store)
		// An unleased answer closes nothing by itself: the Job stays open until the
		// next pass of whoever holds the task lease, and with nobody holding it that
		// pass waits for the next tick. Say so, so the answer is not mistaken for done.
		if unleased {
			fmt.Fprintln(env.Stderr, "Saved. The answer will be taken up on the next pass of the task's orchestrator.")
			if *asJSON {
				return writeJSON(env.Stdout, struct {
					feed.Record
					Pending bool `json:"pending"`
				}{record, true})
			}
		}
		if *asJSON {
			return writeJSON(env.Stdout, record)
		}
		fmt.Fprintln(env.Stdout, record.ID)
		return 0
	}
	typ := verb
	if verb == "post" {
		typ = *kind
	}
	if verb == "cancel" {
		if *reason == "" {
			return fail(errors.New("cancel requires --reason"))
		}
		typ = "control"
		obj = map[string]any{"target": *job, "status": "cancelled", "reason": *reason, "close_reason": *reason}
	}
	write := orchestrator.Write
	if unleased {
		write = orchestrator.PostPerson
	}
	// A Job post says what render will say about the Job; it refuses nothing itself.
	var findings []lint.Finding
	var findingsErr error
	checked := verb == "post" && *kind == "job" && *person == ""
	if checked {
		findings, findingsErr = postFindings(env, scope.TaskPath, obj)
	}
	record, err := write(store, writer, feed.Input{ID: *recordID, Type: typ, Body: obj, Job: *job, Attempt: *attempt, Wake: *wake, Re: *re})
	if errors.Is(err, orchestrator.ErrDuplicateWake) {
		fmt.Fprintln(env.Stdout, record.ID)
		return fail(err)
	}
	if err != nil {
		return fail(err)
	}
	// The post is written and write has released every lock it took. The plugin
	// owns needs-you.json and the app reads it as authoritative, so refold it now,
	// as grid feed post does: an answer, Job or result that opens or closes a
	// question shows without a tick. A fold error does not undo the post; the next
	// fold repairs the files. The write's own fold has said any withdrawal-policy
	// warning for this operation already, so the refold does not say it again.
	refoldAfterWrite(env, store)
	// A cancel request moves no Job by itself: the Job stays as it is until the next
	// pass of whoever holds the task lease writes the leased control. Say so, as an
	// unleased answer does above, so the request is not mistaken for a cancelled Job.
	if request {
		fmt.Fprintln(env.Stderr, "Saved. The cancel will be carried out on the next pass of the task's orchestrator.")
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				feed.Record
				Pending bool `json:"pending"`
			}{record, true})
		}
	}
	if checked {
		printPostFindings(env.Stderr, *job, findings, findingsErr)
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				feed.Record
				Findings []lint.Finding `json:"findings"`
			}{record, findings})
		}
	}
	if *asJSON {
		return writeJSON(env.Stdout, record)
	}
	fmt.Fprintln(env.Stdout, record.ID)
	return 0
}

// refoldAfterWrite refolds the published state after a write that is already
// done and has released its locks. A fold error does not undo the write: it is
// said on stderr and the next fold repairs the files. The write's own fold has
// said any withdrawal-policy warning already, so this one does not say it again.
func refoldAfterWrite(env Env, store feed.Store) {
	refold := store
	refold.Warn = io.Discard
	refold.Machine = env.MachineFolder
	if _, err := refold.RefoldPublished(); err != nil {
		fmt.Fprintln(env.Stderr, "refold:", err)
	}
}

// jobFromBrief copies the brief at src into the task's inputs and answers the Job
// body job.FromBrief builds from it, with Job id id, or the task's next free Job id
// when id is empty. A brief whose repo the project does not list is said on stderr
// and the Job names no repo, so it works in the project's first one.
func jobFromBrief(env Env, store feed.Store, taskPath, src, id string) (map[string]any, error) {
	if !filepath.IsAbs(src) {
		src = filepath.Join(env.Cwd, src)
	}
	if id == "" {
		snap, err := orchestrator.Load(store)
		if err != nil {
			return nil, err
		}
		id = nextJobID(snap)
	}
	input, data, err := job.CopyBrief(taskPath, src)
	if err != nil {
		return nil, err
	}
	var repos []string
	if opts, err := jobcmd.LintOptions(taskPath, jobcmdEnv(env, resolver(env))); err == nil {
		repos = opts.Repos
	}
	body, skipped, err := job.FromBrief(id, input, data, repos)
	if err != nil {
		return nil, &feed.InvalidRecordError{Err: fmt.Errorf("--from-brief %s: %w", src, err)}
	}
	if skipped != "" {
		fmt.Fprintf(env.Stderr, "grid job post: the brief names repo %s, which the project does not list; the Job works in the project's first repo\n", skipped)
	}
	return body, nil
}

// postFindings lints a Job body with the options grid job render uses. It is told
// to the poster and never refuses: a failure to build the options is returned for
// the caller to say, with no findings.
func postFindings(env Env, taskPath string, body map[string]any) ([]lint.Finding, error) {
	opts, err := jobcmd.LintOptions(taskPath, jobcmdEnv(env, resolver(env)))
	if err != nil {
		return []lint.Finding{}, err
	}
	data, err := json.Marshal(body)
	if err != nil {
		return []lint.Finding{}, err
	}
	return lint.CheckJSON(data, opts), nil
}

// printPostFindings writes one stderr line per finding; an error says render will refuse the Job.
func printPostFindings(w io.Writer, job string, findings []lint.Finding, err error) {
	if err != nil {
		fmt.Fprintf(w, "grid job post: %s: the findings could not be checked: %v\n", job, err)
		return
	}
	for _, f := range findings {
		tail := ""
		if f.Severity == "error" {
			tail = " (render will refuse this Job)"
		}
		fmt.Fprintf(w, "grid job post: %s: %s: %s%s\n", job, f.Rule, f.Message, tail)
	}
}

// recordLookupError records a failure that carries a lookup sentinel (an
// unregistered project, an unknown task) so --json also gets the error envelope;
// any other failure keeps its stderr line and empty stdout.
func recordLookupError(env Env, err error) {
	if errorCode(err) != "" {
		env.recordError(err)
	}
}

// runPersonApproval answers the person Job of an approve step with the plan
// hash the person saw. It moves an old approval receipt into the Feed first, runs
// the compare-and-set of grid approve, and then posts the Job result once
// Approve has released .task.lock and .plan.lock; a stale hash posts nothing.
func runPersonApproval(env Env, store feed.Store, w feed.Writer, dir, job string, body map[string]any, hash string, asJSON bool) int {
	answer, _ := body["answer"].(string)
	note, _ := body["note"].(string)
	if _, _, err := migrate.ReceiptToJob(store.TaskDir, w); err != nil {
		fmt.Fprintln(env.Stderr, err)
		recordLookupError(env, err)
		return 1
	}
	// The same refusals AnswerPerson gives, before anything is written: an answer
	// over the cap, and a Job that is not a person's or is already closed.
	if answer = strings.TrimSpace(answer); answer == "" {
		answer = "approved"
	}
	if err := orchestrator.CheckPersonAnswer(answer); err != nil {
		fmt.Fprintln(env.Stderr, err)
		recordLookupError(env, err)
		return exitCode(err)
	}
	snap, err := orchestrator.Load(store)
	if err == nil {
		err = orchestrator.PersonOpen(snap, job, time.Now())
	}
	// The ship Job lands the task's delivery: it waits while that delivery is
	// rejected and its rework is not yet accepted (shipWait).
	if err == nil && isShipJobID(snap, job) {
		err = shipWait(snap, job)
	}
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		if asJSON {
			writeJobWaiting(env, err)
		}
		recordLookupError(env, err)
		return exitCode(err)
	}
	resolved, err := resolveTaskOperand(env, dir)
	if err != nil {
		return taskOperandError(env, "grid job result", err)
	}
	o := writeOptions(env, resolved.ID, "approve", note)
	o.PlanHashToken, err = workflow.PlanHashToken(resolved.Path)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid job result: %v\n", err)
		return 3
	}
	event, err := (land.Store{Resolver: resolver(env)}).Approve(o, hash)
	// Approve has returned, so no task or plan lock is held while the post
	// waits for .orchestrator.lock, which a tick holds before .task.lock.
	if err == nil {
		env.afterApproveHook()
		err = recordApproval(env, resolved.Path, store, w, job, answer, event)
		if err == nil {
			// A leased answer accepts the ship person Job here: the task lands now.
			landAfterShip(env, resolved.Path, o.Who)
		}
	}
	return taskWriteResult(env, event, err, asJSON, resolved.Path)
}

// loadFeedSnapshot reads the Feed snapshot for grid approve; a test replaces it
// to make the second read fail after planPersonJob's read succeeded.
var loadFeedSnapshot = orchestrator.Load

// planPersonJob is the plan Job a grid approve answers: the open person Job of
// the task's approve step that has no answer yet and is ready for a person, the
// first such in Feed order. An earlier open approve Job that already holds a
// person's answer waits for the holder's next tick, and a press for a later step
// must not name it again. When no open Job meets both, it is the first open one
// (a repeat press of it posts nothing, and a Job that still waits for another
// Job is refused by the caller), else the last accepted one, which takes a new
// result when every such Job is already accepted (the plan changed since and is
// approved again): the readers use the newest result, so a changed plan can be
// approved again. It is "" when the task has no approve Job.
func planPersonJob(env Env, dir string) (feed.Store, string) {
	id, _, err := taskstore.FeedIdentity(dir)
	if err != nil {
		return feed.Store{}, ""
	}
	store := feed.Store{TaskDir: dir, TaskID: id, Warn: env.Stderr}
	snap, err := orchestrator.Load(store)
	if err != nil {
		return store, ""
	}
	accepted, open := "", ""
	for _, it := range snap.Items {
		j := snap.State.Jobs[it.ID]
		if !it.IsPerson() || it.Kind != "approve" || j == nil || j.Status == contract.Cancelled || j.Status == contract.Rejected {
			continue
		}
		if j.Status == contract.Accepted {
			accepted = it.ID
			continue
		}
		if open == "" {
			open = it.ID
		}
		if !personJobAnswered(snap, j) && orchestrator.PersonReady(snap, it.ID, time.Now()) == nil {
			return store, it.ID
		}
	}
	if open != "" {
		return store, open
	}
	return store, accepted
}

// personJobAnswered reports whether the Feed holds an answer to the person Job j:
// a record of type answer whose re is the Job's first history record, whoever
// wrote it and whatever plan it carries.
func personJobAnswered(snap orchestrator.Snapshot, j *contract.Job) bool {
	if len(j.History) == 0 {
		return false
	}
	for _, r := range snap.Records {
		if r.Type == "answer" && r.Re != nil && *r.Re == j.History[0] {
			return true
		}
	}
	return false
}

// personApproval posts an approval as the result of the person Job: one
// accepting control with answer, plan_hash, plan_digest, lands_hash and note,
// written by one orchestrator.Write, so a failure leaves no half-written answer
// and a repeat press finds the Job accepted. The Feed record carries who and
// when. A Job already accepted with this digest gets nothing new: that check is
// made under .orchestrator.lock, in the same pass as the write, so two presses at
// once post one control. The caller holds no task or plan lock: the post takes
// .orchestrator.lock, and a tick takes that lock before .task.lock.
func personApproval(store feed.Store, w feed.Writer, job, answer string, e taskstore.Event) error {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = "approved"
	}
	body := map[string]any{"target": job, "status": "accepted", "reason": "answered by " + w.Author.ID, "answer": answer, "plan_hash": e.PlanHash, "plan_digest": e.PlanDigest}
	if e.LandsHash != "" {
		body["lands_hash"] = e.LandsHash
	}
	if e.Note != "" {
		body["note"] = e.Note
	}
	approved := func() bool {
		a, err := taskstore.ApprovalFromJob(store.TaskDir)
		return err == nil && a.Job == job && a.PlanDigest == e.PlanDigest
	}
	_, err := orchestrator.WriteUnless(store, w, feed.Input{Type: "control", Body: body}, approved)
	return err
}

// configureTickStore sets what a pass of the scheduler needs beyond the Feed and
// the writer: where Job worktrees live, how to judge a local process, and where
// a crash loop is recorded. job next and worker next build a task's Store here.
func configureTickStore(env Env, s *orchestrator.Store, project, taskID string) {
	// A project name the machine folder cannot hold leaves WorktreesDir empty: no path is computed.
	s.WorktreesDir, _ = env.MachineFolder.WorktreesDir(env.GridHome, project)
	s.Feed.Machine = env.MachineFolder
	s.Diagnostics = env.Stderr
	s.AccountRoom = yaAccountRoomFor(env)
	s.Probe = func(live session.Live) session.ProcessState {
		local := s.Writer.Worker[strings.LastIndexByte(s.Writer.Worker, '@'):]
		if !strings.HasSuffix(live.Holder, local) {
			return session.Unknown
		}
		return session.ProbeLocal(live)
	}
	s.Incident = func(job string, exits []string) error {
		name := "crash-" + strings.ToLower(job) + "-" + strings.ToLower(exits[len(exits)-1])
		bs := brain.Store{GridHome: env.GridHome, Project: project}
		if _, err := bs.Proposal("project", name); err == nil {
			return nil
		}
		_, err := bs.Propose(brain.Note{Name: name, Topic: "incidents", Type: "incident", Level: "project", Description: "Three attempts made no progress", Summary: "Inspect the failed attempts before retrying this Job.", Source: taskID + "/" + job, Body: strings.Join(exits, "\n")}, brain.Origin{Kind: "job", ID: job})
		return err
	}
}

func orchestratorRows(env Env, rows []orchestrator.Row, asJSON bool) int {
	if asJSON {
		return writeJSON(env.Stdout, rows)
	}
	if len(rows) == 0 {
		fmt.Fprintln(env.Stdout, "Nothing ready.")
		return 0
	}
	fmt.Fprintln(env.Stdout, "ID  ROLE  STATE  WHY")
	for _, r := range rows {
		fmt.Fprintf(env.Stdout, "%s  %s  %s  %s\n", r.ID, r.Role, r.State, r.Why)
	}
	return 0
}

// localMachine is this machine's id as a lease holder names it: the Env's, else the Grid
// home's. It is empty when neither is known.
func localMachine(env Env) string {
	if env.Machine != "" {
		return env.Machine
	}
	return gridhome.Machine(env.GridHome)
}

func orchestratorWriter(env Env, name, epoch string) (feed.Writer, error) {
	member, role := env.getenv("GRID_MEMBER"), contract.NormalizeRole(env.getenv("GRID_ROLE"))
	if member == "" {
		return feed.Writer{}, errors.New("GRID_MEMBER is required")
	}
	if role == "" {
		role = "member"
	}
	// A worker supervising a task performs mechanical CLI controls. Its Job
	// writes retain the worker role; this never grants person takeover authority.
	if name == "task" && role == "worker" {
		role = "cli"
	}
	machine := localMachine(env)
	if machine == "" {
		return feed.Writer{}, errors.New("machine id is missing; initialize the Grid home")
	}
	parts := strings.Split(epoch, ".")
	if len(parts) != 2 {
		return feed.Writer{}, errors.New("GRID_LEASE_EPOCH or --lease-epoch must be gen.fence; the task holder gets it from: grid lease take task")
	}
	gen, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil {
		return feed.Writer{}, e
	}
	fence, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil {
		return feed.Writer{}, e
	}
	return feed.Writer{Author: feed.Author{ID: member, Role: role}, Worker: member + "@" + machine, Lease: name, Gen: gen, Fence: fence}, nil
}
