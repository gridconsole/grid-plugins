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
// own streams, opens the host's browser, or starts a child with the host's
// environment. Every other verb reads and writes the Grid home and answers yes.
var hsIpTable = map[string]hsIpVerb{
	"version": {},
	"where":   {},
	// cli.go:105 and open.go:67: opens the system browser and may start the app.
	"open":   {why: "opens the browser and may start the app (open.go:67, open_start.go:216)"},
	"doctor": {},
	// cli.go:107: a hook answers a Claude Code or Codex event on the host's stdin.
	"hook": {why: "answers a host hook event on stdin (cli.go:107)"},
	"task": {subs: map[string]hsIpSub{
		// task_new_first_job.go:84 records os.Getpid as the first Job's owner. Kept
		// binary-only even if C-hs-plib-call makes it owner-aware, until a later brief
		// changes this answer.
		"new":     {why: "records the process id as the first Job's owner (task_new_first_job.go:84)"},
		"show":    {},
		"list":    {},
		"status":  {},
		"stop":    {},
		"close":   {},
		"reopen":  {},
		"history": {},
		// task.go:235 records os.Getppid as the attached session.
		"attach":  {why: "records the parent process id as the attached session (task.go:235)"},
		"move":    {},
		"decline": {},
		"delete":  {},
		"rename":  {},
		"restore": {},
	}},
	"plan": {},
	"job": {subs: map[string]hsIpSub{
		// orchestrator.go:300 waits on os.Interrupt.
		"next":    {why: "waits on the process's interrupt signal (orchestrator.go:300)"},
		"post":    {},
		"ack":     {},
		"result":  {},
		"cancel":  {},
		"tree":    {},
		"blocked": {},
		"render":  {},
		"lint":    {},
		"show":    {},
		// jobcmd/verify.go:31 runs the verify commands as children.
		"verify": {why: "runs the verify commands as host children (job/jobcmd/verify.go:31)"},
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
		// orchestrator.go:300 waits on os.Interrupt.
		"tick":     {why: "waits on the process's interrupt signal (orchestrator.go:300)"},
		"post":     {},
		"read":     {},
		"state":    {},
		"types":    {},
		"handover": {},
	}},
	// cli.go:113: a stdio MCP server on the host's own streams.
	"mcp":      {why: "serves MCP on stdio (cli.go:113)"},
	"question": {},
	"answer":   {},
	"approve":  {},
	"deliver":  {},
	"proof":    {},
	"history":  {},
	// land.go:34 waits on os.Interrupt; its children take the host environment (lib.go:34).
	"land":  {why: "waits on the process's interrupt signal and starts host-environment children (land.go:34)"},
	"brain": {},
	"env": {subs: map[string]hsIpSub{
		// environment.go:128 waits on SIGINT and SIGTERM; env/env.go:113 claims the
		// port with os.Getpid and env.go:121 starts the server as a child.
		"up":   {why: "waits on process signals and claims a port for the process id (environment.go:128, env/env.go:113)"},
		"down": {},
	}},
	"person": {},
	"routine": {subs: map[string]hsIpSub{
		"add":  {},
		"list": {},
		// routine.go:43 runs the routine's command as a host-environment child (lib.go:34).
		"run":    {why: "runs the routine as a host-environment child (routine.go:43, routine/execute.go)"},
		"remove": {},
		// routine.go:197 writes a system schedule that calls the host's executable.
		"schedule":   {why: "writes a system schedule for the host's executable (routine.go:197, routine/schedule.go)"},
		"unschedule": {},
	}},
	"worker": {subs: map[string]hsIpSub{
		"list": {},
		// worker.go:189 waits on os.Interrupt.
		"next": {why: "waits on the process's interrupt signal (worker.go:189)"},
	}},
	// agent.go:36 waits on signals while agent check probes host children
	// (agent/runner.go:32); agent sync finds the plugin from the host's executable
	// (agent_sync.go:17); the agent's runs take the host environment (lib.go:34).
	"agent":    {why: "waits on process signals, probes host children and reads the host's executable (agent.go:36, agent_sync.go:17)"},
	"claude":   {},
	"workflow": {},
	"skill":    {},
	"session":  {},
	"intent":   {},
	"usage":    {},
	"account": {subs: map[string]hsIpSub{
		// account/account.go:183 runs the provider's status command with os.Environ
		// (read from account.go:214 and :270 for every folder listed).
		"list":   {why: "probes the provider's status command with the host environment (account/account.go:183)"},
		"status": {why: "probes the provider's status command with the host environment (account/account.go:183)"},
		"scan":   {why: "probes the provider's status command with the host environment (account/account.go:183)"},
		// account.go:54 then account/account.go:385: the provider's sign-in waits on a person.
		"sign-in":    {why: "runs the provider's sign-in as a host-environment child (account.go:54, account/account.go:385)"},
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
	"eval": {subs: map[string]hsIpSub{
		"list": {},
		// evalcmd/command.go:155 runs the agent under evaluation as a child.
		"run":  {why: "runs the evaluated agent as a host child (evalcmd/command.go:155)"},
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
