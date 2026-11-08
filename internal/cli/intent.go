package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/intent"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runSessionCommand(args []string, env Env) int {
	if len(args) == 0 || args[0] == "list" || args[0] == "show" {
		return runSession(args, env)
	}
	switch args[0] {
	case "start", "stop", "send", "pause", "resume", "handback", "switch":
		return runIntentWrite(args[0], args[1:], env)
	default:
		fmt.Fprintln(env.Stderr, "Usage: grid session list|show|start|stop|send|pause|resume|handback|switch")
		return 2
	}
}

func runTaskIntent(args []string, env Env) int {
	if len(args) == 0 || args[0] != "move" {
		fmt.Fprintln(env.Stderr, "Usage: grid task move <machine> [--task <task>] [--project <project>] [--who <caller>] [--json]")
		return 2
	}
	return runIntentWrite("move", args[1:], env)
}

func intentScope(env Env, project, task string) (where.Result, error) {
	scope, err := livenessScope(env, project, task)
	if err != nil {
		return scope, err
	}
	if scope.TaskPath == "" {
		return scope, errors.New("no task selected; run inside a task folder or use --task")
	}
	return scope, nil
}

// A single positional argument precedes flags; --text handles messages beginning
// with a dash. Separate flag sets keep unrelated action flags from being accepted.
func runIntentWrite(verb string, args []string, env Env) int {
	value := ""
	if (verb == "send" || verb == "switch" || verb == "move") && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		value, args = args[0], args[1:]
	}
	fs := newFlags("intent "+verb, env)
	task := fs.String("task", "", "task folder or id; defaults to current task")
	project := fs.String("project", "", "project name or id")
	who := fs.String("who", "cli", "caller label for audit")
	asJSON := fs.Bool("json", false, "print JSON")
	target, agent, message, key, to := "", "", "", "", ""
	if verb != "move" {
		fs.StringVar(&target, "target", "", "session identifier; defaults to the selected task")
	}
	if verb == "start" || verb == "switch" {
		fs.StringVar(&agent, "agent", "", "agent to use")
	}
	if verb == "send" {
		fs.StringVar(&message, "text", "", "message text (or use a positional argument)")
		fs.StringVar(&key, "idempotency-key", "", "optional send retry key")
	}
	if verb == "move" {
		fs.StringVar(&to, "to", "", "destination worker; with a positional task selects that task")
	}
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	payload := map[string]string{}
	switch verb {
	case "send":
		if value != "" && message != "" {
			return intentUsage(env, "give send text once")
		}
		if value != "" {
			message = value
		}
		payload["text"] = message
	case "switch":
		if value != "" && agent != "" {
			return intentUsage(env, "give the agent once")
		}
		if value != "" {
			agent = value
		}
		payload["agent"] = agent
	case "start":
		if agent != "" {
			payload["agent"] = agent
		}
	case "move":
		target = value
		if to != "" {
			if value != "" {
				if *task != "" {
					return intentUsage(env, "give the task once")
				}
				*task = value
			}
			target = to
		}
		if strings.TrimSpace(target) == "" {
			return intentUsage(env, "move requires a destination machine")
		}
	}
	// Validate before resolving or writing scope; task is a placeholder here.
	checkTarget := target
	if checkTarget == "" {
		checkTarget = "task"
	}
	if err := intent.ValidateRequest(verb, checkTarget, payload, *who); err != nil {
		return intentUsage(env, err.Error())
	}
	scope, err := intentScope(env, *project, *task)
	if err != nil {
		return livenessError(env, err)
	}
	if target == "" {
		target = scope.Task
	}
	if verb == "move" {
		if err := movableProject(env, scope); err != nil {
			return livenessError(env, err)
		}
	}
	r, err := (intent.Store{TaskDir: scope.TaskPath}).Enqueue(verb, target, payload, *who, key)
	if err != nil {
		return livenessError(env, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, r)
	}
	fmt.Fprintf(env.Stdout, "intent: %s  verb: %s  target: %s\n", r.ID, r.Verb, r.Target)
	return 0
}

func intentUsage(env Env, message string) int {
	fmt.Fprintln(env.Stderr, "intent: "+message)
	return 2
}

