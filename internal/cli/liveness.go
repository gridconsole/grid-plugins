package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// These adapters only resolve scope and render results. Persistence and liveness
// decisions belong to the session and lease packages.
func runLease(args []string, env Env) int {
	const usage = "Usage: grid lease take|release|status task --task <task> [--project <project>] [--json]"
	help := func(a string) bool { return a == "-h" || a == "--help" || a == "-help" }
	if len(args) > 0 && help(args[0]) || len(args) == 2 && help(args[1]) {
		fmt.Fprintln(env.Stdout, usage)
		return 0
	}
	if len(args) < 2 || args[1] != "task" {
		fmt.Fprintln(env.Stderr, usage)
		return 2
	}
	switch action := args[0]; action {
	case "take", "release", "status":
		return runTaskLeaseAction(action, args[2:], env)
	default:
		// A task lease has no renewal, so renew is not an action.
		fmt.Fprintf(env.Stderr, "lease: unknown action %q; use take, release or status\n", action)
		return 2
	}
}

type sessionView struct {
	session.Record
	Task string `json:"task"`
}

func runSession(args []string, env Env) int {
	if len(args) == 0 || (args[0] != "list" && args[0] != "show") {
		fmt.Fprintln(env.Stderr, "Usage: grid session list|show [session-id] [--task <task>] [--project <project>] [--machine <machine>] [--json]")
		return 2
	}
	action, args := args[0], args[1:]
	id := ""
	if action == "show" && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		id, args = args[0], args[1:]
	}
	fs := newFlags("session "+action, env)
	project := fs.String("project", "", "project name or id (otherwise resolve the current folder)")
	task := fs.String("task", "", "task folder or id; defaults to the current task, else all project tasks")
	machine := fs.String("machine", "", "machine id to disambiguate a resumed session")
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if action == "show" && id == "" {
		fmt.Fprintln(env.Stderr, "session show: session id is required before flags")
		return 2
	}
	scope, err := livenessScope(env, *project, *task)
	if err != nil {
		return livenessError(env, err)
	}
	records, err := scopedSessions(env, scope)
	if err != nil {
		return livenessError(env, err)
	}
	filtered := []sessionView{}
	for _, r := range records {
		if (*machine == "" || r.MachineID == *machine) && (id == "" || r.SessionID == id) {
			filtered = append(filtered, r)
		}
	}
	if action == "list" {
		if *asJSON {
			return writeJSON(env.Stdout, filtered)
		}
		if len(filtered) == 0 {
			fmt.Fprintln(env.Stdout, "No sessions.")
		}
		for _, r := range filtered {
			fmt.Fprintf(env.Stdout, "session_id: %s  machine_id: %s  task: %s  state: %s\n", r.SessionID, r.MachineID, r.Task, session.Fold(r.Record))
		}
		return 0
	}
	if len(filtered) == 0 {
		return livenessError(env, fmt.Errorf("session %q not found", id))
	}
	if len(filtered) > 1 {
		return livenessError(env, fmt.Errorf("session %q is ambiguous; specify --machine and --task", id))
	}
	r := filtered[0]
	if *asJSON {
		return writeJSON(env.Stdout, r)
	}
	fmt.Fprintf(env.Stdout, "session_id: %s\nmachine_id: %s\ntask: %s\nstate: %s\nruntime: %s\nwhere: %s\nstarted: %s\nlast: %s\n", r.SessionID, r.MachineID, r.Task, session.Fold(r.Record), r.Runtime, r.Where, r.Started.Format(time.RFC3339), r.Last.Format(time.RFC3339))
	if r.Cwd != "" {
		fmt.Fprintf(env.Stdout, "cwd: %s\n", r.Cwd)
	}
	if r.Transcript != "" {
		fmt.Fprintf(env.Stdout, "transcript: %s\n", r.Transcript)
	}
	if r.Slept != nil {
		fmt.Fprintf(env.Stdout, "slept: %s\n", r.Slept.Format(time.RFC3339))
	}
	if r.Ended != nil {
		fmt.Fprintf(env.Stdout, "ended: %s\n", r.Ended.Format(time.RFC3339))
	}
	return 0
}

func livenessScope(env Env, project, task string) (where.Result, error) {
	if env.GridHome == "" {
		return where.Result{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	return resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: project, Task: task})
}
func livenessError(env Env, err error) int {
	env.recordError(err)
	fmt.Fprintf(env.Stderr, "grid: %v\n", err)
	return 1
}
func scopedSessions(env Env, scope where.Result) ([]sessionView, error) {
	records := []sessionView{}
	appendTask := func(path, task string) error {
		rows, err := (session.Store{TaskDir: path}).List()
		if err != nil {
			return err
		}
		for _, r := range rows {
			records = append(records, sessionView{r, task})
		}
		return nil
	}
	if scope.TaskPath != "" {
		return recordsForTask(scope.TaskPath, scope.Task)
	}
	dir := gridhome.TasksDir(env.GridHome, scope.Project)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "T-") {
			if err := appendTask(filepath.Join(dir, entry.Name()), entry.Name()); err != nil {
				return nil, err
			}
		}
	}
	return records, nil
}
func recordsForTask(path, task string) ([]sessionView, error) {
	rows, err := (session.Store{TaskDir: path}).List()
	if err != nil {
		return nil, err
	}
	records := make([]sessionView, 0, len(rows))
	for _, r := range rows {
		records = append(records, sessionView{r, task})
	}
	return records, nil
}
