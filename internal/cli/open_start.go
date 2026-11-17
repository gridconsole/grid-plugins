package cli

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/logshift"
	"github.com/gridconsole/grid-plugins/internal/worker"
)

const (
	// AppServerBin and AppWorkerBin are the app's two programs. The Grid app installs
	// them; the plugin only finds and starts them.
	AppServerBin = "grid-server"
	AppWorkerBin = "grid-worker"
	// OpenStartLock is the file in the Grid home that makes two grid open calls start
	// the app one after the other. It is taken with the same fslock helper as .task.lock.
	OpenStartLock = "open-start.lock"
	// defaultAppWait is how long grid open waits for the server after it starts it.
	defaultAppWait = 30 * time.Second
	// appLogLimit and appLogFiles: grid open shifts <name>.log at this size, keeping the
	// active file and six old ones, before it starts a program.
	appLogLimit = 10 << 20
	appLogFiles = 7
	appPoll     = 50 * time.Millisecond
	dialTimeout = time.Second
)

// AppStart tells grid open how to find and start the app. A nil *AppStart starts
// nothing: OSEnv sets it, the test harness leaves it nil.
type AppStart struct {
	// Dirs, when set, are the only folders searched for the two programs. Nil searches
	// beside the grid program, then <Grid home>/bin, then PATH.
	Dirs []string
	// Wait is how long to wait for the app-url file after the start; 0 is 30 seconds.
	Wait time.Duration
}

// appPlan is what is not running.
type appPlan struct {
	base         string // the address app-url holds, "" when none
	server, work bool   // true: this one must be started
}

// state reads the two signs of a running app. The server runs when app-url holds an
// address something listens on. The worker runs when another handle holds its
// worker.lock (internal/worker, presence "reported"), as the app decides it.
// Neither is a PID probe.
func (a *AppStart) state(env Env) appPlan {
	p := appPlan{server: true, work: true}
	if base := appBaseURL(env); base != "" {
		p.base = base
		p.server = !listening(base)
	}
	if entries, err := (worker.Store{GridHome: env.GridHome}).List(); err == nil {
		for _, e := range entries {
			if e.Presence == "reported" {
				p.work = false
			}
		}
	}
	return p
}

