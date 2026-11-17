package cli

import (
	"fmt"
	"strings"
)

// hsIpVerb is the in-process answer for one verb of commands().
type hsIpVerb struct {
	// why, when set, makes the whole verb binary-only.
	why string
	// subs, when not nil, lists every subverb the verb takes; a subverb it does
	// not name answers false.
	subs map[string]hsIpSub
}

// hsIpSub is the answer for one subverb.
type hsIpSub struct {
	// why, when set, makes the subverb binary-only.
	why string
	// flags names the flags (without dashes) that make the subverb binary-only.
	flags map[string]string
}

// hsIpTable answers, per verb of commands(), whether a call may run inside a host
// process through lib.Run. A binary-only entry is one that waits on a signal of
// the process, records the process id as an owner, serves or reads the host's
// own streams, opens the host's browser, starts a child with the host's
// environment, or writes a system schedule for the host's executable. Every
// other verb reads and writes the Grid home, or takes its signals, children and
// owner from the Call, and answers yes.
var hsIpTable = map[string]hsIpVerb{
	"version": {},
	"where":   {},
	// cli.go:132 and open.go:67: opens the system browser and may start the app.
	"open":   {why: "opens the browser and may start the app (open.go:67, open_start.go:216)"},
	"doctor": {},
	// cli.go:134 and hook.go:19: a hook answers a Claude Code or Codex event on stdin.
	"hook": {why: "answers a host hook event on stdin (cli.go:134, hook.go:19)"},
	"task": {subs: map[string]hsIpSub{
		// task_new_first_job.go:84-86 takes the Call's OwnerPID as the first Job's
		// owner, and task.go:125 names the task's owner from the Call's Getenv.
		"new":     {},
		"show":    {},
		"list":    {},
		"status":  {},
		"stop":    {},
		"close":   {},
		"reopen":  {},
		"history": {},
		// task.go:241 records os.Getppid as the attached session.
		"attach":  {why: "records the parent process id as the attached session (task.go:241)"},
		"move":    {},
		"decline": {},
		"delete":  {},
		"rename":  {},
		"restore": {},
		"summary": {},
		// task.go (taskMergeGit) runs git as a child, as deliver does through gitx;
		// it takes no signal, owner or stream of the host.
		"merge": {},
	}},
	"plan": {},
	"job": {subs: map[string]hsIpSub{
		// orchestrator.go:303 waits on os.Interrupt.
		"next":    {why: "waits on the process's interrupt signal (orchestrator.go:303)"},
		"post":    {},
		"ack":     {},
		"result":  {},
		"cancel":  {},
		"tree":    {},
		"blocked": {},
		"render":  {},
		"lint":    {},
		"show":    {},
		// jobcmd/verify.go:38 waits on SIGINT and SIGTERM; :41 runs the verify commands as children.
		"verify": {why: "runs the verify commands as host children (job/jobcmd/verify.go:38, :41)"},
		"cycles": {},
		"trace":  {},
	}},
	"lease": {subs: map[string]hsIpSub{
		// task_lease.go:85: --hold makes the parent process the lease holder. Kept
		// binary-only even if C-hs-plib-call makes it owner-aware, until a later brief
		// changes this answer.
		"take":    {flags: map[string]string{"hold": "--hold records the parent process id as the lease holder (task_lease.go:85)"}},
		"release": {},
		"status":  {},
	}},
	"feed": {subs: map[string]hsIpSub{
		// orchestrator.go:303 waits on os.Interrupt.
		"tick":     {why: "waits on the process's interrupt signal (orchestrator.go:303)"},
		"post":     {},
		"read":     {},
		"state":    {},
		"types":    {},
		"handover": {},
	}},
	// cli.go:140 and mcp.go:35: a stdio MCP server on the process's streams.
	"mcp":      {why: "serves MCP on stdio (cli.go:140, mcp.go:35)"},
	"question": {},
	"answer":   {},
	"approve":  {},
	"deliver":  {},
	"proof":    {},
	"history":  {},
	// land.go:32 stops on the Call's Context; land.go:34 hands the Call's Getenv to
	// the deploy child (land/land.go:245, land/environment.go:82-83).
	"land":  {},
	"brain": {},
	"env": {subs: map[string]hsIpSub{
		// environment.go:126 waits on SIGINT and SIGTERM; env/env.go:117 claims the
		// port with os.Getpid and env.go:127 starts the server as a child.
		"up":   {why: "waits on process signals and claims a port for the process id (environment.go:126, env/env.go:117)"},
		"down": {},
	}},
	"person": {},
	"routine": {subs: map[string]hsIpSub{
		"add":  {},
		"list": {},
		// routine.go:159-160 runs the routine on context.Background with the Call's
		// getenv, which builds the child's environment (routine/execute.go:147-150, :169-174).
		"run":    {},
		"remove": {},
		// routine.go:222-224 writes a system schedule that calls the host's executable.
		"schedule":   {why: "writes a system schedule for the host's executable (routine.go:222-224, routine/schedule.go:37)"},
		"unschedule": {},
	}},
	"worker": {subs: map[string]hsIpSub{
		"list": {},
		// worker.go:112 waits on os.Interrupt.
		"next": {why: "waits on the process's interrupt signal (worker.go:112)"},
	}},
	// run.go:51 waits on os.Interrupt, run.go:43 and :76 name the host's process id in the
	// task lease, and the runner starts agents as host children (runner/runner.go).
	"run": {why: "waits on the process's interrupt signal, holds task leases for the process id and starts agents as host children (run.go:43, run.go:51)"},
	// agent.go:34 stops on the Call's Context; agent check probes with the Call's
	// PATH and Getenv (agent.go:14, agent/runner.go:57-58); agent sync asks the
	// Call's Executable (agent_sync.go:17); budget reads the Call's getenv (budget.go:19).
	"agent":    {},
	"claude":   {},
	"workflow": {},
	"skill":    {},
	"session":  {},
	"intent":   {},
	"usage":    {},
	"account": {subs: map[string]hsIpSub{
		// account.go:30 gives account.Env no Environ, so account/account.go:216 runs
		// the provider's status command with os.Environ (account/account.go:68).
		"list":   {why: "probes the provider's status command with the host environment (account/account.go:216, :68)"},
		"status": {why: "probes the provider's status command with the host environment (account/account.go:216, :68)"},
		"scan":   {why: "probes the provider's status command with the host environment (account/account.go:216, :68)"},
		// account.go:54 then account/account.go:388: the provider's sign-in waits on a person.
		"sign-in":    {why: "runs the provider's sign-in as a host-environment child (account.go:54, account/account.go:388)"},
		"statusline": {},
		"windows":    {},
		"folder":     {},
		// account.go:200 then account/statusline.go:125: --then runs a shell command.
		"note": {flags: map[string]string{"then": "--then runs a shell command as a host child (account.go:200, account/statusline.go:125)"}},
	}},
	"project":    {},
	"project-md": {},
	"setup":      {},
	"signal":     {},
	"artifact":   {},
	// shot.go reads and writes the PNG files it is given, by path from the Call's Cwd.
	"shot": {},
	"eval": {subs: map[string]hsIpSub{
		"list": {},
		// evalcmd/command.go:169 waits on os.Interrupt; :175 runs the agent under evaluation as a child.
		"run":  {why: "runs the evaluated agent as a host child (evalcmd/command.go:169, :175)"},
		"show": {},
	}},
	"migrate": {},
	// update.go:25 acts on a project's update file in the Grid home; it mails nothing
	// and uses no process id, signal or child (internal/updates holds none).
	"update": {},
}

