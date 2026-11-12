package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	gridplugins "github.com/gridconsole/grid-plugins"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/migrate"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/settings"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/worker"
)

const workerUsage = "Usage: grid worker list [--json]\n       grid worker next --json --pid <worker pid> --pid-start <RFC3339>\n"

func runWorker(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(env.Stdout, workerUsage)
		return 0
	}
	if args[0] == "next" {
		return runWorkerNext(args[1:], env)
	}
	if args[0] != "list" {
		fmt.Fprintf(env.Stderr, "grid worker: unknown command %q\n", args[0])
		return 2
	}
	fs := newFlags("worker list", env)
	asJSON := fs.Bool("json", false, "print JSON")
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(env.Stderr, "grid worker list: unexpected argument")
		return 2
	}
	entries, err := (worker.Store{GridHome: env.GridHome}).List()
	if err != nil {
		return failure(env, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, entries)
	}
	for _, entry := range entries {
		started := ""
		if !entry.StartedAt.IsZero() {
			started = contract.FormatTime(entry.StartedAt)
		}
		fmt.Fprintf(env.Stdout, "machine: %s\npid: %d\nversion: %s\nstarted_at: %s\naddress: %s\nstate: %s\npresence: %s\n",
			entry.Machine, entry.PID, entry.Version, started, entry.Address, entry.State, entry.Presence)
	}
	return 0
}

var taskFolderPattern = regexp.MustCompile(`^T-[0-9]+(?:-.+)?$`)