// listening says whether something accepts a connection at the address.
func listening(base string) bool {
	u, err := url.Parse(base)
	if err != nil || u.Hostname() == "" {
		return false
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "https" {
			port = "443"
		}
	}
	c, err := net.DialTimeout("tcp", net.JoinHostPort(u.Hostname(), port), dialTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func (a *AppStart) searchDirs(env Env) []string {
	if a.Dirs != nil {
		return a.Dirs
	}
	var dirs []string
	if exe, err := env.executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
		if real, err := filepath.EvalSymlinks(exe); err == nil && filepath.Dir(real) != filepath.Dir(exe) {
			dirs = append(dirs, filepath.Dir(real))
		}
	}
	return append(dirs, filepath.Join(env.GridHome, "bin"))
}

// find returns the path of a program, or "" when it is in none of the places.
func (a *AppStart) find(env Env, name string) string {
	file := name
	if runtime.GOOS == "windows" {
		file += ".exe"
	}
	for _, dir := range a.searchDirs(env) {
		p := filepath.Join(dir, file)
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() && (runtime.GOOS == "windows" || st.Mode()&0o111 != 0) {
			return p
		}
	}
	if a.Dirs == nil {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// ensure starts the server and the worker when they do not run and waits for the
// server's address. It returns the app's base URL, or an exit code with the reason
// already printed: 3 when a program is missing, 1 when a start or the wait fails.
func (a *AppStart) ensure(env Env) (string, int) {
	if p := a.state(env); !p.server && !p.work {
		return p.base, 0
	}
	// Two grid open calls at once: the second waits here, then sees the app the first started.
	lockPath, err := env.MachineFolder.StartLockPath(env.GridHome, env.login())
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid open: cannot place %s: %v\n", OpenStartLock, err)
		return "", 1
	}
	lock, err := env.MachineFolder.OpenStartLock(lockPath)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid open: cannot open %s: %v\n", OpenStartLock, err)
		return "", 1
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		fmt.Fprintf(env.Stderr, "grid open: cannot take %s: %v\n", OpenStartLock, err)
		return "", 1
	}
	defer fslock.Unlock(lock)
	p := a.state(env)
	if !p.server && !p.work {
		return p.base, 0
	}
	type prog struct{ name, path string }
	var todo, missing []string
	var progs []prog
	for _, w := range []struct {
		name string
		need bool
	}{{AppWorkerBin, p.work}, {AppServerBin, p.server}} {
		if !w.need {
			continue
		}
		if path := a.find(env, w.name); path != "" {
			progs = append(progs, prog{w.name, path})
			todo = append(todo, w.name)
		} else {
			missing = append(missing, w.name)
		}
	}
	if len(missing) > 0 {
		// Nothing is started when one program is missing, so the app is never half up.
		where := strings.Join(a.searchDirs(env), ", ")
		if a.Dirs == nil {
			where += " and PATH"
		}
		fmt.Fprintf(env.Stderr, "grid open: %s not found (looked in %s). Install the Grid app, then run grid open again.\n", strings.Join(missing, " and "), where)
		return "", 3
	}
	if p.server {
		// The file holds the address of a server that is gone; the new server writes its own.
		_ = os.Remove(filepath.Join(env.GridHome, AppURLFile))
	}
	home, err := filepath.Abs(env.GridHome)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid open: cannot resolve the Grid home %s: %v\n", env.GridHome, err)
		return "", 1
	}
	for _, pr := range progs {
		if err := startDetached(env, pr.name, pr.path, appStartArgs(pr.name, home)); err != nil {
			fmt.Fprintf(env.Stderr, "grid open: could not start %s: %v\n", pr.name, err)
			return "", 1
		}
	}
	fmt.Fprintf(env.Stderr, "grid open: started %s.\n", strings.Join(todo, " and "))
	if !p.server {
		return p.base, 0
	}
	wait := a.Wait
	if wait <= 0 {
		wait = defaultAppWait
	}
	for deadline := time.Now().Add(wait); ; time.Sleep(appPoll) {
		if base := appBaseURL(env); base != "" && listening(base) {
			return base, 0
		}
		if time.Now().After(deadline) {
			break
		}
	}
	serverLog, _ := env.MachineFolder.LogPath(env.GridHome, env.login(), AppServerBin)
	fmt.Fprintf(env.Stderr, "grid open: %s did not write %s within %s (see %s). Run grid open again.\n",
		AppServerBin, AppURLFile, wait, serverLog)
	return "", 1
}

// appStartArgs is the command line each app program needs to run on its own: the
// app's contract, the same one the desktop uses for its server. grid-server runs only
// with the serve command and, with --home, a --who and a --role; grid-worker runs only
// with a command, and takes -home after it. home is an absolute path.
func appStartArgs(name, home string) []string {
	if name == AppServerBin {
		return []string{"serve", "--addr", "127.0.0.1:0", "--home", home, "--who", "local", "--role", "owner"}
	}
	return []string{"start", "-home", home}
}

// shiftAppLog shifts path to path.1 ... path.6 when it is appLogLimit bytes or more,
// as the worker's own log does. The first error that is not a missing file stops the
// shift; what moved stays moved, and the caller appends to path as before.
func shiftAppLog(path string) { logshift.Shift(path, appLogLimit, appLogFiles) }

// startDetached starts a program that outlives grid open: its own session or process
// group, no terminal, output in <Grid home>/logs/<name>.log. It runs with args, and
// the app finds the Grid home through GRID_HOME as well.
func startDetached(env Env, name, path string, args []string) error {
	logPath, err := env.MachineFolder.LogPath(env.GridHome, env.login(), name)
	if err != nil {
		return err
	}
	shiftAppLog(logPath)
	logf, err := env.MachineFolder.OpenLog(logPath)
	if err != nil {
		return err
	}
	defer logf.Close()
	cmd := exec.Command(path, args...)
	cmd.Dir = env.GridHome
	cmd.Env = append(os.Environ(), "GRID_HOME="+env.GridHome)
	cmd.Stdout, cmd.Stderr = logf, logf
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
