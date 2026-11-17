package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/judge"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const judgeUsage = `usage: grid judge <verb> <job> [flags]

The mechanical part of judging a Job. The orchestrator reads a verdict and chooses the
next verb; each verb posts only what that choice needs, under the task lease the worker
gave it, and decides nothing itself.

  verdict <review>              the review's verdict (ACCEPTED, REJECTED or STUCK), its
                                must-fix, the attempt it was given for, and the review
                                rounds used over the reviewed Job's continues chain; posts nothing
  review <job>                  post the review Job of a submitted Job, based on the SHA it
                                submitted for its current attempt (--agent checker for a checker)
  rework <job> --from <review>  post the rework Job, which continues <job>, and the control that
                                rejects <job> as reworked by it
  person <job>                  post the person Job for the task owner: --from <review> after a
                                second rejection, STUCK, or a review cancelled before it ran;
                                --point <step> for a person point of the workflow

--wake <event>/<n> names the event a call answers: a second call with a wake already
answered posts nothing and exits 0. A post's id comes from what it says, so a call that
runs again after a crash makes the same post and the Feed keeps one.
`

// judgePost is one post of a judge verb, as it prints: posted, or not posted and why.
type judgePost struct {
	ID     string `json:"id"`
	Type   string `json:"type"`
	Job    string `json:"job"`
	Why    string `json:"why,omitempty"`
	Record string `json:"record,omitempty"`
}

func runJudge(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprint(env.Stderr, judgeUsage)
		return 2
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(env.Stdout, judgeUsage)
		return 0
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "verdict", "review", "rework", "person":
	default:
		fmt.Fprintf(env.Stderr, "grid judge: unknown verb %q\n%s", verb, judgeUsage)
		return 2
	}
	id := ""
	if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		id, rest = rest[0], rest[1:]
	}
	fs := newFlags("judge "+verb, env)
	fs.Usage = func() { fmt.Fprint(fs.Output(), judgeUsage); fs.PrintDefaults() }
	task := fs.String("task", "", "task folder or id")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	var wake, from, target, agent, point, owner *string
	rounds := new(int)
	switch verb {
	case "verdict":
		rounds = fs.Int("rounds", 0, "the workflow's review rounds, printed beside rounds_used; 0 prints null")
	case "review":
		wake = fs.String("wake", env.getenv("GRID_WAKE"), "event/n this call answers")
		agent = fs.String("agent", "", "the reviewing agent; checker or grid-checker posts a checker Job")
	case "rework":
		wake = fs.String("wake", env.getenv("GRID_WAKE"), "event/n this call answers")
		from = fs.String("from", "", "the review Job that rejected the Job (required)")
		target = fs.String("target", "", "the branch the rework starts from when the task has no head")
	case "person":
		wake = fs.String("wake", env.getenv("GRID_WAKE"), "event/n this call answers")
		from = fs.String("from", "", "the review Job the person decides after")
		point = fs.String("point", "", "the workflow step a person point comes before")
		owner = fs.String("owner", env.getenv("GRID_OWNER"), "the person's member id; defaults to the task owner")
	}
	if code, stop := parse(fs, rest, env); stop {
		return code
	}
	if id == "" {
		fmt.Fprintf(env.Stderr, "grid judge %s needs a Job id\n", verb)
		return 2
	}
	if verb == "rework" && *from == "" {
		fmt.Fprintln(env.Stderr, "grid judge rework needs --from <review>")
		return 2
	}
	fail := func(err error) int {
		fmt.Fprintln(env.Stderr, err)
		return exitCode(err)
	}
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
	if verb != "verdict" {
		if _, err := feed.EnsureTaskID(scope.TaskPath); err != nil {
			return fail(err)
		}
	}
	taskID, taskOwner, err := taskstore.FeedIdentity(scope.TaskPath)
	if err != nil {
		return fail(err)
	}
	store := feed.Store{TaskDir: scope.TaskPath, TaskID: taskID, Warn: env.Stderr}
	snap, err := orchestrator.Load(store)
	if err != nil {
		return fail(err)
	}
	st := judge.State{Task: taskID, Jobs: snap.State.Jobs, Bodies: snap.Bodies, Records: snap.Records, IDs: snap.JobIDs}
	if snap.State.Head != nil {
		st.Head = *snap.State.Head
	}

	var plan judge.Plan
	switch verb {
	case "verdict":
		rep, err := judge.Verdict(st, id)
		if err != nil {
			return fail(err)
		}
		if *rounds > 0 {
			rep.Rounds = rounds
		}
		if *asJSON {
			return writeJSON(env.Stdout, rep)
		}
		printVerdict(env, rep)
		return 0
	case "review":
		plan, err = judge.Review(st, id, *wake, *agent)
	case "rework":
		plan, err = judge.Rework(st, id, *from, *wake, *target)
	case "person":
		if *owner == "" {
			*owner = taskOwner
		}
		plan, err = judge.Person(st, id, *from, *owner, *wake, *point)
	}
	if err != nil {
		return fail(err)
	}

	// Every post is a task write: it goes under the task lease, with the task epoch the worker
	// gave the agent, as a Job post by grid job post does.
	epoch := env.getenv("GRID_TASK_EPOCH")
	if epoch == "" {
		epoch = env.getenv("GRID_LEASE_EPOCH")
	}
	var writer feed.Writer
	if len(plan.Posts) > 0 {
		if writer, err = orchestratorWriter(env, "task", epoch); err != nil {
			return fail(err)
		}
	}
	posted, skipped := []judgePost{}, []judgePost{}
	for _, p := range plan.Posts {
		post := judgePost{ID: p.ID(), Type: p.Type, Job: p.Job}
		if rec, ok := st.Answered(p); ok {
			post.Why, post.Record = "wake "+p.Wake+" is already answered", rec.ID
			skipped = append(skipped, post)
			continue
		}
		if _, ok := snap.Records[post.ID]; ok {
			post.Why, post.Record = "the same post is already on the Feed", post.ID
			skipped = append(skipped, post)
			continue
		}
		record, err := orchestrator.Write(store, writer, feed.Input{ID: post.ID, Type: p.Type, Body: p.Body, Job: p.Job, Wake: p.Wake})
		if errors.Is(err, orchestrator.ErrDuplicateWake) {
			post.Why, post.Record = "wake "+p.Wake+" is already answered", record.ID
			skipped = append(skipped, post)
			continue
		}
		if err != nil {
			if len(posted) > 0 {
				refoldAfterWrite(env, store)
			}
			return fail(err)
		}
		post.ID = record.ID
		posted = append(posted, post)
	}
	if len(posted) > 0 {
		refoldAfterWrite(env, store)
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Posted        []judgePost `json:"posted"`
			NothingPosted []judgePost `json:"nothing_posted"`
			Note          string      `json:"note,omitempty"`
		}{posted, skipped, plan.Note})
	}
	for _, p := range posted {
		fmt.Fprintf(env.Stdout, "posted %s %s: %s\n", p.Type, p.Job, p.ID)
	}
	for _, p := range skipped {
		fmt.Fprintf(env.Stdout, "nothing posted for %s %s: %s by record %s\n", p.Type, p.Job, p.Why, p.Record)
	}
	if plan.Note != "" {
		fmt.Fprintln(env.Stdout, "nothing posted: "+plan.Note)
	}
	return 0
}

func printVerdict(env Env, rep judge.Report) {
	verdict := rep.Verdict
	if verdict == "" {
		verdict = "no verdict"
	}
	line := fmt.Sprintf("%s: %s", rep.Review, verdict)
	if rep.Result != "" {
		line += fmt.Sprintf(" (result %s, attempt %d of %d)", rep.Result, rep.ReviewAttempt, rep.Attempt)
	}
	if rep.Stale {
		line += "; stale: the result is from an older attempt of the review"
	}
	if rep.Reviewed != "" {
		line += fmt.Sprintf("; reviews %s, rounds used %d", rep.Reviewed, rep.RoundsUsed)
		if rep.Rounds != nil {
			line += fmt.Sprintf(" of %d", *rep.Rounds)
		}
	}
	fmt.Fprintln(env.Stdout, line)
	for _, m := range rep.MustFix {
		fmt.Fprintln(env.Stdout, "  must-fix: "+m)
	}
}
