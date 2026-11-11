// Package cli is the grid command dispatcher. Output follows plugin-design.md
// section 9: one plain line per fact, and --json carries fields of the same names.
package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/question"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// guestSettingsRefusal is the one text every settings write gives a guest.
const guestSettingsRefusal = "guests cannot change project settings"

// Env is everything a command reads from the machine, so tests can set it.
// Paths are expected to be symlink-resolved (see gridhome.Real).
type Env struct {
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	Cwd        string
	Home       string
	GridHome   string // "" when neither GRID_HOME nor a home folder is known
	TempRoots  []string
	Machine    string                 // this machine's id, "" before it is made
	Ceiling    string                 // "" walks to the root; tests set their fixture root
	GitVersion func() (string, error) // nil runs git --version
	failure    *commandFailure
	Getenv     func(string) string    // nil reads the process environment
	Executable func() (string, error) // nil asks os.Executable
	OpenURL    func(string) error     // nil opens the system browser; tests set it
	AppStart   *AppStart              // how grid open starts the app; nil starts nothing, OSEnv sets it
	// afterApprove is a test seam: it runs once Approve has returned and before
	// the person Job result is posted, so a test can look at the locks there.
	afterApprove func()
	// typedGlobal names the variables a global flag was typed for (GRID_LEASE_EPOCH for --lease-epoch):
	// getenv shows the flag's value either way, and this tells a typed flag from an inherited variable.
	typedGlobal map[string]bool
}

func (e Env) afterApproveHook() {
	if e.afterApprove != nil {
		e.afterApprove()
	}
}

func (e Env) getenv(key string) string {
	if e.Getenv != nil {
		return e.Getenv(key)
	}
	return os.Getenv(key)
}

func (e Env) executable() (string, error) {
	if e.Executable != nil {
		return e.Executable()
	}
	return os.Executable()
}

// OSEnv reads the real process environment.
func OSEnv() Env {
	home, _ := os.UserHomeDir()
	if home != "" {
		home = gridhome.Real(home)
	}
	cwd, _ := os.Getwd()
	gh := gridhome.Dir(os.Getenv, home)
	if gh != "" {
		gh = gridhome.Real(gh)
	}
	machine := ""
	if gh != "" {
		machine = gridhome.Machine(gh)
	}
	return Env{
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Cwd: cwd, Home: home, GridHome: gh,
		TempRoots: []string{os.TempDir(), "/tmp", "/var/tmp"},
		Machine:   machine,
		AppStart:  &AppStart{},
	}
}

type command struct {
	name    string
	summary string
	run     func(args []string, env Env) int
}

func commands() []command {
	return []command{
		{"version", "print the CLI, contract and app_min versions", runVersion},
		{"where", "show which project and task a folder belongs to", runWhere},
		{"open", "open the task page in the browser", runOpen},
		{"doctor", "check machine health and collect a support bundle", runDoctor},
		{"hook", "answer a Claude Code or Codex hook event; set and list hook levels", runHook},
		{"task", "new, show, list, status, stop, close, reopen, history, attach, move, decline, delete, rename, restore", runTask},
		{"plan", "write, check, items, hash, diff, migrate", runPlan},
		{"job", "next, post, ack, result, cancel, tree, blocked, render, lint, show, verify, cycles, order, trace", runOrchestratorJob},
		{"lease", "take, release, status (task lease)", runTaskLease},
		{"feed", "tick, post, read, state, types, handover, fold", runOrchestratorFeed},
		{"mcp", "stdio server with the Feed tools", runMCP},
		{"question", "add, ask, withdraw, list, tick and sync task questions", runQuestion},
		{"answer", "record a person's answer", runAnswer},
		{"approve", "approve the plan or a later approve step (a person or the app only)", runApprove},
		{"deliver", "write the outcome and wait for the person", runDeliver},
		{"proof", "add evidence and rebuild the task proof index", runProof},
		{"history", "add a dated line to the task history", runHistory},
		{"land", "land a task per the deploy topic", runLand},
		{"brain", "show, search, topics, index, propose, accept, keep, decide, supersede, check, changes, revert", runBrain},
		{"env", "up, down browser Job dev environments", runEnv},
		{"person", "check person access; list, show and find approvers", runPerson},
		{"routine", "add, list, remove, run, schedule and unschedule routines", runRoutine},
		{"worker", "list local grid-worker processes; next is a worker's call for its next request", runWorker},
		{"agent", "check agent readiness; sync templates; set or unset models and efforts; budget; list settings", runAgentCommand},
		{"claude", "show and set the person's Claude Code settings rows", runClaude},
		{"workflow", "show workflow template files and parse problems; write a template file", runWorkflow},
		{"skill", "enable, disable and list skills", runSkill},
		{"session", "list, show, start, stop, send, pause, resume, handback, switch", runSessionCommand},
		{"intent", "list, claim, done, release, renew (worker requests)", runIntent},
		{"usage", "post and show usage, fold observed session totals", runUsage},
		{"account", "list, status, statusline, scan, sign-in, windows, folder and note for agent accounts", runAccount},
		{"project", "add, remove, list, scan and relink projects", runProject},
		{"project-md", "get or set a project front-matter key", runProjectMD},
		{"setup", "record project setup answers and write the Claude Code settings block (--sandbox turns on its sandbox)", runSetup},
		{"signal", "add, act on, dismiss and move inbox signals", runSignal},
		{"artifact", "import, link, list and publish task artifacts", runArtifact},
		{"eval", "list, run and show task evaluations", runEval},
		{"migrate", "upgrade the Grid home to this format", runMigrate},
	}
}

