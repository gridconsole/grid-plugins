package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Reasons of a refused person check. The app shows the reason word as given.
const (
	personCheckWorker = "agent_worker"
	personCheckRole   = "agent_role"
)

const personCheckHint = "approvals need a person; this caller is an agent"

// personCheckReason answers from the environment the caller already carries and
// from the same role tiers the stores use: an empty role or the local cli is the
// machine's owner, owner, member, lead, guest and external roles are people, and
// any other role, or a worker marker, is an agent. It returns "" for a person.
func personCheckReason(env Env) string {
	if workerMarked(env) {
		return personCheckWorker
	}
	role := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	if role == "" || role == "cli" || contract.TierFor(role) != contract.Agent {
		return ""
	}
	return personCheckRole
}

// runPersonCheck is `grid person check [--json]`, the app's startup question
// whether approvals may be offered. It reads the environment only and writes
// nothing; it is an answer, and no other command consults it.
// Exit 0 with {"ok":true} for a person, exit 3 with the person_required object
// for an agent.
func runPersonCheck(args []string, env Env) int {
	fs := newFlags("person check", env)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parsePositionals(fs, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "person check: expected 0 arguments")
		return 2
	}
	reason := personCheckReason(env)
	if reason == "" {
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				OK bool `json:"ok"`
			}{true})
		}
		fmt.Fprintln(env.Stdout, "person check: ok")
		return 0
	}
	line := fmt.Sprintf("grid person check: %s (reason: %s).", personCheckHint, reason)
	fmt.Fprintln(env.Stderr, line)
	if *asJSON {
		_ = writeJSON(env.Stdout, struct {
			Error   string `json:"error"`
			Reason  string `json:"reason"`
			Hint    string `json:"hint"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}{"person_required", reason, personCheckHint, "person_required", line})
	}
	return 3
}
