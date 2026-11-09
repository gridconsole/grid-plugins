// Package hook answers Claude Code and Codex hook events. A hook never fails the
// session on an internal error; a stop guard may return 2 to request completion.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/usage"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Options is one hook call. Cwd is used when the event JSON carries none.
// Getenv defaults to os.Getenv; callers can isolate it in tests.
type Options struct {
	Event    string
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Resolver where.Resolver
	Cwd      string
	Getenv   func(string) string
}

// Handles reports whether Run answers the event, so callers can tell a handled
// event from one that silently prints nothing.
func Handles(event string) bool {
	return handler(event) != nil || componentEvent(event)
}

// Run handles one event. Internal errors allow the turn; stop guards may block. Unknown events and
// malformed input print nothing. SessionStart never creates project/task records.
func Run(o Options) int {
	if o.Getenv == nil {
		o.Getenv = os.Getenv
	}
	if run := handler(o.Event); run != nil {
		return run(o)
	}
	if componentEvent(o.Event) {
		return remaining(o)
	}
	return 0
}

func sessionStart(o Options) {
	var in struct {
		Cwd        string `json:"cwd"`
		Source     string `json:"source"`
		SessionID  string `json:"session_id"`
		Transcript string `json:"transcript_path"`
	}
	if o.Stdin != nil {
		const limit = 1 << 20
		b, err := io.ReadAll(io.LimitReader(o.Stdin, limit+1))
		if err != nil {
			o.log("read input", err)
			return
		}
		if len(b) > limit {
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
	cwd := in.Cwd
	if cwd == "" {
		cwd = o.Cwd
	}
	task, proj := o.Getenv("GRID_TASK"), o.Getenv("GRID_PROJECT")
	res, err := o.Resolver.Resolve(where.Query{Cwd: cwd, Task: task, Project: proj})
	note := ""
	if err != nil && (task != "" || proj != "") {
		// A stale GRID_TASK or GRID_PROJECT must not wipe the session's context.
		res, err = o.Resolver.Resolve(where.Query{Cwd: cwd})
		note = " GRID_TASK or GRID_PROJECT names something that is gone; ignored."
	}
	if err != nil {
		o.log("resolve session", err)
		return
	}
	recordSession(o, hookio.Input{HookEventName: "SessionStart", SessionID: in.SessionID, TranscriptPath: in.Transcript, Cwd: cwd, Source: in.Source})
	level, err := o.hookLevel("session-start", hookio.Input{Cwd: cwd})
	if err != nil {
		// Unreadable settings keep the default levels and say so, instead of hiding the context.
		note += " Hook settings cannot be read: " + err.Error() + "."
	}
	context := sessionContext(res, o.Resolver, note)
	brainProject := res.Project
	if brainProject == where.Personal {
		brainProject = ""
	}
	if idx, err := (brain.Store{GridHome: o.Resolver.GridHome, Project: brainProject}).Index(); err == nil {
		context += "\n" + idx.Text
	} else {
		o.log("brain index", err)
	}
	out := map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "SessionStart", "additionalContext": boundContext(context),
	}}
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	if envFile := o.Getenv("CLAUDE_ENV_FILE"); envFile != "" {
		if err := writeEnv(envFile, o.Resolver.GridHome, res.Task); err != nil {
			o.log("session environment", err)
			return
		}
	}
	if level != "off" {
		_, err = o.Stdout.Write(append(b, '\n'))
		o.log("session context", err)
	}
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

func sessionContext(r where.Result, resolver where.Resolver, note string) string {
	home := gridhome.Tilde(resolver.GridHome, resolver.Home)
	text := strings.Join([]string{
		"Grid tracks tasks, Jobs and shared knowledge in one Grid home.",
		"GRID_HOME is the Grid data folder " + home + "; GRID_PROJECTS is " + gridhome.Tilde(filepath.Join(resolver.GridHome, "projects"), resolver.Home) + ".",
		ProjectLine(r, resolver.Home) + note,
		"Task files are written by grid under the Grid home. Project records and task files live outside repositories.",
		"Grid agents cover delegated work in any folder: grid:grid-scout maps code; grid:grid-researcher researches facts; grid:grid-builder changes files; grid:grid-runner runs checks; grid:grid-reviewer reviews work; grid:grid-designer designs interfaces; grid:grid-checker checks pages; grid:grid-operator changes browser state; grid:grid-planner plans tasks; grid:grid-curator curates knowledge; grid:grid-library answers questions from the brain. Job briefs are rendered by grid job.",
		"The Grid brain holds shared topics, knowledge and decisions at home and project level.",
		"Approvals come from a person through the Grid app or grid approve in a terminal outside Claude Code or Codex.",
	}, "\n")
	return boundContext(text)
}

func boundContext(text string) string {
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

// guardEnv is the explicit environment the pure guard evaluators read. HOME and
// GRID_HOME come from the resolver so the guards judge the same home as the CLI.
func guardEnv(o Options) map[string]string {
	env := map[string]string{"HOME": o.Resolver.Home, "GRID_HOME": o.Resolver.GridHome, "PWD": o.Cwd, "GRID_MACHINE": o.Resolver.Machine}
	for _, key := range []string{"GRID_ROLE", "GRID_MEMBER", "GRID_PROJECTS", "GRID_PROJECT", "GRID_TASK", "GRID_PLAN_GUARD", "CLAUDE_PLUGIN_DATA", "CLAUDE_PLUGIN_ROOT"} {
		if v := o.Getenv(key); v != "" {
			env[key] = v
		}
	}
	return env
}

func toolUse(o Options) {
	in, tooLarge := hookio.Read(o.Stdin)
	if in == nil {
		return
	}
	// The CLI event is authoritative; a mismatched stdin event cannot select a
	// different decision envelope or bypass pre-tool guards.
	if strings.EqualFold(o.Event, "permissionrequest") {
		in.HookEventName = "PermissionRequest"
	} else {
		in.HookEventName = "PreToolUse"
	}
	if in.Cwd == "" {
		in.Cwd = o.Cwd
	}
	if tooLarge {
		// An event the guards could not read must not pass as one they approved.
		if guard.Guarded(in.ToolName) && o.Stdout != nil {
			d := hookio.Deny(fmt.Sprintf("This tool call is larger than %d bytes, more than the Grid guards read, so they cannot check it. Make the change in smaller steps.", hookio.InputLimit))
			if in.HookEventName == "PermissionRequest" {
				d.Event = "PermissionRequest"
			}
			_ = d.Write(o.Stdout)
		}
		return
	}
	env := guardEnv(o)
	// PermissionRequest also rechecks denies before granting a home fallback.
	// Convert a refusal to that event's decision shape after arbitration.
	decision := evaluateGuards(o, *in, env)
	if in.HookEventName == "PreToolUse" && decision.Behavior != "deny" {
		// A held agent start is refused before the history and planner records it would make.
		if decision = usageHold(o, *in, decision); decision.Behavior != "deny" {
			history(o, *in)
			recall(o, *in)
		}
	}
	if decision.Behavior == "allow" {
		CountHomeAllow(env, *in)
	}
	if in.HookEventName == "PermissionRequest" && decision.Behavior == "" {
		decision = forwardPermission(o, *in)
	}
	if in.HookEventName == "PermissionRequest" && decision.Behavior != "" {
		decision.Event = "PermissionRequest"
	}
	if o.Stdout != nil {
		o.log("guard output", decision.Write(o.Stdout))
	}
}

func evaluateGuards(o Options, in hookio.Input, env map[string]string) hookio.Decision {
	return hookio.Strongest(
		guard.EvaluateHost(in, o.Cwd, env),
		guard.EvaluateOnlyWriter(in, o.Cwd, env),
		guard.EvaluatePlan(in, o.Cwd, env),
		guard.EvaluateBrief(in, o.Cwd, env),
		guard.EvaluateHome(in, o.Cwd, env),
	)
}

// usageHold denies a tool call that starts an agent for a task at its usage
// budget. It never replaces a guard's deny. Without a task, with no budget, or
// when the usage data cannot be read, the guards' decision stands and the
// error is logged: the budget is a spending convenience, not a security gate.
func usageHold(o Options, in hookio.Input, decision hookio.Decision) hookio.Decision {
	if !usage.HoldsTool(in.ToolName) {
		return decision
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("usage hold", err)
		return decision
	}
	if dir == "" {
		return decision
	}
	held, err := (usage.Store{TaskPath: dir, GridHome: o.Resolver.GridHome}).Hold()
	if err != nil {
		o.log("usage hold", err)
		return decision
	}
	if !held.Hold {
		return decision
	}
	name := filepath.Base(dir)
	deny := hookio.Deny("Task " + name + " is at its usage budget (" + held.Reason + "), so this session cannot start a new agent for it. " +
		"Stop and ask the person: a person raises the budget with `grid usage budget set --task " + name + " --dollars N` (or --minutes N, --tokens N) or removes it with `grid usage budget clear --task " + name + "`. Do not run these yourself.")
	deny.Warnings = decision.Warnings
	return deny
}
