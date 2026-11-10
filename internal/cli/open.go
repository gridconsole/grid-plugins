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

// openBrowser opens a URL in the person's browser; it fails when there is none to open.
func openBrowser(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	default:
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return errors.New("no display")
		}
		path, err := exec.LookPath("xdg-open")
		if err != nil {
			return err
		}
		cmd = exec.Command(path, target)
	}
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
// grid where --task finds it, from the folder when none is given.
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
	if feed.IsID(operand) {
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
	linkID := t.ID
	if stable, err := feed.TaskID(t.Path); err == nil && feed.IsID(stable) {
		linkID = stable
	}
	route := "#/t/" + url.PathEscape(t.Project) + "/" + url.PathEscape(linkID) + "/overview"
	base := appBaseURL(env)
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
