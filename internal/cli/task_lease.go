package cli

import (
	"fmt"
	"slices"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runTaskLease(args []string, env Env) int {
	if len(args) < 2 || args[0] != "take" || args[1] != "task" {
		return runLease(args, env)
	}
	fs := newFlags("lease take task", env)
	task := fs.String("task", "", "task folder or id")
	project := fs.String("project", "", "project name or id")
	pid := fs.Int("pid", 0, "supervising process id (required)")
	start := fs.String("pid-start", "", "supervising process start identity (required)")
	gen := fs.Int64("gen", 0, "task generation")
	adopt := fs.Bool("adopt", false, "adoption of a provably gone local session")
	takeover := fs.Bool("takeover", false, "person-authorized takeover after 15 minutes offline")
	expected := fs.String("expected-epoch", "", "current generation.fence required for adoption or takeover")
	offline := fs.String("server-offline-since", "", "server-observed offline time in RFC3339; never infer from lease age")
	asJSON := fs.Bool("json", false, "print the granted tenure")
	if code, stop := parse(fs, args[2:], env); stop {
		return code
	}
	fail := func(err error) int { fmt.Fprintln(env.Stderr, err); return 1 }
	if *pid <= 0 || *start == "" || *adopt && *takeover {
		return fail(fmt.Errorf("--pid and --pid-start are required; select at most one of --adopt and --takeover"))
	}
	writer, err := orchestratorWriter(env, "task", fmt.Sprintf("%d.0", *gen))
	if err != nil {
		return fail(err)
	}
	typ, _ := contract.LookupType("handover")
	if !slices.Contains(typ.Roles, writer.Author.Role) {
		return fail(fmt.Errorf("task acquisition requires a task-holder role"))
	}
	policy := lease.TaskTake{Adopt: *adopt, Takeover: *takeover}
	if *adopt || *takeover {
		old, err := orchestratorWriter(env, "task", *expected)
		if err != nil {
			return fail(err)
		}
		policy.Gen, policy.Fence = old.Gen, old.Fence
	}
	if *offline != "" {
		policy.OfflineSince, err = time.Parse(time.RFC3339Nano, *offline)
		if err != nil {
			return fail(err)
		}
	}
	scope, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *task, Project: *project})
	if err != nil {
		return fail(err)
	}
	if scope.TaskPath == "" {
		return fail(fmt.Errorf("no task selected; use --task"))
	}
	id, _, err := taskstore.FeedIdentity(scope.TaskPath)
	if err != nil {
		return fail(err)
	}
	store := feed.Store{TaskDir: scope.TaskPath, TaskID: id}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(writer, *pid, *start, policy)
	if err != nil {
		return fail(err)
	}
	writer.Fence = grant.Fence
	body := map[string]any{}
	if *adopt {
		body["adopted"] = "session-gone"
	}
	if *takeover {
		body["reason"] = "person takeover after server-observed offline interval"
	}
	if _, err := store.Handover(feed.Input{Body: body}, writer); err != nil {
		return fail(err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, grant)
	}
	fmt.Fprintf(env.Stdout, "task lease taken, epoch %d.%d\n", grant.Gen, grant.Fence)
	return 0
}
