package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// roleRefusalHint and the "role_not_allowed" reason word the role-list refusal.
// The JSON error code (person_required) and exit status (3) are unchanged.
const roleRefusalHint = "your role is not in this workflow's list for this command"

func roleRefusal(env Env, verb string, allowed []string, asJSON bool) int {
	const reason = "role_not_allowed"
	roleRefusalHint := roleRefusalHint
	if len(allowed) > 0 {
		roleRefusalHint += "; allowed: " + strings.Join(allowed, ", ")
	}
	line := fmt.Sprintf("grid %s: %s (reason: %s).", verb, roleRefusalHint, reason)
	fmt.Fprintln(env.Stderr, line)
	if asJSON {
		_ = writeJSON(env.Stdout, struct {
			Error   string `json:"error"`
			Reason  string `json:"reason"`
			Hint    string `json:"hint"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}{"person_required", reason, roleRefusalHint, "person_required", line})
	}
	return 3
}

// workflowRoleRefusal enforces the role lists of a workflow's person_verbs map
// (person_verbs: {answer: [member]}) before the verb runs: GRID_ROLE, default
// member, must be in the list of every policy that names the verb. It reads the
// role only; it does not look at the process, the environment's agent markers
// or the caller's ancestry. It returns 0 to let the verb run, or the exit code
// after printing the refusal.
func workflowRoleRefusal(args []string, env Env) int {
	// Hook execution must stay available even with a broken policy.
	if len(args) == 0 || (args[0] == "hook" && (len(args) < 2 || args[1] != "level")) {
		return 0
	}
	verb := args[0]
	if len(args) > 1 && !strings.HasPrefix(args[1], "-") {
		verb += " " + args[1]
	}
	// Only the standalone help form is exempt; a --help used as a flag value
	// must not exempt an otherwise listed verb.
	words := len(strings.Fields(verb))
	if len(args) == words+1 && (args[words] == "--help" || args[words] == "-h") {
		return 0
	}
	if verb == "question withdraw" && env.getenv("GRID_LEASE") == "task" {
		// A task lease holder is checked by the question store and the feed.
		return 0
	}
	// Task-local policies resolve through the caller-aware source, including
	// --task and an explicit task operand.
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
	// Signal operands are signal IDs; only --task can select a task.
	if args[0] == "signal" {
		query.Task = roleFlag(args, "task", "")
	}
	query.Project = roleFlag(args, "project", "")
	r := projectStore(env).Resolver
	res, err := r.Resolve(query)
	if err != nil && args[0] == "signal" && query.Task != "" {
		// A missing task cannot bypass the signal project's policy.
		query.Task = ""
		res, err = r.Resolve(query)
	}
	if err != nil {
		return 0
	}
	matchArgs := args
	if verb == "signal act" {
		matchArgs = []string{"signal", "act", roleFlag(args, "act", "note")}
	} else if verb == "signal dismiss" {
		matchArgs = []string{"signal", "act", "dismiss"}
	}
	var roleLists [][]string
	match := ""
	apply := func(policy map[string][]string, policyErr error) {
		if policyErr != nil {
			// A policy that cannot be read adds no role rule and removes no other
			// scope's; grid workflow show reports the problem itself.
			return
		}
		local := ""
		for v := range policy {
			w := strings.Fields(v)
			if len(w) > 0 && len(matchArgs) >= len(w) && slices.Equal(matchArgs[:len(w)], w) && len(v) > len(local) {
				local = v
			}
		}
		if local == "" {
			return
		}
		if policy[local] != nil {
			roleLists = append(roleLists, policy[local])
		}
		if len(local) > len(match) {
			match = local
		}
	}
	// Signal policy follows project -> workspace -> defaults even when a task is
	// selected; the task can add requirements, never replace them.
	if res.Project != "" && (args[0] == "signal" || res.TaskPath == "") {
		apply(workflow.ProjectPersonVerbs(r.GridHome, res.Project))
	}
	if res.TaskPath != "" {
		apply(workflow.PersonVerbs(res.TaskPath))
	}
	role := strings.ToLower(strings.TrimSpace(env.getenv("GRID_ROLE")))
	if role == "" {
		role = "member"
	}
	for _, roles := range roleLists {
		if !slices.Contains(roles, role) {
			return roleRefusal(env, roleDisplayVerb(match), roles, jsonRequested(args))
		}
	}
	return 0
}

// roleDisplayVerb keeps the command label when a policy names a specific act.
func roleDisplayVerb(verb string) string {
	if strings.HasPrefix(verb, "signal act ") {
		return "signal act"
	}
	return verb
}

// roleFlag recognizes both Go flag spellings and stops at the option fence.
// Skip values of other signal options so text cannot masquerade as a selector.
func roleFlag(args []string, key, fallback string) string {
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
		} else if !equal && slices.Contains([]string{"project", "act", "task", "until", "source", "source-id", "title", "sender", "recipient", "body", "to"}, name) {
			i++
		}
	}
	return value
}
