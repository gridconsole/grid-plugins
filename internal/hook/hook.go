// Package hook answers Claude Code and Codex hook events. A hook never fails the
// session on an internal error; a stop guard may return 2 to request completion.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/settings"
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
	run := handler(o.Event)
	if run == nil && componentEvent(o.Event) {
		run = remaining
	}
	if run == nil {
		return 0
	}
	defer o.logSlow(time.Now())
	return run(o)
}

// slowHook is the time from the start of a hook call to its output from which
// the call leaves a line in the hook log. It limits nothing: the call runs to
// its end either way.
const slowHook = 2 * time.Second

// logSlow writes `hook <event> slow: <ms> ms` when the call that began at start
// has taken slowHook or more, so a hook that Claude Code cut off at its own limit
// can be told from one that was never slow.
func (o Options) logSlow(start time.Time) {
	d := time.Since(start)
	if d < slowHook {
		return
	}
	name := o.Event
	for _, e := range events {
		if strings.EqualFold(name, e.Name) {
			name = e.Name
		}
	}
	o.log("hook "+name+" slow", fmt.Errorf("%d ms", d.Milliseconds()))
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
	if level != "off" {
		syncAgents(o)
	}
	brainProject := res.Project
	if brainProject == where.Personal {
		brainProject = ""
	}
	var brainText *string
	if idx, err := (brain.Store{GridHome: o.Resolver.GridHome, Project: brainProject}).Index(); err == nil {
		brainText = &idx.Text
	} else {
		o.log("brain index", err)
	}
	context := assembleContext(sessionContext(res, o.Resolver, note), jobContext(o, res), brainText)
	out := map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "SessionStart", "additionalContext": context,
	}}
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	if envFile := o.Getenv("CLAUDE_ENV_FILE"); envFile != "" {
		pathLine := voLauncherPathLine(o.Resolver.GridHome, o.Getenv("PATH"))
		if pathLine == "" {
			pathLine = pluginPathLine(o.Getenv("CLAUDE_PLUGIN_ROOT"), o.Getenv("PATH"))
		}
		if err := writeEnv(envFile, o.Resolver.GridHome, res.Task, pathLine); err != nil {
			o.log("session environment", err)
			return
		}
	}
	if level != "off" {
		_, err = o.Stdout.Write(append(b, '\n'))
		o.log("session context", err)
	}
}

// syncAgents renders the plugin's agent templates into the person's agents folder.
// It writes a file only when the template or the person's model and effort changed,
// and a failure is logged and never reaches the session.
func syncAgents(o Options) {
	root := o.Getenv("CLAUDE_PLUGIN_ROOT")
	if root == "" {
		return
	}
	dir, err := settings.ClaudeAgentsDir(o.Resolver.Home, o.Getenv("CLAUDE_CONFIG_DIR"))
	if err == nil {
		_, err = settings.SyncAgents(settings.SyncOptions{
			Home:         o.Resolver.GridHome,
			TemplatesDir: settings.AgentTemplatesPath(root),
			AgentsDir:    dir,
		})
	}
	o.log("agent sync", err)
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }

// pluginPathLine returns the line that puts the plugin's bin folder on PATH, or ""
// when the plugin root is unset, bin/grid is missing, the folder is already on PATH,
// or the host is Windows (its env output is unchanged).
func pluginPathLine(root, pathEnv string) string {
	if root == "" || runtime.GOOS == "windows" {
		return ""
	}
	bin := filepath.Join(root, "bin")
	if st, err := os.Stat(filepath.Join(bin, "grid")); err != nil || st.IsDir() {
		return ""
	}
	for _, p := range filepath.SplitList(pathEnv) {
		if p != "" && filepath.Clean(p) == bin {
			return ""
		}
	}
	return "export PATH=" + shellQuote(bin) + ":\"$PATH\"\n"
}

// The Grid lines in the session env file sit between these two marker lines, so a
// later SessionStart replaces them instead of adding another copy.
const (
	envBlockStart = "# >>> grid-session-env >>>"
	envBlockEnd   = "# <<< grid-session-env <<<"
)