// Resolve project code separately because the current folder is often the task
// data folder. No remote means the worker cannot transport the project's code.
func movableProject(env Env, scope where.Result) error {
	records, err := project.Load(env.GridHome)
	if err != nil {
		return err
	}
	var repos []string
	for _, record := range records {
		if record.Name != scope.Project {
			continue
		}
		for _, folder := range record.FoldersOn(env.Machine) {
			if len(record.Repos) == 0 {
				if root, ok := gitx.FindRoot(folder, env.Ceiling); ok {
					repos = append(repos, root)
				}
			}
			for _, repo := range record.Repos {
				path := repo.Path
				if !filepath.IsAbs(path) {
					path = filepath.Join(folder, path)
				}
				repos = append(repos, path)
			}
		}
	}
	if len(repos) == 0 {
		return errors.New(noMoveRemote)
	}
	for _, repo := range repos {
		if len(gitx.Remotes(repo)) == 0 {
			return errors.New(noMoveRemote)
		}
	}
	return nil
}

const noMoveRemote = "This project has no remote, so its code cannot reach the other machine. Add a remote (git remote add origin ...) or run the task here."

func runIntent(args []string, env Env) int {
	if len(args) == 0 {
		return intentUsage(env, "use list, claim <id>, renew <id> --token <token>, release <id> --token <token>, or done <id> --token <token> [--error[=message]]")
	}
	action, args := args[0], args[1:]
	if action != "list" && action != "claim" && action != "renew" && action != "release" && action != "done" {
		return intentUsage(env, "unknown action "+action)
	}
	id := ""
	if action != "list" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, args = args[0], args[1:]
	}
	fs := newFlags("intent "+action, env)
	task := fs.String("task", "", "task folder or id; defaults to current task")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	pending, who, token, failure := false, "", "", ""
	if action == "list" {
		fs.BoolVar(&pending, "pending", false, "only intents available to claim")
	}
	if action == "claim" {
		fs.StringVar(&who, "who", "worker", "worker label for audit")
	}
	if action == "renew" || action == "release" || action == "done" {
		fs.StringVar(&token, "token", env.getenv("GRID_INTENT_CLAIM"), "token returned by claim (or GRID_INTENT_CLAIM)")
		fs.StringVar(&token, "claim", env.getenv("GRID_INTENT_CLAIM"), "same as --token")
	}
	if action == "done" {
		// A bare --error means failed; --error=message also records the reason.
		fs.Var(&intentErrorFlag{value: &failure}, "error", "record failure; optionally --error=message")
	}
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if action != "list" && id == "" {
		return intentUsage(env, action+" requires an intent id before flags")
	}
	if (action == "renew" || action == "release" || action == "done") && token == "" {
		return intentUsage(env, action+" requires --token (or --claim) or GRID_INTENT_CLAIM from the successful claim")
	}
	scope, err := intentScope(env, *project, *task)
	if err != nil {
		return livenessError(env, err)
	}
	store := intent.Store{TaskDir: scope.TaskPath}
	switch action {
	case "list":
		rows, err := store.List(pending)
		if err != nil {
			return livenessError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, rows)
		}
		if len(rows) == 0 {
			fmt.Fprintln(env.Stdout, "No intents.")
		}
		for _, r := range rows {
			fmt.Fprintf(env.Stdout, "intent: %s  verb: %s  target: %s  state: %s\n", r.ID, r.Verb, r.Target, r.State)
		}
	case "claim":
		claim, err := store.Claim(id, who)
		if err != nil {
			livenessError(env, err)
			return exitCode(err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, claim)
		}
		fmt.Fprintf(env.Stdout, "intent: %s  claim: %s  until: %s\n", id, claim.Token, claim.Until.Format("2006-01-02T15:04:05.000Z"))
	case "renew":
		claim, err := store.Renew(id, token)
		if err != nil {
			livenessError(env, err)
			return exitCode(err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, claim)
		}
		fmt.Fprintf(env.Stdout, "intent: %s  claim: %s  until: %s\n", id, claim.Token, claim.Until.Format("2006-01-02T15:04:05.000Z"))
	case "release":
		released, err := store.Release(id, token)
		if err != nil {
			livenessError(env, err)
			return exitCode(err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, released)
		}
		fmt.Fprintf(env.Stdout, "intent: %s  state: pending\n", id)
	case "done":
		done, err := store.Done(id, token, failure)
		if err != nil {
			livenessError(env, err)
			return exitCode(err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, done)
		}
		fmt.Fprintf(env.Stdout, "intent: %s  state: done\n", id)
	}
	return 0
}

type intentErrorFlag struct{ value *string }

var _ flag.Value = (*intentErrorFlag)(nil)

func (f *intentErrorFlag) String() string   { return *f.value }
func (f *intentErrorFlag) IsBoolFlag() bool { return true }
func (f *intentErrorFlag) Set(value string) error {
	if value == "true" {
		value = "worker reported an error"
	}
	if value == "false" {
		value = ""
	}
	*f.value = value
	return nil
}
