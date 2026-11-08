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

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

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
	Getenv     func(string) string    // nil reads the process environment
	Executable func() (string, error) // nil asks os.Executable
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
	}
}

type command struct {
	name    string
	summary string
	run     func(args []string, env Env) int // nil: not built yet
}

func commands() []command {
	return []command{
		{"version", "print the CLI, contract and app_min versions", runVersion},
		{"where", "show which project and task a folder belongs to", runWhere},
		{"doctor", "check machine health and collect a support bundle", runDoctor},
		{"hook", "answer a Claude Code or Codex hook event; set and list hook levels", runHook},
		{"task", "new, show, list, status, stop, close, reopen, history, attach, move", runTask},
		{"plan", "write, check, items, hash, diff", runPlan},
		{"views", "render the task pages", nil},
		{"job", "next, post, ack, result, cancel, tree, blocked, render, lint, show, verify, cycles, floor", runOrchestratorJob},
		{"lease", "take task (adoption, takeover); take, renew, release, status merge", runTaskLease},
		{"feed", "tick, post, read, state, types, handover, fold", runOrchestratorFeed},
		{"mcp", "stdio server with the Feed tools", runMCP},
		{"question", "add, withdraw, list, tick and sync task questions", runQuestion},
		{"ask", "ask a person a question", nil},
		{"answer", "record a person's answer", runAnswer},
		{"approve", "approve a gate (a person or the app only)", runApprove},
		{"refusal", "decide a refused command (a person or the app only)", runRefusal},
		{"deliver", "write the outcome and raise the ship gate", runDeliver},
		{"proof", "add evidence and rebuild the task proof index", runProof},
		{"history", "append a dated task history line", runHistory},
		{"land", "land a task per the deploy topic", runLand},
		{"brain", "show, search, topics, index, propose, accept, keep, decide", runBrain},
		{"machine", "machines topic", nil},
		{"env", "up, down browser Job dev environments", runEnv},
		{"ports", "claim, release, list, gc dev port blocks", runPorts},
		{"deploy", "deploy topic", nil},
		{"person", "people topic", runPerson},
		{"routine", "add, list, remove, run, schedule and unschedule routines", runRoutine},
		{"worker", "list local grid-worker processes", runWorker},
		{"agent", "check agent readiness; set models and efforts; list settings", runAgentCommand},
		{"workflow", "show workflow template files and parse problems", runWorkflow},
		{"skill", "enable, disable and list skills", runSkill},
		{"session", "list, show, start, stop, send, pause, resume, handback, switch", runSessionCommand},
		{"intent", "list, claim, done (worker requests)", runIntent},
		{"usage", "post and show usage, set budgets and notice thresholds, fold observed session totals", runUsage},
		{"account", "agent accounts", nil},
		{"project", "add, remove, list, scan and relink projects", runProject},
		{"project-md", "get or set a project front-matter key", runProjectMD},
		{"setup", "record validated project setup answers", runSetup},
		{"signal", "add, act on and dismiss inbox signals", runSignal},
		{"artifact", "import, link, list and publish task artifacts", runArtifact},
		{"eval", "list, run and show task evaluations", runEval},
		{"migrate", "upgrade the Grid home to this format", runMigrate},
		{"sync", "sync with a Grid server", nil},
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
func Run(args []string, env Env) int {
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
		if c.run == nil {
			fmt.Fprintf(env.Stderr, "%s: not built yet\n", name)
			return 2
		}
		return c.run(rest, env)
	}
	fmt.Fprintf(env.Stderr, "grid: unknown command %q. Run: grid help\n", name)
	return 2
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
		note := ""
		if c.run == nil {
			note = " (not built yet)"
		}
		fmt.Fprintf(w, "  %-*s  %s%s\n", width, c.name, c.summary, note)
	}
	fmt.Fprintln(w, "\n"+exitStatusHelp)
}

func newFlags(name string, env Env) *flag.FlagSet {
	fs := flag.NewFlagSet("grid "+name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
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
