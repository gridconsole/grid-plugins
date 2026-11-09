package cli

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/persongate"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

const personHint = "a person must run this, from the Grid app or your own terminal outside Claude Code or Codex"

func personRefusal(env Env, verb, reason string, json bool) int {
	fmt.Fprintf(env.Stderr, "grid %s: %s (reason: %s).\n", verb, personHint, reason)
	if json {
		_ = writeJSON(env.Stdout, struct {
			Error  string `json:"error"`
			Reason string `json:"reason"`
			Hint   string `json:"hint"`
		}{"person_required", reason, personHint})
	}
	return 3
}
func gateRequest(env Env, verb string, args []string) persongate.Request {
	task := ""
	for i, a := range args {
		if (a == "--task" || a == "-task") && i+1 < len(args) {
			task = args[i+1]
			break
		}
		if strings.HasPrefix(a, "T-") {
			task = a
			break
		}
	}
	return persongate.Request{Verb: strings.Fields(verb), Argv: args, Task: task, Getenv: env.getenv, Procs: env.Procs, PersonDir: env.PersonDir}
}
func jsonRequested(args []string) bool {
	return slices.Contains(args, "--json") || slices.Contains(args, "-json") || slices.Contains(args, "--json=true") || slices.Contains(args, "-json=true")
}

// gateCommand is called before dispatch, so aliases and settings writes share
// the same floor. Store callers (deliver and the orchestrator) remain independent.
func gateCommand(args []string, env *Env) int {
	verb := args[0]
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		verb += " " + args[1]
	}
	for _, v := range persongate.Floor() {
		words := strings.Fields(v)
		if len(args) >= len(words) && slices.Equal(args[:len(words)], words) {
			verb = v
			break
		}
	}
	// Hook handlers must reach their decision protocol even when task policy is
	// invalid or tries to gate hooks. Only hook level is in the person-only floor.
	if args[0] == "hook" && !persongate.Required(verb, nil) {
		return 0
	}
	// Only standalone help forms bypass the gate; a --help used as a flag value
	// must not exempt an otherwise valid write.
	words := len(strings.Fields(verb))
	if len(args) == words+1 && (args[words] == "--help" || args[words] == "-h") {
		return 0
	}
	if verb == "setup" && settings.ClaudeSetupCommand(args[1:]) && !env.personSetupWrite {
		return 0
	}
	if verb == "question withdraw" && env.getenv("GRID_LEASE") == "task" {
		// question.Store and Feed validate the task lease and fence before writes.
		return 0
	}
	var extra []string
	var roles []string
	taskID := ""
	// Task-local workflows may extend the floor. Resolve through the existing
	// caller-aware source, including --task and an explicit task operand.
	query := where.Query{Cwd: env.Cwd}
	for i, a := range args {
		if (a == "--task" || a == "-task") && i+1 < len(args) {
			query.Task = args[i+1]
			break
		}
		if strings.HasPrefix(a, "T-") {
			query.Task = a
			break
		}
	}
	if res, err := resolver(*env).Resolve(query); err == nil && res.TaskPath != "" {
		taskID = res.Task
		policy, err := workflow.PersonVerbs(res.TaskPath)
		if errors.Is(err, workflow.ErrPersonVerbs) {
			fmt.Fprintln(env.Stderr, "grid: warning: invalid person_verbs; ignoring the key and keeping the person-only floor")
		}
		if err != nil {
			// Existing workflow readers retain their diagnostics and read-only behavior.
			// No workflow failure can remove a floor verb.
			policy = nil
		}
		match := ""
		for v := range policy {
			extra = append(extra, v)
			words := strings.Fields(v)
			if len(words) > 0 && len(args) >= len(words) && slices.Equal(args[:len(words)], words) && len(v) > len(match) {
				match = v
			}
		}
		if match != "" {
			verb = match
			roles = policy[match]
		}
	}
	if !persongate.Required(verb, extra) {
		return 0
	}
	request := gateRequest(*env, verb, args)
	request.ExtraVerbs = extra
	if taskID != "" {
		request.Task = taskID
	}
	d, _ := persongate.Check(request)
	if !d.Allow {
		return personRefusal(*env, verb, d.Reason, jsonRequested(args))
	}
	if roles != nil {
		role := strings.ToLower(strings.TrimSpace(env.getenv("GRID_ROLE")))
		if role == "" {
			role = "member"
		}
		if !slices.Contains(roles, role) {
			return personRefusal(*env, verb, "agent_env", jsonRequested(args))
		}
	}
	env.personVia = d.Via
	return 0
}
func runPersonCheck(args []string, env Env) int {
	fs := newFlags("person check", env)
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	// Probe the least privileged person write; each actual command still checks
	// its own verb and workflow roles, and the store enforces question scope.
	d, _ := persongate.Check(gateRequest(env, "answer", append([]string{"person", "check"}, args...)))
	if *asJSON {
		if d.Allow {
			_ = writeJSON(env.Stdout, d)
		} else {
			_ = writeJSON(env.Stdout, struct {
				persongate.Decision
				Error string `json:"error"`
				Hint  string `json:"hint"`
			}{d, "person_required", personHint})
		}
	} else {
		state := "refuse"
		if d.Allow {
			state = "allow"
		}
		fmt.Fprintf(env.Stdout, "%s reason=%s via=%s\n", state, d.Reason, d.Via)
	}
	if !d.Allow {
		fmt.Fprintf(env.Stderr, "grid check: %s (reason: %s).\n", personHint, d.Reason)
		return 3
	}
	return 0
}
