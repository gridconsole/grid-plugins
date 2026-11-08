// Package hook answers Claude Code and Codex hook events. A hook never fails the
// session: every path returns exit code 0.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Options is one hook call. Cwd is used when the event JSON carries none.
// Getenv defaults to os.Getenv; callers can isolate it in tests.
type Options struct {
	Event    string
	Stdin    io.Reader
	Stdout   io.Writer
	Resolver where.Resolver
	Cwd      string
	Getenv   func(string) string
}

// Run handles one event and returns the exit code (always 0). Unknown events and
// malformed input print nothing. SessionStart never creates project/task records.
func Run(o Options) int {
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	switch strings.ToLower(o.Event) {
	case "sessionstart":
		sessionStart(o)
	}
	return 0
}

func sessionStart(o Options) {
	var in struct {
		Cwd    string `json:"cwd"`
		Source string `json:"source"`
	}
	if o.Stdin != nil {
		const limit = 1 << 20
		b, err := io.ReadAll(io.LimitReader(o.Stdin, limit+1))
		if err != nil || len(b) > limit {
			return
		}
		if len(strings.TrimSpace(string(b))) > 0 {
			if strings.TrimSpace(string(b)) == "null" || json.Unmarshal(b, &in) != nil {
				return
			}
		}
	}
	if o.Resolver.GridHome == "" || o.Stdout == nil {
		return
	}
	if n, err := gridhome.ReadFormat(o.Resolver.GridHome); err == nil {
		if n != gridhome.FormatVersion {
			return
		}
	} else if !os.IsNotExist(err) {
		return
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = o.Cwd
	}
	res, err := o.Resolver.Resolve(where.Query{Cwd: cwd, Task: o.Getenv("GRID_TASK"), Project: o.Getenv("GRID_PROJECT")})
	if err != nil {
		return
	}
	context := sessionContext(res, o.Resolver)
	out := map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "SessionStart", "additionalContext": context,
	}}
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	if envFile := o.Getenv("CLAUDE_ENV_FILE"); envFile != "" {
		if err := writeEnv(envFile, o.Resolver.GridHome, res.Task); err != nil {
			return
		}
	}
	_, _ = o.Stdout.Write(append(b, '\n'))
}

func writeEnv(path, home, task string) error {
	q := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
	text := "\nexport GRID_HOME=" + q(home) + "\nexport GRID_PROJECTS=" + q(filepath.Join(home, "projects")) + "\n"
	if task != "" {
		text += "export GRID_TASK=" + q(task) + "\n"
	} else {
		text += "unset GRID_TASK\n"
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func sessionContext(r where.Result, resolver where.Resolver) string {
	home := gridhome.Tilde(resolver.GridHome, resolver.Home)
	text := strings.Join([]string{
		"Grid tracks tasks, Jobs and shared knowledge in one Grid home.",
		"GRID_HOME is the Grid data folder " + home + "; GRID_PROJECTS is " + gridhome.Tilde(filepath.Join(resolver.GridHome, "projects"), resolver.Home) + ".",
		ProjectLine(r, resolver.Home),
		"Task files are written by grid under the Grid home. Project records and task files live outside repositories.",
		"Grid agents cover delegated work in any folder: grid:grid-scout maps code; grid:grid-researcher researches facts; grid:grid-builder changes files; grid:grid-runner runs checks; grid:grid-reviewer reviews work; grid:grid-designer designs interfaces; grid:grid-checker checks pages; grid:grid-operator changes browser state; grid:grid-planner plans tasks; grid:grid-curator curates knowledge; grid:grid-library answers questions from the brain. Job briefs are rendered by grid job.",
		"The Grid brain holds shared topics, knowledge and decisions at home and project level.",
		"Approvals come from a person through !grid approve in Claude Code bash mode, or grid approve in a shell.",
	}, "\n")
	const limit = 9900
	runes := []rune(text)
	if len(runes) > limit {
		text = string(runes[:limit]) + "\nContext truncated."
	}
	return text
}

// ProjectLine states what grid where resolved, as a fact.
func ProjectLine(r where.Result, home string) string {
	switch {
	case r.Task != "":
		return fmt.Sprintf("Grid task: %s in project %s, at %s.", r.Task, r.Project, gridhome.Tilde(r.TaskPath, home))
	case r.Rule == where.RulePersonal:
		return "Grid project: Personal. This folder is not inside any project."
	case r.Record:
		return fmt.Sprintf("Grid project: %s, folder %s.", r.Project, gridhome.Tilde(r.Folder, home))
	default:
		return fmt.Sprintf("Grid project: %s, folder %s. It has no Grid record yet; the first task makes one.", r.Project, gridhome.Tilde(r.Folder, home))
	}
}
