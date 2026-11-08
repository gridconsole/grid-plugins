package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

func runTaskLease(args []string, env Env) int {
	return runLease(args, env)
}

// taskRetry is what task-take.lock holds: the granted fence and its handover body.
type taskRetry struct {
	Fence int64
	Body  map[string]any
}

// afterTaskGrant is a test hook, called once a task grant is durable and before its handover.
// A test that exits the process in it leaves the disk as a crash at that point would.
var afterTaskGrant func()

// Test barriers around the held-check/adoption gap and CLI lock acquisition.
var beforeTaskAdopt func()
var beforeTaskLeaseLock func(action string, lock *os.File)

var processStates = map[session.ProcessState]string{session.Alive: "alive", session.Dead: "dead", session.Unknown: "unknown"}

func runTaskLeaseAction(action string, args []string, env Env) int {
	if action == "renew" {
		fmt.Fprintln(env.Stderr, "task lease has no renewal; use take, release or status")
		return 2
	}
	fs := newFlags("lease "+action+" task", env)
	task := fs.String("task", "", "task folder or id")
	project := fs.String("project", "", "project name or id")
	asJSON := fs.Bool("json", false, "print JSON")
	var pid int
	var gen int64
	var adopt, takeover bool
	var expected, offline string
	var start, sessionID, epoch string
	if action == "take" {
		fs.Int64Var(&gen, "gen", 0, "task generation")
		fs.BoolVar(&adopt, "adopt", false, "adoption of a provably gone local session")
		fs.BoolVar(&takeover, "takeover", false, "person-authorized takeover after 15 minutes offline")
		fs.StringVar(&expected, "expected-epoch", "", "current generation.fence required for adoption or takeover")
		fs.StringVar(&offline, "server-offline-since", "", "server-observed offline time in RFC3339; never infer from lease age")
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
				state = processStates[session.ProbeLocal(live)]
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
	if action == "take" && store.TaskID == "" {
		store.TaskID, err = feed.EnsureTaskID(store.TaskDir)
		if err != nil {
			return livenessError(env, err)
		}
		leases.Feed = store
	}
	if action == "take" {
		epoch = fmt.Sprintf("%d.0", gen)
	}
	writer, err := orchestratorWriter(env, "task", epoch)
	if err != nil {
		return livenessError(env, err)
	}
	if !contract.ValidWorker(writer.Worker) {
		return usage("writer must be <member>@<machine ULID> with no slash or whitespace")
	}
	genSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "gen" {
			genSet = true
		}
	})
	policy := lease.TaskTake{Adopt: adopt, Takeover: takeover}
	if action == "take" {
		typ, _ := contract.LookupType("handover")
		if !slices.Contains(typ.Roles, contract.NormalizeRole(writer.Author.Role)) {
			return livenessError(env, errors.New("task acquisition requires a task-holder role"))
		}
		if gen < 0 || adopt && takeover {
			return usage("--gen must be nonnegative; select at most one of --adopt and --takeover")
		}
		if adopt || takeover {
			old, err := orchestratorWriter(env, "task", expected)
			if err != nil || old.Gen < 0 || old.Fence < 1 {
				return usage("adoption or takeover requires --expected-epoch gen.fence with a positive fence")
			}
			policy.Gen, policy.Fence = old.Gen, old.Fence
		}
		if offline != "" {
			policy.OfflineSince, err = time.Parse(time.RFC3339Nano, offline)
			if err != nil {
				return usage("--server-offline-since must be RFC3339")
			}
		}
	}
	if action == "release" {
		if writer.Gen < 0 || writer.Fence < 1 {
			return usage("--epoch must be a nonnegative generation and positive fence")
		}
		// A free release must not create a directory or lock file.
		current, err := leases.Read("task")
		if errors.Is(err, os.ErrNotExist) || err == nil && current.Holder == nil {
			return livenessError(env, lease.ErrTaskFree)
		}
		if err != nil {
			return livenessError(env, err)
		}
	}
	if action == "take" {
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
		// A stored identity that is not this live process would let a later taker adopt the real holder.
		if state := session.ProbeLocal(session.Live{PID: pid, PIDStart: start}); state != session.Alive {
			return livenessError(env, fmt.Errorf("task lease take refused: pid %d started %s is not a live process on this host (%s)", pid, start, processStates[state]))
		}
	}
	// Serialize release with the held check, adoption and first handover post.
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
	if beforeTaskLeaseLock != nil {
		beforeTaskLeaseLock(action, lock)
	}
	if err := fslock.LockExclusive(lock); err != nil {
		return livenessError(env, err)
	}
	defer fslock.Unlock(lock)
	if action == "release" {
		if err := leases.ReleaseTask(writer); err != nil {
			return livenessError(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, map[string]any{"lease": "task", "holder": nil, "gen": writer.Gen, "fence": writer.Fence})
		}
		fmt.Fprintf(env.Stdout, "task lease released, fence %d\n", writer.Fence)
		return 0
	}
	tenure, err := leases.Read("task")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return livenessError(env, err)
	}
	body := map[string]any{}
	recovery := false
	// Only the granted process finishes its handover; another process of the same member is a new taker.
	if !adopt && !takeover && tenure.Holder != nil && *tenure.Holder == writer.Worker && tenure.PID != nil && *tenure.PID == pid && tenure.PIDStart != nil && *tenure.PIDStart == start {
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
			var pending taskRetry
			if json.NewDecoder(lock).Decode(&pending) == nil && pending.Fence == tenure.Fence && pending.Body != nil {
				body = pending.Body
			}
		}
	}
	if !recovery {
		if !genSet && tenure.Lease.Lease != "" && tenure.Fence > 0 {
			writer.Gen = tenure.Gen
		}
		// Save the retry metadata before the tenure is written, so a crash after the
		// grant still leaves the adoption for the retry to post.
		leases.BeforeTaskGrant = func(next, old lease.Tenure) error {
			body = map[string]any{}
			if old.Holder != nil {
				if takeover {
					body = map[string]any{"reason": "person takeover after server-observed offline interval", "from": *old.Holder}
				} else {
					body = map[string]any{"adopted": "session-gone", "from": *old.Holder}
				}
			}
			if err := lock.Truncate(0); err != nil {
				return err
			}
			if _, err := lock.Seek(0, 0); err != nil {
				return err
			}
			if err := json.NewEncoder(lock).Encode(taskRetry{next.Fence, body}); err != nil {
				return err
			}
			return feed.Sync(lock)
		}
		tenure, err = leases.TakeTask(writer, pid, start, policy)
		if err != nil && (adopt || takeover || !errors.Is(err, lease.ErrTaskHeld)) {
			return livenessError(env, err)
		}
		if err != nil {
			if beforeTaskAdopt != nil {
				beforeTaskAdopt()
			}
			tenure, _, err = leases.AdoptTask(writer, pid, start, session.ProbeLocal)
			if err != nil {
				return livenessError(env, err)
			}
		}
		if afterTaskGrant != nil {
			afterTaskGrant()
		}
	}
	writer.Gen, writer.Fence = tenure.Gen, tenure.Fence
	printGrant := func() int {
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				lease.Tenure
				Adopted any `json:"adopted"`
			}{tenure, body["adopted"]})
		}
		fmt.Fprintf(env.Stdout, "task lease taken, fence %d\nepoch %d.%d\n", tenure.Fence, tenure.Gen, tenure.Fence)
		return 0
	}
	_, err = store.Handover(feed.Input{Body: body}, writer)
	code := printGrant()
	if err != nil {
		return livenessError(env, err)
	}
	return code
}
