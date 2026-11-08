package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
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
	if len(args) > 0 && args[0] == "task" {
		return runTaskLease(action, args[1:], env)
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
	if action != "status" && !contract.ValidWorker(*holder) {
		fmt.Fprintln(env.Stderr, "lease: --holder must be <member>@<machine ULID> with no slash or whitespace (contract/schemas/lease.json)")
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
	store := lease.Store{Project: scope.Project, ProjectDir: filepath.Join(env.GridHome, "projects", scope.Project)}
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

func runTaskLease(action string, args []string, env Env) int {
	if action == "renew" {
		fmt.Fprintln(env.Stderr, "task lease has no renewal; use take, release or status")
		return 2
	}
	fs := newFlags("lease "+action+" task", env)
	task := fs.String("task", "", "task folder or id")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	var pid int
	var start, sessionID, epoch string
	if action == "take" {
		fs.IntVar(&pid, "pid", 0, "holder process id")
		fs.StringVar(&start, "pid-start", "", "holder process start in RFC3339")
		fs.StringVar(&sessionID, "session", "", "registered local session id")
	}
	if action == "release" {
		fs.StringVar(&epoch, "epoch", "", "required generation.fence")
	}
	if code, stop := parse(fs, args, env); stop {
		return code
	}
	usage := func(message string) int { fmt.Fprintln(env.Stderr, "lease: "+message); return 2 }
	if action == "take" {
		if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
			return livenessError(env, errors.New("task lease needs a Unix host for process identity"))
		}
		if sessionID != "" && (pid != 0 || start != "") {
			return usage("use either --session or --pid and --pid-start")
		}
		if sessionID == "" && (pid < 1 || start == "") {
			return usage("--pid and --pid-start, or --session, are required")
		}
		if sessionID == "" {
			parsed, err := time.Parse(time.RFC3339Nano, start)
			if err != nil {
				return usage("--pid-start must be RFC3339")
			}
			start = contract.FormatTime(parsed)
		}
	}
	if action == "release" && epoch == "" {
		return usage("--epoch is required")
	}
	store, err := feedStore(env, *task, *project)
	if err != nil {
		return livenessError(env, err)
	}
	leases := lease.TaskStore{Feed: store}
	if action == "status" {
		tenure, err := leases.Read("task")
		if errors.Is(err, os.ErrNotExist) {
			tenure = lease.Tenure{Lease: feed.Lease{Lease: "task"}}
		} else if err != nil {
			return livenessError(env, err)
		}
		state := "free"
		if tenure.Holder != nil {
			machine, err := feed.MachineID(env.GridHome)
			if err != nil {
				return livenessError(env, err)
			}
			state = "remote"
			if strings.HasSuffix(*tenure.Holder, "@"+machine) {
				live := session.Live{}
				if tenure.PID != nil {
					live.PID = *tenure.PID
				}
				if tenure.PIDStart != nil {
					live.PIDStart = *tenure.PIDStart
				}
				state = map[session.ProcessState]string{session.Alive: "alive", session.Dead: "dead", session.Unknown: "unknown"}[session.ProbeLocal(live)]
			}
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				lease.Tenure
				Process string `json:"process"`
			}{tenure, state})
		}
		if tenure.Holder == nil {
			fmt.Fprintln(env.Stdout, "task lease free")
			return 0
		}
		fmt.Fprintf(env.Stdout, "task lease held by %s since %s\ngen: %d\nfence: %d\npid: %d\nprocess: %s\n", *tenure.Holder, tenure.Since, tenure.Gen, tenure.Fence, *tenure.PID, state)
		return 0
	}
	writer, err := feedWriter(env, "task", epoch)
	if err != nil {
		return livenessError(env, err)
	}
	if action == "release" {
		if writer.Gen < 0 || writer.Fence < 1 {
			return usage("--epoch must be a nonnegative generation and positive fence")
		}
		if err := leases.ReleaseTask(writer); err != nil {
			return livenessError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, map[string]any{"lease": "task", "holder": nil, "gen": writer.Gen, "fence": writer.Fence})
		}
		fmt.Fprintf(env.Stdout, "task lease released, fence %d\n", writer.Fence)
		return 0
	}
	if sessionID != "" {
		rows, err := (session.Store{TaskDir: store.TaskDir}).List()
		if err != nil {
			return livenessError(env, err)
		}
		found := false
		for _, r := range rows {
			if r.SessionID == sessionID && strings.HasSuffix(writer.Worker, "@"+r.MachineID) {
				found, pid, start = true, r.PID, r.PIDStart
				break
			}
		}
		if !found {
			return livenessError(env, fmt.Errorf("local session %q not found", sessionID))
		}
		parsed, err := time.Parse(time.RFC3339Nano, start)
		if err != nil {
			// Older hooks may store proc ticks or raw lstart. Prove that identity
			// before resolving its canonical timestamp; never bless a reused pid.
			if session.ProbeLocal(session.Live{PID: pid, PIDStart: start}) != session.Alive {
				return livenessError(env, errors.New("session process identity is not alive"))
			}
			parsed, err = session.ProcessStart(pid)
			if err != nil {
				return livenessError(env, err)
			}
			if session.ProbeLocal(session.Live{PID: pid, PIDStart: start}) != session.Alive {
				return livenessError(env, errors.New("session process identity changed"))
			}
		}
		start = contract.FormatTime(parsed)
	}
	// Serialize CLI grants through their first post, including handover recovery.
	// The lease owner still checks and probes under the lease file's own lock.
	dir := filepath.Join(store.Dir(), "lease")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return livenessError(env, err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return livenessError(env, err)
	}
	if !info.IsDir() {
		return livenessError(env, errors.New("lease must be a real directory"))
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(dir, "task-take.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return livenessError(env, err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return livenessError(env, err)
	}
	defer fslock.Unlock(lock)
	tenure, err := leases.Read("task")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return livenessError(env, err)
	}
	body := map[string]any{}
	recovery := false
	if tenure.Holder != nil && *tenure.Holder == writer.Worker {
		records, err := store.Read(feed.Filter{Type: "handover"})
		if err != nil {
			return livenessError(env, err)
		}
		recovery = true
		for _, r := range records.Records {
			if r.Worker == writer.Worker && r.Gen == tenure.Gen && r.Fence == tenure.Fence && r.Verdict == "counted" {
				recovery = false
				break
			}
		}
		if recovery {
			// Retain adoption metadata after a failed post. The file is only a
			// retry hint; the lease and Feed remain authoritative.
			var pending struct {
				Fence int64
				Body  map[string]any
			}
			if json.NewDecoder(lock).Decode(&pending) == nil && pending.Fence == tenure.Fence && pending.Body != nil {
				body = pending.Body
			}
		}
	}
	if !recovery {
		writer.Gen = tenure.Gen
		tenure, err = leases.TakeTask(writer, pid, start)
		if err != nil && !errors.Is(err, lease.ErrTaskHeld) {
			return livenessError(env, err)
		}
		if err != nil {
			var old lease.Tenure
			tenure, old, err = leases.AdoptTask(writer, pid, start, session.ProbeLocal)
			if err != nil {
				return livenessError(env, err)
			}
			body = map[string]any{"adopted": "session-gone", "from": *old.Holder}
		}
	}
	writer.Gen, writer.Fence = tenure.Gen, tenure.Fence
	printGrant := func() int {
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Lease   string  `json:"lease"`
				Holder  *string `json:"holder"`
				Gen     int64   `json:"gen"`
				Fence   int64   `json:"fence"`
				Since   string  `json:"since"`
				Adopted any     `json:"adopted"`
			}{"task", tenure.Holder, tenure.Gen, tenure.Fence, tenure.Since, body["adopted"]})
		}
		fmt.Fprintf(env.Stdout, "task lease taken, fence %d\nepoch %d.%d\n", tenure.Fence, tenure.Gen, tenure.Fence)
		return 0
	}
	// Save only retry metadata under the same command lock before posting.
	if !recovery {
		if err = lock.Truncate(0); err == nil {
			_, err = lock.Seek(0, 0)
		}
		if err == nil {
			err = json.NewEncoder(lock).Encode(struct {
				Fence int64
				Body  map[string]any
			}{tenure.Fence, body})
		}
		if err == nil {
			err = feed.Sync(lock)
		}
	}
	if err == nil {
		_, err = store.Handover(feed.Input{Body: body}, writer)
	}
	code := printGrant()
	if err != nil {
		return livenessError(env, err)
	}
	return code
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
