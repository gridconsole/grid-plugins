package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
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
// task folder dir: the catalog default of the role's agent, then the personal
// values, then those of the project the folder belongs to. A role with no catalog
// agent answers two empty strings. A project folder with no registered record has
// only the personal values, as settingsStore does for an unregistered folder.
func workerSpawnModel(env Env, dir, role string) (string, string, error) {
	if _, ok := settings.AgentForRole(role); !ok {
		return "", "", nil
	}
	// A read: settingsStore resolves the project folder and falls back to Personal for one with no record.
	st, err := settingsStore(env, filepath.Base(filepath.Dir(filepath.Dir(dir))), true)
	if errors.Is(err, where.ErrProjectUnregistered) {
		st, err = settingsStore(env, gridhome.Personal, true)
	}
	if err != nil {
		return "", "", err
	}
	return st.ModelFor(role)
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