// runWorkerNext is the stream a grid-worker supervises: it keeps the task
// lease of every task with an open Job in the worker's name, assigns Ready Jobs
// to the worker, and prints one spawn, wake or skip line per event. It runs
// until SIGINT (exit 130) and never exits 0 by itself. Exit 2 is a bad flag or
// setting, 3 a holder that is not this machine's (final: do not restart).
func runWorkerNext(args []string, env Env) int {
	fs := newFlags("worker next", env)
	fs.Bool("json", true, "print JSON lines (the only form)")
	pid := fs.Int("pid", 0, "the worker's process id; the task lease names this process")
	start := fs.String("pid-start", "", "the worker's process start in RFC3339")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	usage := func(message string) int { fmt.Fprintln(env.Stderr, "grid worker next: "+message); return 2 }
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		return livenessError(env, errors.New("task lease needs a Unix host for process identity"))
	}
	if env.GridHome == "" {
		return usage("cannot find the Grid home; set GRID_HOME or use --grid-home")
	}
	member := env.getenv("GRID_MEMBER")
	if member == "" {
		return usage("the holder is required: --member or GRID_MEMBER")
	}
	machine := env.Machine
	if machine == "" {
		machine = gridhome.Machine(env.GridHome)
	}
	if machine == "" {
		return usage("machine id is missing; initialize the Grid home")
	}
	// A holder may be written member@machine; the machine must be this one.
	if at := strings.IndexByte(member, '@'); at >= 0 {
		if member[at+1:] != machine {
			fmt.Fprintf(env.Stderr, "grid worker next: holder %q is not on this machine (%s)\n", member, machine)
			return 3
		}
		member = member[:at]
	}
	role := contract.NormalizeRole(env.getenv("GRID_ROLE"))
	if role == "" {
		role = "member"
	}
	// A worker performs mechanical CLI controls on the task lease and takes no person authority.
	if role == "worker" {
		role = "cli"
	}
	typ, _ := contract.LookupType("handover")
	if !slices.Contains(typ.Roles, role) {
		return usage("task acquisition requires a task-holder role")
	}
	if !contract.ValidWorker(member + "@" + machine) {
		return usage("holder must be <member>@<machine ULID> with no slash or whitespace")
	}
	if *pid < 1 || *start == "" {
		return usage("--pid and --pid-start are required")
	}
	parsed, err := time.Parse(time.RFC3339Nano, *start)
	if err != nil {
		return usage("--pid-start must be RFC3339")
	}
	*start = contract.FormatTime(parsed)
	if state := session.ProbeLocal(session.Live{PID: *pid, PIDStart: *start}); state != session.Alive {
		return usage(fmt.Sprintf("pid %d started %s is not a live process on this host (%s)", *pid, *start, processStates[state]))
	}

	// A stream runs for days: do not let the failure recorder keep every diagnostic line.
	diag := io.Writer(env.Stderr)
	if env.failure != nil {
		f := env.failure
		diag = writerFunc(func(p []byte) (int, error) {
			n, err := env.Stderr.Write(p)
			f.stderr.Reset()
			return n, err
		})
	}
	s := &orchestrator.Stream{
		Member: member, Machine: machine, Role: role,
		PID: *pid, PIDStart: *start,
		Out: env.Stdout, Diag: diag,
		Tasks: func() ([]string, error) { return workerTaskDirs(env) },
		Model: func(dir, role string) (string, string, error) { return workerSpawnModel(env, dir, role) },
		Tools: func(dir, role string) ([]string, error) { return workerSpawnTools(env, dir, role) },
		// A task whose first Job was never posted gets the same J-1 grid task new posts.
		FirstJob: firstJobOf,
		// The pass moves an old approval receipt into the task's Feed once, under the task lease.
		ReceiptPending: migrate.ReceiptPending,
		ReceiptMove: func(dir string, w feed.Writer) error {
			_, _, err := migrate.ReceiptToJob(dir, w)
			return err
		},
	}
	s.Configure = func(st *orchestrator.Store, taskID string) {
		dir := st.Feed.TaskDir
		configureTickStore(env, st, filepath.Base(filepath.Dir(filepath.Dir(dir))), taskID)
		st.Diagnostics = diag
		_, st.Owner, _ = taskstore.FeedIdentity(dir)
		if owner := env.getenv("GRID_OWNER"); owner != "" {
			st.Owner = owner
		}
	}
	// The first Job a task is assigned moves it from queued to running, written as the
	// stream's own member and role. Any other state is left as it is.
	s.Assigned = func(dir, job string) error {
		resolved, err := resolveTaskOperand(env, dir)
		if err != nil {
			return err
		}
		_, _, err = taskWriter(env).RunningFromQueued(taskstore.WriteOptions{
			Cwd: env.Cwd, ID: resolved.ID, Note: job + " assigned",
			Who: contract.Author{ID: member, Role: role},
		})
		return err
	}
	// The stream runs until SIGINT. It does not release the task leases then: the
	// tenure names the worker's process, and the worker's next stream reuses it.
	ctx, cancel := env.signalContext(os.Interrupt)
	defer cancel()
	err = s.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return 130
	}
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid worker next: %v\n", err)
		return 1
	}
	return 1 // Run returns only an error
}

// workerSpawnModel is the model and effort a Job of the given role runs on, for the
// task folder dir: the catalog default of the role's agent, then the `model:` and
// `effort:` of the brain agent the project or the home holds for the role (passed
// through as written), then the personal values, then those of the project the
// folder belongs to. A role with no catalog agent answers the brain agent's values,
// or two empty strings when the brain holds none. A project folder with no
// registered record has only the personal values, as settingsStore does for an
// unregistered folder.
func workerSpawnModel(env Env, dir, role string) (string, string, error) {
	project := filepath.Base(filepath.Dir(filepath.Dir(dir)))
	var fm struct {
		Model  string `yaml:"model"`
		Effort string `yaml:"effort"`
	}
	if md, _, ok := (brain.Store{GridHome: env.GridHome, Project: project}).AgentFor(role); ok {
		// AgentFor has parsed this front matter already; a value that is not a plain string is no value.
		front, _, err := mdfile.Split([]byte(md))
		if err != nil || mdfile.Load(front, &fm, false) != nil {
			fm.Model, fm.Effort = "", ""
		}
	}
	if _, ok := settings.AgentForRole(role); !ok {
		return fm.Model, fm.Effort, nil
	}
	// A read: settingsStore resolves the project folder and falls back to Personal for one with no record.
	st, err := settingsStore(env, project, true)
	if errors.Is(err, where.ErrProjectUnregistered) {
		st, err = settingsStore(env, gridhome.Personal, true)
	}
	if err != nil {
		return "", "", err
	}
	return st.ModelForWith(role, settings.Agent{Model: fm.Model, Effort: fm.Effort})
}