// InProcess reports whether lib.Run may run args inside a host process, with the
// reason. It fails closed: an empty argv, an unknown verb, or a subverb the table
// does not name answers false.
func InProcess(args []string) (ok bool, why string) {
	return hsIpAnswer(hsIpTable, args)
}

// hsIpAnswer answers args from table.
func hsIpAnswer(table map[string]hsIpVerb, args []string) (bool, string) {
	args, ok := hsIpSkipGlobal(args)
	if !ok {
		return false, "a global flag before the verb has no value"
	}
	if len(args) == 0 {
		return false, "no verb"
	}
	verb, rest := args[0], args[1:]
	if verb == "help" || verb == "-h" || verb == "--help" {
		return true, "prints the command list"
	}
	v, ok := table[verb]
	if !ok {
		return false, fmt.Sprintf("unknown verb %q", verb)
	}
	if v.why != "" {
		return false, verb + ": " + v.why
	}
	if v.subs == nil {
		return true, verb + " reads and writes the Grid home only"
	}
	if len(rest) == 0 {
		return false, verb + " needs a subverb"
	}
	sub := rest[0]
	if sub == "help" || sub == "-h" || sub == "--help" || sub == "-help" {
		return true, verb + " prints its usage"
	}
	s, ok := v.subs[sub]
	if !ok {
		return false, fmt.Sprintf("unknown subverb %q of %s", sub, verb)
	}
	if s.why != "" {
		return false, verb + " " + sub + ": " + s.why
	}
	for _, a := range rest[1:] {
		if why, ok := s.flags[hsIpFlagName(a)]; ok {
			return false, verb + " " + sub + ": " + why
		}
	}
	return true, verb + " " + sub + " reads and writes the Grid home only"
}

// hsIpSkipGlobal strips the global flags at the front of args the way
// applyGlobalFlags does (global.go:28-47); ok is false for one without a value.
func hsIpSkipGlobal(args []string) ([]string, bool) {
	for len(args) > 0 && strings.HasPrefix(args[0], "--") {
		name, value, hasValue := strings.Cut(args[0][2:], "=")
		if _, ok := globalFlags[name]; !ok {
			break
		}
		args = args[1:]
		if !hasValue {
			if len(args) == 0 {
				return nil, false
			}
			value, args = args[0], args[1:]
		}
		if value == "" {
			return nil, false
		}
	}
	return args, true
}

// hsIpFlagName is the name of a flag argument (-x, --x, --x=v), or "" for an operand.
func hsIpFlagName(a string) string {
	if !strings.HasPrefix(a, "-") {
		return ""
	}
	name := strings.TrimPrefix(strings.TrimPrefix(a, "-"), "-")
	name, _, _ = strings.Cut(name, "=")
	return name
}
