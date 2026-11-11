// Package lib runs one grid command inside the caller's process. The output on
// Stdout and Stderr and the exit code are the ones the grid binary gives for
// the same arguments, environment and Grid home.
//
// # What a call reads
//
// A call takes its environment, its working folder, its home folder and its
// streams from the Call and not from the host process: Environ is the whole
// environment (the last entry of a name wins, names fold case on Windows, nil
// is an empty one), Dir is the folder the command resolves its project and task
// from (empty means the Grid home), and nil Stdin is empty while nil Stdout and
// Stderr discard. The member, role and lease travel in Environ (GRID_MEMBER,
// GRID_ROLE, GRID_LEASE, GRID_LEASE_EPOCH, GRID_JOB and the other GRID_* names)
// or as the global flags at the front of Args (--member, --role, --lease-epoch,
// --grid-home). Two calls with different Grid homes can run at the same time and
// do not see each other.
//
// The Grid home is GridHome when set, else GRID_HOME in Environ, else .grid
// under Home. Home is the field, else HOME in Environ (USERPROFILE on Windows).
// The temporary folders are TMPDIR, TMP and TEMP from Environ, then /tmp and
// /var/tmp; the host's temporary folder is not asked for.
//
// A call does not claim that nothing is ever read from the host. These stay
// with the host process, as in the binary:
//   - the user name (USER, and the system account) that names a new task's
//     owner (internal/task) and, when Environ has no GRID_MEMBER and Args no
//     --member, the member of a Feed write: that case also runs `git config
//     user.email` in Dir with the host's environment;
//   - internal/account has its own fallback to the host's environment when it
//     is given no getenv, and the migrate plan reads CLAUDE_PLUGIN_DATA from
//     the host (internal/migrate);
//   - the process id of the caller (os.Getppid) that `ports claim`, `env up`
//     and the task lease record as their owner;
//   - the children that land, agent and routine runs start, which take the
//     host's environment, not Environ.
//
// Executable is the path of the grid binary, which doctor reports and routine
// schedules. Empty asks the host process for its own path, which is the right
// answer only when the host is grid; a host that is not grid sets it.
//
// OpenURL is what `grid open` calls with the task page address. Nil does
// nothing and returns nil; the lib never starts the system browser, and it
// never starts the app.
//
// # Exit codes
//
// The codes are the binary's: 0 done, 1 failed, 2 usage, 3 rejected (stale
// lease or invalid record; do not retry), 4 disk full. A panic inside the
// command is 2 and one line, "grid: internal error", on Stderr.
package lib

import (
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// Call is one grid command and everything it may read.
type Call struct {
	// Args is the command line after the program name: {"feed", "state", "--json"}.
	Args []string

	// GridHome is the Grid data folder. It wins over GRID_HOME in Environ; empty
	// reads that, then falls back to <Home>/.grid.
	GridHome string
	// Home is the person's home folder, used to shorten paths and to find the
	// default Grid home. Empty reads HOME (USERPROFILE on Windows) from Environ.
	Home string
	// Dir is the folder the command resolves its project and task from. Empty
	// means the Grid home.
	Dir string

	// Environ is the complete environment the command sees, as KEY=VALUE
	// entries. The last entry of a name wins and names fold case on Windows. Nil
	// is an empty environment: the host's own is not read.
	Environ []string

	// Stdin is what the command reads; nil is empty. Stdout and Stderr receive
	// its output; nil discards it.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Executable is the path of the grid binary. Empty asks the host process for
	// its own path.
	Executable string
	// OpenURL opens an address for `grid open`. Nil does nothing and returns nil.
	OpenURL func(string) error
}

// Run executes the call and returns the exit code the binary would: 0 done,
// 1 failed, 2 usage, 3 rejected, 4 disk full. A panic inside the command is
// reported like a crashed binary: the line "grid: internal error" on Stderr and
// exit 2.
func Run(c Call) (code int) {
	stderr := c.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	defer func() {
		if recover() == nil {
			return
		}
		code = 2
		defer func() { _ = recover() }() // a Stderr that panics too must not escape
		fmt.Fprintln(stderr, "grid: internal error")
	}()
	return cli.Run(c.Args, c.env(stderr))
}

// env builds the command's view of the machine from the call alone.
func (c Call) env(stderr io.Writer) cli.Env {
	vars := environment(c.Environ)
	if c.GridHome != "" {
		vars[key("GRID_HOME")] = c.GridHome
	}
	getenv := func(name string) string { return vars[key(name)] }

	home := c.Home
	if home == "" {
		if runtime.GOOS == "windows" {
			home = getenv("USERPROFILE")
		} else {
			home = getenv("HOME")
		}
	}
	if home != "" {
		home = gridhome.Real(home)
	}
	grid := gridhome.Dir(getenv, home)
	machine := ""
	if grid != "" {
		grid = gridhome.Real(grid)
		machine = gridhome.Machine(grid)
	}
	dir := c.Dir
	if dir == "" {
		dir = grid
	}
	stdin, stdout := c.Stdin, c.Stdout
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	openURL := c.OpenURL
	if openURL == nil {
		openURL = func(string) error { return nil }
	}
	e := cli.Env{
		Stdin: stdin, Stdout: stdout, Stderr: stderr,
		Cwd: dir, Home: home, GridHome: grid,
		TempRoots: tempRoots(getenv),
		Machine:   machine,
		Getenv:    getenv,
		OpenURL:   openURL,
	}
	if c.Executable != "" {
		path := c.Executable
		e.Executable = func() (string, error) { return path, nil }
	}
	return e
}

// environment reads KEY=VALUE entries into a map, the last entry winning.
func environment(environ []string) map[string]string {
	vars := make(map[string]string, len(environ)+1)
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name != "" {
			vars[key(name)] = value
		}
	}
	return vars
}

// key is the map key for an environment name: Windows ignores case.
func key(name string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(name)
	}
	return name
}

// tempRoots lists the folders treated as temporary: the call's own settings,
// then the fixed Unix ones. The host's os.TempDir is never asked.
func tempRoots(getenv func(string) string) []string {
	var roots []string
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		if v := getenv(name); v != "" {
			roots = append(roots, v)
		}
	}
	return append(roots, "/tmp", "/var/tmp")
}