// writeEnv puts the Grid block in the env file. A block from an earlier run is
// replaced in place; every line outside it is kept. The file is written to a temp
// file next to it and renamed over it.
func writeEnv(path, home, task string, extra ...string) error {
	q := shellQuote
	block := envBlockStart + "\nexport GRID_HOME=" + q(home) + "\nexport GRID_PROJECTS=" + q(filepath.Join(home, "projects")) + "\n"
	if task != "" {
		block += "export GRID_TASK=" + q(task) + "\n"
	} else {
		block += "unset GRID_TASK\n"
	}
	block += strings.Join(extra, "") + envBlockEnd + "\n"

	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	mode := os.FileMode(0o600)
	old, err := os.ReadFile(path)
	switch {
	case err == nil:
		if st, serr := os.Stat(path); serr == nil {
			mode = st.Mode().Perm()
		}
	case errors.Is(err, os.ErrNotExist):
		old = nil
	default:
		return err
	}
	text, replaced := replaceEnvBlock(string(old), block)
	if !replaced {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += "\n" + block
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".grid-env-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.WriteString(text)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(name, mode)
	}
	if err == nil {
		err = os.Rename(name, path)
	}
	if err != nil {
		os.Remove(name)
	}
	return err
}

// replaceEnvBlock swaps the first marked block in text for block and drops any
// later marked block. A start marker with no end marker after it is an ordinary
// line. It reports whether a block was found.
func replaceEnvBlock(text, block string) (string, bool) {
	lines := strings.SplitAfter(text, "\n")
	var out strings.Builder
	found := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if line == envBlockStart {
			// The block runs to the next end marker unless another start comes first.
			end := -1
			for j := i + 1; j < len(lines); j++ {
				l := strings.TrimRight(lines[j], "\r\n")
				if l == envBlockEnd {
					end = j
					break
				}
				if l == envBlockStart {
					break
				}
			}
			if end >= 0 {
				if !found {
					out.WriteString(block)
				}
				found = true
				i = end
				continue
			}
		}
		out.WriteString(lines[i])
	}
	return out.String(), found
}

func sessionContext(r where.Result, resolver where.Resolver, note string) string {
	home := gridhome.Tilde(resolver.GridHome, resolver.Home)
	text := strings.Join([]string{
		"Grid tracks tasks, Jobs and shared knowledge in one Grid home.",
		"GRID_HOME is the Grid data folder " + home + "; GRID_PROJECTS is " + gridhome.Tilde(filepath.Join(resolver.GridHome, "projects"), resolver.Home) + ".",
		ProjectLine(r, resolver.Home) + note,
		"Task files are written by grid under the Grid home. Project records and task files live outside repositories.",
		"The Grid brain holds shared topics, knowledge and decisions at home and project level.",
		"Approvals come from a person through the Grid app or grid approve in a terminal outside Claude Code or Codex.",
	}, "\n")
	return boundContext(text)
}

// assembleContext joins the session text, the Job lines and the brain index in
// that order and bounds the whole, so a long index is what gets cut.
func assembleContext(session, jobLines string, brainText *string) string {
	context := session
	if jobLines != "" {
		context += "\n" + jobLines
	}
	if brainText != nil {
		context += "\n" + *brainText
	}
	return boundContext(context)
}

// jobContext names the Job a worker started this session for, where its brief is
// and the command that posts its result. It is empty for a session that has no
// valid GRID_JOB or no resolved task.
func jobContext(o Options, res where.Result) string {
	id := o.Getenv("GRID_JOB")
	if res.Task == "" || !job.ValidID(id) || strings.Trim(strings.SplitN(id[2:], ".", 2)[0], "0") == "" {
		return ""
	}
	first := "You run Job " + id + " of task " + res.Task
	if n, err := strconv.Atoi(o.Getenv("GRID_ATTEMPT")); err == nil && n > 0 {
		first += " (attempt " + strconv.Itoa(n) + ")"
	}
	show := "grid job show " + res.Task + " " + id
	brief := "Your brief: " + show + "."
	if res.TaskPath != "" {
		path := filepath.Join(res.TaskPath, "jobs", id, "brief.md")
		if st, err := os.Stat(path); err == nil && st.Mode().IsRegular() {
			brief = "Your brief: " + gridhome.Tilde(path, o.Resolver.Home) + " (or: " + show + ")."
		}
	}
	return strings.Join([]string{
		first + ".",
		brief,
		"Post your result with grid job result --task " + res.Task + " --job " + id + " --body <json> before you end, also when blocked.",
	}, "\n")
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
		history(o, *in)
	}
	if decision.Behavior == "allow" {
		CountHomeAllow(env, *in)
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
		guard.EvaluatePlan(in, o.Cwd, env),
		guard.EvaluateHome(in, o.Cwd, env),
	)
}