// Implemented reports whether the command table has a built command of that name.
func Implemented(name string) bool {
	for _, c := range commands() {
		if c.name == name {
			return c.run != nil
		}
	}
	return false
}

// anyFormat lists the commands that run whatever format the Grid home has: version
// reads nothing, doctor reports the format, and migrate is how an old home moves on.
var anyFormat = map[string]bool{"version": true, "doctor": true, "migrate": true}

// Run executes one grid call and returns the exit code: 0 done, 1 failed,
// 2 usage, 3 rejected (stale lease or invalid record; do not retry), 4 disk full.
func Run(args []string, env Env) (exit int) {
	args, env, code := applyGlobalFlags(args, env)
	if code != 0 {
		return code
	}
	if jsonRequested(args) {
		f := &commandFailure{stdout: outputCount{Writer: env.Stdout}}
		env.failure = f
		env.Stdout = &f.stdout
		env.Stderr = io.MultiWriter(env.Stderr, &f.stderr)
		defer func() { f.finish(args, exit) }()
	}
	if len(args) == 0 {
		printHelp(env.Stdout)
		return 0
	}
	name, rest := args[0], args[1:]
	if name == "help" || name == "-h" || name == "--help" {
		printHelp(env.Stdout)
		return 0
	}
	for _, c := range commands() {
		if c.name != name {
			continue
		}
		if !anyFormat[name] {
			if msg := formatRefusal(env); msg != "" {
				if name == "hook" {
					return 0 // a hook never speaks into a session
				}
				fmt.Fprintln(env.Stderr, msg)
				return 1
			}
		}
		if guestWriteVerb(name, rest) && strings.ToLower(strings.TrimSpace(env.getenv("GRID_ROLE"))) == "guest" && env.getenv("GRID_LEASE") != "task" {
			// A guest's role is refused before any project is resolved, so the
			// answer is the same for an unknown project and an invited one.
			if name == "question" {
				id := ""
				if len(rest) > 1 {
					id = rest[1]
				}
				fmt.Fprintf(env.Stderr, "question: %s\n", question.WithdrawRefusalText(id, "guest"))
				return 3
			}
			if name == "task" {
				if guestTaskMove(rest[0]) {
					// Exit 1, as a hidden task answers a guest's stop
					// (TestPersonalBypassTaskCommands); delete and restore exit 3.
					fmt.Fprintf(env.Stderr, "grid task %s: %s\n", rest[0], guestTaskMoveRefusal(rest[0]))
					return 1
				}
				fmt.Fprintf(env.Stderr, "grid task %s: %v\n", rest[0], fmt.Errorf("%w: %s", task.ErrEditRejected, task.MoveRefusal))
				return 3
			}
			if name == "session" {
				fmt.Fprintf(env.Stderr, "grid session %s: %s\n", rest[0], guestSessionRefusal(rest[0]))
				return 1
			}
			fmt.Fprintln(env.Stderr, "grid: "+guestSettingsRefusal)
			return 1
		}
		if code := workflowRoleRefusal(args, env); code != 0 {
			return code
		}
		return c.run(rest, env)
	}
	fmt.Fprintf(env.Stderr, "grid: unknown command %q. Run: grid help\n", name)
	return 2
}

// guestWriteVerb names the writes a guest role may not make: the answer must not
// depend on whether the project exists. Standalone help is not a write.
func guestWriteVerb(name string, rest []string) bool {
	if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		return false
	}
	switch name {
	case "setup":
		return true
	case "task":
		return len(rest) > 0 && (rest[0] == "delete" || rest[0] == "restore" || guestTaskMove(rest[0]))
	case "question":
		return len(rest) > 0 && rest[0] == "withdraw"
	case "session":
		return len(rest) > 0 && guestSessionWrite(rest[0])
	}
	return false
}

// guestSessionWrite is a session write a person makes: the owner's rule is that a
// guest answers and comments, and does not start, stop, steer or switch a session.
// list and show read, so a guest keeps them.
func guestSessionWrite(verb string) bool {
	switch verb {
	case "start", "stop", "send", "pause", "resume", "handback", "switch":
		return true
	}
	return false
}

