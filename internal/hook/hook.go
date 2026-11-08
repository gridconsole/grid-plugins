// Package hook answers Claude Code and Codex hook events. A hook never fails the
// session: every path returns exit code 0.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Options is one hook call. Cwd is used when the event JSON carries none.
type Options struct {
	Event    string
	Stdin    io.Reader
	Stdout   io.Writer
	Resolver where.Resolver
	Cwd      string
}

// Run handles one event and returns the exit code (always 0). Events it does not
// know print nothing.
func Run(o Options) int {
	switch strings.ToLower(o.Event) {
	case "sessionstart":
		sessionStart(o)
	}
	return 0
}

func sessionStart(o Options) {
	var in struct {
		Cwd string `json:"cwd"`
	}
	if o.Stdin != nil {
		if b, err := io.ReadAll(io.LimitReader(o.Stdin, 1<<20)); err == nil {
			_ = json.Unmarshal(b, &in) // bad JSON means no cwd; fall back
		}
	}
	cwd := in.Cwd
	if cwd == "" {
		cwd = o.Cwd
	}
	res, err := o.Resolver.Resolve(where.Query{Cwd: cwd})
	if err != nil {
		return
	}
	out := map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName":     "SessionStart",
		"additionalContext": ProjectLine(res, o.Resolver.Home),
	}}
	enc := json.NewEncoder(o.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(out)
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