// workerSpawnTools is the tool names the agent that runs a Job of the given role names
// in its `tools:` front matter, for the task folder dir, in the order the file gives
// them. The agent is the one the Job's Instructions come from: the brain agent the
// project or the home holds for the role, else the plugin's own agents/<role>.md. The
// two are never merged: a brain agent with no `tools:` answers nil, as does a role with
// no agent file. Nothing is added, removed or checked here; the worker decides what to
// do with the names.
func workerSpawnTools(env Env, dir, role string) ([]string, error) {
	project := filepath.Base(filepath.Dir(filepath.Dir(dir))) // the project workerSpawnModel reads
	md, _, ok := (brain.Store{GridHome: env.GridHome, Project: project}).AgentFor(role)
	if !ok {
		md, ok = gridplugins.AgentFor(contract.NormalizeRole(role))
	}
	if !ok {
		return nil, nil
	}
	return beAgentTools(md), nil
}

// beToolsFragment is the lines of the one top-level `tools:` key of a front matter: the
// key line and the indented or `- ` lines under it. Only these lines are parsed, so a
// description elsewhere in the file that is not strict YAML (agents/builder.md has a
// ": " in its description) does not hide the tools. No `tools:` key, or two, is no
// fragment.
func beToolsFragment(front []byte) ([]byte, bool) {
	var frag []string
	keys := 0
	in := false
	for _, l := range strings.Split(string(front), "\n") {
		trimmed := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(l, "tools:"):
			keys++
			in = true
			frag = append(frag, l)
		case in && (trimmed == "" || l[0] == ' ' || l[0] == '\t' || l[0] == '-' || l[0] == '#'):
			frag = append(frag, l)
		default:
			in = false
		}
	}
	return []byte(strings.Join(frag, "\n")), keys == 1
}

// beAgentTools is the `tools:` of an agent file's front matter, read as a
// comma-separated string or as a list. Each entry is trimmed; empty entries and
// repeats are dropped and the order is kept. A file with no `tools:`, a `tools:`
// that does not parse, and a value of any other shape (a map, a number, a list that
// holds a non-string) give nil, as workerSpawnModel treats a model that is not a
// plain string.
func beAgentTools(md string) []string {
	front, _, err := mdfile.Split([]byte(md))
	if err != nil {
		return nil
	}
	frag, ok := beToolsFragment(front)
	if !ok {
		return nil
	}
	var fm struct {
		Tools any `yaml:"tools"`
	}
	if mdfile.Load(frag, &fm, false) != nil {
		return nil
	}
	var raw []string
	switch v := fm.Tools.(type) {
	case string:
		raw = strings.Split(v, ",")
	case []any:
		for _, e := range v {
			s, ok := e.(string)
			if !ok {
				return nil
			}
			raw = append(raw, s)
		}
	default:
		return nil
	}
	var tools []string
	for _, r := range raw {
		if r = strings.TrimSpace(r); r != "" && !slices.Contains(tools, r) {
			tools = append(tools, r)
		}
	}
	return tools
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// workerTaskDirs lists every task folder the caller may see, in every project and Personal.
func workerTaskDirs(env Env) ([]string, error) {
	roots, err := resolver(env).TasksDirs()
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() && taskFolderPattern.MatchString(e.Name()) {
				dirs = append(dirs, filepath.Join(root, e.Name()))
			}
		}
	}
	return dirs, nil
}