// guestSessionRefusal is the reason a guest's session write gives; it names the
// role the way a guest's task move does.
func guestSessionRefusal(verb string) string {
	return fmt.Sprintf("role %q may not %s a session; a guest answers and comments", "guest", verb)
}

// guestTaskMove is a task move a person makes: the owner's rule is that a guest
// answers and comments, and does not create, close, reopen, stop or move a task.
func guestTaskMove(verb string) bool {
	return verb == "new" || verb == "close" || verb == "reopen" || verb == "stop" || verb == "move"
}

// guestTaskMoveRefusal is the reason a guest's new, close, reopen, stop or move
// gives; it names the role the way a guest's question withdrawal does.
func guestTaskMoveRefusal(verb string) string {
	return fmt.Sprintf("role %q may not %s a task; a guest answers and comments", "guest", verb)
}

// formatRefusal says why this grid must not read the Grid home, or "" when it may.
// A home without VERSION holds nothing yet; its first write makes the file.
func formatRefusal(env Env) string {
	if env.GridHome == "" {
		return ""
	}
	n, err := gridhome.ReadFormat(env.GridHome)
	if errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err != nil {
		path := gridhome.Tilde(filepath.Join(env.GridHome, "VERSION"), env.Home)
		return fmt.Sprintf("Cannot read the format version in %s. Run: grid doctor", path)
	}
	return gridhome.FormatProblem(n)
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, "grid: the Grid command line\n\nUsage: grid <command> [flags]\n\nCommands:\n")
	width := 0
	for _, c := range commands() {
		width = max(width, len(c.name))
	}
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-*s  %s\n", width, c.name, c.summary)
	}
	fmt.Fprint(w, "\nGlobal flags go before the command; each wins over its environment variable:\n"+
		"  --grid-home DIR    the Grid home (GRID_HOME)\n"+
		"  --member ID        who is calling (GRID_MEMBER)\n"+
		"  --role ROLE        the caller's role (GRID_ROLE)\n"+
		"  --lease-epoch G.F  the lease generation.fence (GRID_LEASE_EPOCH)\n")
	fmt.Fprintln(w, "\n"+exitStatusHelp)
}

func newFlags(name string, env Env) *flag.FlagSet {
	fs := flag.NewFlagSet("grid "+name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	if env.failure != nil {
		env.failure.flags = fs
	}
	return fs
}

// parse returns stop true with the exit code when the command must end.
func parse(fs *flag.FlagSet, args []string, env Env) (code int, stop bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, true
		}
		return 2, true
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(env.Stderr, "%s: unexpected argument %q\n", fs.Name(), fs.Arg(0))
		return 2, true
	}
	return 0, false
}

func writeJSON(w io.Writer, v any) int {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return 1
	}
	return 0
}

func resolver(env Env) where.Resolver {
	return where.Resolver{GridHome: env.GridHome, Home: env.Home, TempRoots: env.TempRoots,
		Machine: env.Machine, Ceiling: env.Ceiling, Caller: where.CallerFrom(env.getenv)}
}

func runWhere(args []string, env Env) int {
	fs := newFlags("where", env)
	asJSON := fs.Bool("json", false, "print JSON")
	task := fs.String("task", "", "a task folder path or id (T-07)")
	proj := fs.String("project", "", "a project name or id")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if env.GridHome == "" {
		fmt.Fprintln(env.Stderr, "grid: cannot find the Grid home; set GRID_HOME")
		return 1
	}
	res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: *task, Project: *proj})
	if err != nil {
		env.recordError(err)
		fmt.Fprintf(env.Stderr, "grid: %v\n", err)
		return 1
	}
	if *asJSON {
		return writeJSON(env.Stdout, res)
	}
	// Plain output names each fact as --json does; a list gives one line per entry.
	w := env.Stdout
	tilde := func(p string) string { return gridhome.Tilde(p, env.Home) }
	fmt.Fprintf(w, "project: %s\n", res.Project)
	if res.ID != "" {
		fmt.Fprintf(w, "id: %s\n", res.ID)
	}
	fmt.Fprintf(w, "rule: %s\n", res.Rule)
	if res.Folder != "" {
		fmt.Fprintf(w, "folder: %s\n", tilde(res.Folder))
	}
	fmt.Fprintf(w, "record: %t\n", res.Record)
	if res.Task != "" {
		fmt.Fprintf(w, "task: %s\ntask_path: %s\n", res.Task, tilde(res.TaskPath))
	}
	for _, r := range res.Repos {
		fmt.Fprintf(w, "repos: %s\n", tilde(r))
	}
	for _, k := range res.Keys {
		fmt.Fprintf(w, "keys: %s\n", k)
	}
	return 0
}
