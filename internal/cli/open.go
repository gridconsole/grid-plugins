package cli

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// AppURLFile is the file in the Grid home where the app writes its base URL when it
// starts; grid open reads it. GRID_APP_URL, when set, wins over the file.
const AppURLFile = "app-url"

// appBaseURL is the app's address, "" when none is known or the text is not an http(s) address.
func appBaseURL(env Env) string {
	raw := strings.TrimSpace(env.getenv("GRID_APP_URL"))
	if raw == "" && env.GridHome != "" {
		b, err := os.ReadFile(filepath.Join(env.GridHome, AppURLFile))
		if err == nil {
			raw, _, _ = strings.Cut(strings.TrimSpace(string(b)), "\n")
			raw = strings.TrimSpace(raw)
		}
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return strings.TrimRight(raw, "/")
}

// wbOpenLaunchFile is the file in the Grid home where a local app writes its one-time
// launch code (one line, mode 0600) before it writes app-url. The app trades the code
// once at /api/launch for an HttpOnly cookie, so the page it opens can call the API.
const wbOpenLaunchFile = "launch-code"

// wbOpenLaunchCode is the code a local app wrote in the Grid home, "" when there is none.
func wbOpenLaunchCode(env Env) string {
	b, err := os.ReadFile(filepath.Join(env.GridHome, wbOpenLaunchFile))
	if err != nil {
		return ""
	}
	code, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return strings.TrimSpace(code)
}

// openCommand is the argv that opens target on the given OS. It is pure: the environment
// and the executable lookup come in as arguments. On windows the target is one argv item
// after the handler and no shell is involved, so a space or & in it needs no quoting.
func openCommand(goos, target string, getenv func(string) string, lookPath func(string) (string, error)) ([]string, error) {
	switch goos {
	case "darwin":
		return []string{"open", target}, nil
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", target}, nil
	default:
		if getenv("DISPLAY") == "" && getenv("WAYLAND_DISPLAY") == "" {
			return nil, errors.New("no display")
		}
		path, err := lookPath("xdg-open")
		if err != nil {
			return nil, err
		}
		return []string{path, target}, nil
	}
}

// openBrowser opens a URL in the person's browser; it fails when there is none to open.
func openBrowser(target string) error {
	argv, err := openCommand(runtime.GOOS, target, os.Getenv, exec.LookPath)
	if err != nil {
		return err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (e Env) openURL(target string) error {
	if e.OpenURL != nil {
		return e.OpenURL(target)
	}
	return openBrowser(target)
}

// runOpen opens the task page: grid open [T-NN | folder] [--json]. The task is found as
// grid where --task finds it, from the folder when none is given. With no operand and a
// folder in no task it opens Home (#/).
func runOpen(args []string, env Env) int {
	fs := newFlags("open", env)
	asJSON := fs.Bool("json", false, "print JSON")
	pos, err := taskArgs(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}
	if len(pos) > 1 {
		fmt.Fprintln(env.Stderr, "usage: grid open [--json] [<task id or folder>]")
		return 2
	}
	if env.GridHome == "" {
		fmt.Fprintln(env.Stderr, "grid: cannot find the Grid home; set GRID_HOME")
		return 1
	}
	operand := ""
	if len(pos) == 1 {
		operand = pos[0]
	}
	var t task.Task
	// No operand and a folder in no task (no error, no task): open Home. A resolve error, or
	// a folder inside a task, goes on below exactly as before.
	home := false
	if operand == "" {
		res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd})
		home = err == nil && res.TaskPath == ""
	}
	if home {
		// Home needs no task.
	} else if feed.IsID(operand) {
		// A task ULID is unique in the home; grid where finds it without a project.
		res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Task: operand})
		if err == nil && res.TaskPath == "" {
			err = where.ErrTaskNotFound
		}
		if err == nil {
			t, err = task.Read(res.TaskPath)
			t.Project = res.Project
		}
		if err != nil {
			return taskOperandError(env, "grid open", err)
		}
	} else {
		// T-NN anywhere in the home, a task folder, or the enclosing task: as grid task show.
		var err error
		if t, err = resolveTaskOperand(env, operand); err != nil {
			return taskOperandError(env, "grid open", err)
		}
	}
	// The route links by the task's ULID; T-NN is only what the page shows. A task that
	// has no ULID yet (an old home before its first Feed write) links by T-NN.
	route := "#/"
	if !home {
		linkID := t.ID
		if stable, err := feed.TaskID(t.Path); err == nil && feed.IsID(stable) {
			linkID = stable
		}
		route = "#/t/" + url.PathEscape(t.Project) + "/" + url.PathEscape(linkID) + "/overview"
	}
	base := appBaseURL(env)
	// GRID_APP_URL points at an app this machine does not run: open it as it is, start
	// nothing, and never hand it this home's launch code.
	local := strings.TrimSpace(env.getenv("GRID_APP_URL")) == ""
	if env.AppStart != nil && local {
		var code int
		if base, code = env.AppStart.ensure(env); code != 0 {
			return code
		}
	}
	if base == "" {
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				URL string `json:"url"`
			}{route})
		}
		fmt.Fprintln(env.Stdout, route)
		fmt.Fprintln(env.Stderr, "grid open: the app's address is not known. Start the app, then run grid open again.")
		return 0
	}
	target := base + "/" + route
	if local {
		if code := wbOpenLaunchCode(env); code != "" {
			// The app answers 303 to / with the cookie; the browser keeps the #route.
			target = base + "/api/launch?code=" + url.QueryEscape(code) + route
		} else {
			fmt.Fprintln(env.Stderr, "grid open: no launch code was found; the page opens without one.")
		}
	}
	openErr := env.openURL(target)
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			URL string `json:"url"`
		}{target})
	}
	if openErr != nil {
		fmt.Fprintln(env.Stdout, target)
		fmt.Fprintf(env.Stderr, "grid open: no browser was opened (%v). Open the address above.\n", openErr)
		return 0
	}
	fmt.Fprintf(env.Stdout, "opened %s\n", target)
	return 0
}
