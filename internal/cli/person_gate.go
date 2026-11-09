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
	// Hook execution must remain available even with broken policy. The
	// explicit settings mutation still belongs to the person-only floor.
	if len(args) == 0 || (args[0] == "hook" && (len(args) < 2 || args[1] != "level")) {
		return 0
	}
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
	var rolePolicies [][]string
	var linkedErr error
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
	// Signal operands are signal IDs; only --task can select a task. Other
	// option values must not accidentally turn into task selectors.
	if args[0] == "signal" {
		query.Task = personFlag(args, "task", "")
	}
	// Selectors use the same last-value semantics as command flags.
	query.Project = personFlag(args, "project", "")
	r := projectStore(*env).Resolver
	res, err := r.Resolve(query)
	if err != nil && args[0] == "signal" && query.Task != "" {
		// A missing task cannot bypass the signal project's policy. The
		// signal handler retains its own validation of the task operand.
		query.Task = ""
		res, err = r.Resolve(query)
	}
	if err == nil {
		matchArgs := args
		if verb == "signal act" {
			matchArgs = []string{"signal", "act", personFlag(args, "act", "note")}
		} else if verb == "signal dismiss" {
			matchArgs = []string{"signal", "act", "dismiss"}
		}
		warned := false
		match := ""
		apply := func(policy map[string][]string, policyErr error) {
			if errors.Is(policyErr, workflow.ErrLinkedFile) {
				// A linked workflows.md may be forged, so neither it nor the
				// floor alone decides: the default policy applies and every
				// person verb refuses below until the link is removed.
				if linkedErr == nil {
					linkedErr = policyErr
				}
				policy, policyErr = workflow.DefaultPersonVerbs(), nil
			}
			if policyErr != nil {
				// A broken extension cannot remove the floor or another scope's policy.
				if errors.Is(policyErr, workflow.ErrPersonVerbs) && !warned {
					fmt.Fprintln(env.Stderr, "grid: warning: invalid person_verbs; ignoring the key and keeping the person-only floor")
					warned = true
				}
				return
			}
			localMatch := ""
			for v := range policy {
				extra = append(extra, v)
				words := strings.Fields(v)
				if len(words) > 0 && len(matchArgs) >= len(words) && slices.Equal(matchArgs[:len(words)], words) && len(v) > len(localMatch) {
					localMatch = v
				}
			}
			if localMatch != "" {
				if policy[localMatch] != nil {
					rolePolicies = append(rolePolicies, policy[localMatch])
				}
				if len(localMatch) > len(match) {
					match = localMatch
				}
			}
		}
		// Signal policy follows project -> workspace -> defaults even when a
		// task is selected; the task can add requirements, never replace them.
		if res.Project != "" && (args[0] == "signal" || res.TaskPath == "") {
			apply(workflow.ProjectPersonVerbs(r.GridHome, res.Project))
		}
		if res.TaskPath != "" {
			taskID = res.Task
			apply(workflow.PersonVerbs(res.TaskPath))
		}
		if match != "" {
			verb = match
		}
	}

	if !persongate.Required(verb, extra) {
		return 0
	}
	if linkedErr != nil {
		fmt.Fprintf(env.Stderr, "grid %s: %v\n", personDisplayVerb(verb), linkedErr)
		env.recordError(linkedErr)
		return 1
	}
	request := gateRequest(*env, verb, args)
	request.ExtraVerbs = extra
	if taskID != "" {
		request.Task = taskID
	}
	d, _ := persongate.Check(request)
	if !d.Allow {
		return personRefusal(*env, personDisplayVerb(verb), d.Reason, jsonRequested(args))
	}
	for _, roles := range rolePolicies {
		role := strings.ToLower(strings.TrimSpace(env.getenv("GRID_ROLE")))
		if role == "" {
			role = "member"
		}
		if !slices.Contains(roles, role) {
			return personRefusal(*env, personDisplayVerb(verb), "agent_env", jsonRequested(args))
		}
	}
	env.personVia = d.Via
	return 0
}

// Keep the A60 command label even when policy/receipts name a specific act.
func personDisplayVerb(verb string) string {
	if strings.HasPrefix(verb, "signal act ") {
		return "signal act"
	}
	return verb
}

// personFlag recognizes both Go flag spellings and stops at the option fence.
// Skip values of other signal options so text cannot masquerade as a selector.
func personFlag(args []string, key, fallback string) string {
	value := fallback
	for i := 1; i < len(args); i++ {
		if args[i] == "--" {
			break
		}
		if !strings.HasPrefix(args[i], "-") {
			continue
		}
		name, v, equal := strings.Cut(strings.TrimLeft(args[i], "-"), "=")
		if name == key {
			if equal {
				value = v
			} else if i+1 < len(args) {
				i++
				value = args[i]
			}
		} else if !equal && slices.Contains([]string{"project", "act", "task", "until", "source", "source-id", "title", "body", "to"}, name) {
			i++
		}
	}
	return value
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
