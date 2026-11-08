package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// These adapters only resolve scope and render results. Persistence and liveness
// decisions belong to the session and lease packages.
func runLease(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "Usage: grid lease take|renew|release|status [merge] [--project <project>] [--holder <session>] [--fence <n>] [--json]")
		return 2
	}
	action, args := args[0], args[1:]
	switch action {
	case "take", "renew", "release", "status":
	default:
		fmt.Fprintf(env.Stderr, "lease: unknown action %q; use take, renew, release or status\n", action)
		return 2
	}
	if len(args) > 0 && args[0] == "merge" {
		args = args[1:]
	}
	fs := newFlags("lease "+action+" merge", env)
	project := fs.String("project", "", "project name or id (otherwise resolve the current folder)")
	task := fs.String("task", "", "resolve the project from a task folder or id")
	holder := fs.String("holder", "", "stable identity of the session landing the branch")
	fence := fs.Uint64("fence", 0, "fence returned by take; required for renew and release")
	asJSON := fs.Bool("json", false, "print JSON")
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	if action != "status" && strings.TrimSpace(*holder) == "" {
		fmt.Fprintln(env.Stderr, "lease: --holder is required")
		return 2
	}
	if (action == "renew" || action == "release") && *fence == 0 {
		fmt.Fprintln(env.Stderr, "lease: --fence is required")
		return 2
	}
	scope, err := livenessScope(env, *project, *task)
	if err != nil {
		return livenessError(env, err)
	}
	store := lease.Store{ProjectDir: filepath.Join(env.GridHome, "projects", scope.Project)}
	if action == "status" {
		status, err := store.Status()
		if err != nil {
			return livenessError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, status)
		}
		if status.Held {
			fmt.Fprintf(env.Stdout, "merge lease held by %s since %s\nfence: %d\nuntil: %s\n", *status.Holder, status.Since.Format(time.RFC3339), status.Fence, status.Until.Format(time.RFC3339))
		} else if status.Holder != nil {
			fmt.Fprintf(env.Stdout, "merge lease expired\nfence: %d\nuntil: %s\n", status.Fence, status.Until.Format(time.RFC3339))
		} else {
			fmt.Fprintln(env.Stdout, "merge lease free")
		}
		return 0
	}
	var rec lease.Record
	switch action {
	case "take":
		rec, err = store.Take(*holder)
	case "renew":
		rec, err = store.Renew(*holder, *fence)
	case "release":
		rec, err = store.Release(*holder, *fence)
	}
	if err != nil {
		// Preserve the exact grant refusal wording from feed-contract.md section 13.
		fmt.Fprintln(env.Stderr, err)
		return 1
	}
	if *asJSON {
		return writeJSON(env.Stdout, lease.Status{Record: rec, Held: rec.Holder != nil})
	}
	verb := map[string]string{"take": "taken", "renew": "renewed", "release": "released"}[action]
	fmt.Fprintf(env.Stdout, "merge lease %s, fence %d\n", verb, rec.Fence)
	return 0
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
	dir := filepath.Join(env.GridHome, "projects", scope.Project, "tasks")
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
