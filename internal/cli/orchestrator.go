package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/session"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

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
	fs := newFlags("job "+verb, env)
	task := fs.String("task", "", "task folder or id")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	wait := fs.Bool("wait", false, "wait for an assignment or judgement event")
	if id == "" {
		id = env.getenv("GRID_JOB")
	}
	job := fs.String("job", id, "Job id")
	kind := fs.String("type", "post", "registered post type")
	body := fs.String("body", "{}", "JSON body, or - for stdin")
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
	if code, stop := parse(fs, rest, env); stop {
		return code
	}
	fail := func(err error) int {
		fmt.Fprintln(env.Stderr, err)
		// The body parser reports this before a record reaches the shared writer.
		if err.Error() == "--body must be a JSON object" {
			err = &feed.InvalidRecordError{Err: err}
		}
		return exitCode(err)
	}
	if verb == "post" {
		if err := refuseAnswerPost(*kind); err != nil {
			return fail(err)
		}
	}
	scope, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *task, Project: *project})
	if err != nil {
		return fail(err)
	}
	if scope.TaskPath == "" {
		return fail(errors.New("no task selected; use --task"))
	}
	taskID, taskOwner, err := taskstore.FeedIdentity(scope.TaskPath)
	if err != nil {
		return fail(err)
	}
	if *owner == "" {
		*owner = taskOwner
	}
	store := feed.Store{TaskDir: scope.TaskPath, TaskID: taskID}
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
					fmt.Fprintf(env.Stdout, "  attempt %d  fence %d  %s  %s\n", a.Number, a.Fence, a.State, a.Exit)
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
	leaseName := "task"
	if verb == "ack" || verb == "result" || verb == "post" {
		leaseName = *job
		if leaseName == "" {
			leaseName = env.getenv("GRID_LEASE")
		}
	}
	if verb == "post" && (*kind == "job" || *kind == "control") {
		leaseName = "task"
	}
	if *leaseFlag != "" {
		leaseName = *leaseFlag
	}
	writer, err := orchestratorWriter(env, leaseName, *epoch)
	if err != nil {
		return fail(err)
	}
	if verb == "next" || verb == "tick" {
		s := orchestrator.Store{AfterWake: *afterWake, Feed: store, Writer: writer, Assignee: *to, Owner: *owner, WorktreesDir: filepath.Join(env.GridHome, "projects", scope.Project, "worktrees")}
		s.Probe = func(live session.Live) session.ProcessState {
			local := writer.Worker[strings.LastIndexByte(writer.Worker, '@'):]
			if !strings.HasSuffix(live.Holder, local) {
				return session.Unknown
			}
			return session.ProbeLocal(live)
		}
		if *usageReady {
			s.UsageReady = func(worker string) bool { return worker == *to || *to == "" && worker == writer.Worker }
		}
		s.Incident = func(job string, exits []string) error {
			name := "crash-" + strings.ToLower(job) + "-" + strings.ToLower(exits[len(exits)-1])
			bs := brain.Store{GridHome: env.GridHome, Project: scope.Project}
			if _, err := bs.Proposal("project", name); err == nil {
				return nil
			}
			_, err := bs.Propose(brain.Note{Name: name, Topic: "incidents", Type: "incident", Level: "project", Description: "Three attempts made no progress", Summary: "Inspect the failed attempts before retrying this Job.", Source: taskID + "/" + job, Body: strings.Join(exits, "\n")}, brain.Origin{Kind: "job", ID: job, Tier: contract.Agent})
			return err
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
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
	if *body == "-" {
		b, err := io.ReadAll(env.Stdin)
		if err != nil {
			return fail(err)
		}
		*body = string(b)
	}
	obj := map[string]any{}
	if err := json.Unmarshal([]byte(*body), &obj); err != nil || obj == nil {
		return fail(errors.New("--body must be a JSON object"))
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
	record, err := orchestrator.Write(store, writer, feed.Input{ID: *recordID, Type: typ, Body: obj, Job: *job, Attempt: *attempt, Wake: *wake, Re: *re})
	if errors.Is(err, orchestrator.ErrDuplicateWake) {
		fmt.Fprintln(env.Stdout, record.ID)
		return fail(err)
	}
	if err != nil {
		return fail(err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, record)
	}
	fmt.Fprintln(env.Stdout, record.ID)
	return 0
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
func orchestratorWriter(env Env, name, epoch string) (feed.Writer, error) {
	member, role := env.getenv("GRID_MEMBER"), env.getenv("GRID_ROLE")
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
	machine := env.Machine
	if machine == "" {
		machine = gridhome.Machine(env.GridHome)
	}
	if machine == "" {
		return feed.Writer{}, errors.New("machine id is missing; initialize the Grid home")
	}
	parts := strings.Split(epoch, ".")
	if len(parts) != 2 {
		return feed.Writer{}, errors.New("GRID_LEASE_EPOCH or --lease-epoch must be gen.fence")
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
