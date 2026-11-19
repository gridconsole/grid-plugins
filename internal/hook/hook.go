// Package hook answers Claude Code and Codex hook events. A hook never fails the
// session on an internal error; a stop guard may return 2 to request completion.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookevents"
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

	// runs collects the rules one Run consulted; Run sets it.
	runs *ruleRuns
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
	o.runs = &ruleRuns{}
	code := run(o)
	// One line per rule the run consulted, after the run has its answer.
	o.flushRuns()
	return code
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
	// Asked before anything below writes under the Grid home, so this session's own
	// records are not taken for the person's data.
	offer := cpOfferWanted(o)
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
		retireDenyRules(o, cwd)
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
	if offer {
		context += "\n" + cpOfferContext
	}
	out := map[string]any{"hookSpecificOutput": map[string]string{
		"hookEventName": "SessionStart", "additionalContext": context,
	}}
	if offer {
		out["systemMessage"] = cpOfferMessage
	}
	b, err := json.Marshal(out)
	if err != nil {
		return
	}
	if envFile := o.Getenv("CLAUDE_ENV_FILE"); envFile != "" {
		pathLine := voLauncherPathLine(gridhome.FindLauncherDir(o.Getenv, o.Resolver.Home, o.Resolver.GridHome), o.Getenv("PATH"))
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

// The first-run offer: shown to the person, and one line for the model. It is an
// offer only; the session goes on as before whatever the person answers.
const (
	cpOfferMessage = "Grid is not set up on this computer yet. Run /grid:first-run to set it up; it takes a minute."
	cpOfferContext = "Grid has no config and no data on this computer yet. Offer the person once to run the grid:first-run skill, and carry on with their request if they decline."
)

// cpOfferWanted reports a machine with no Grid config (gridconfig.Path and the
// older ~/.grid/config.json both absent) and no data under ~/.grid or the Grid
// home. It reads two config paths and one folder list: no network, no wait.
func cpOfferWanted(o Options) bool {
	if o.Resolver.Home == "" {
		return false
	}
	for _, p := range []string{gridconfig.Path(o.Getenv, o.Resolver.Home), gridconfig.OldPath(o.Resolver.Home)} {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); !errors.Is(err, fs.ErrNotExist) {
			return false
		}
	}
	root := gridconfig.DataRoot(o.Resolver.Home)
	if cpOfferHoldsData(root) || cpOfferHoldsData(o.Resolver.GridHome) {
		return false
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && cpOfferHoldsData(filepath.Join(root, e.Name())) {
			return false
		}
	}
	return true
}

// cpOfferHoldsData reports a folder that holds Grid data: VERSION or projects.
func cpOfferHoldsData(dir string) bool {
	if dir == "" {
		return false
	}
	for _, name := range []string{"VERSION", "projects"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
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

// retireDenyRules takes the deny rules an earlier setup wrote for task folders, the
// brain, project.md and the settings files out of the person's Claude settings and
// the setup records (the session home's, and the data folder's and its handles' too,
// since each wrote its own form), so a plugin update frees an agent that setup had
// blocked without the person running setup again. It writes nothing when there is
// nothing to take out, and a failure is logged and never reaches the session.
func retireDenyRules(o Options, cwd string) {
	projects := o.Getenv("GRID_PROJECTS")
	if projects != "" && !filepath.IsAbs(projects) {
		projects = filepath.Join(cwd, projects)
	}
	o.log("retire old deny rules", settings.RetireClaudeRules(settings.ClaudeSetupOptions{
		Home: o.Resolver.Home, GridHome: o.Resolver.GridHome, ConfigDir: o.Getenv("CLAUDE_CONFIG_DIR"), Projects: projects,
	}))
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
	text, replaced := replaceEnvBlock(string(old), block, home)
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
//
// Builds from before the markers wrote the same export lines with no markers
// around them. Outside the marked block, a line byte-equal to a line of block is
// one of those old copies and is dropped, with the blank lines that separate the
// copies. An old build may have put a different bin folder on PATH, so a line
// `export PATH='<dir>/bin':"$PATH"` with <dir> a plugin-dist folder, a .grid folder or
// home counts as one too. Every other line is kept, and a text with no old copy comes
// back as it was.
// isGridPathLine reports whether line is `export PATH='<dir>/bin':"$PATH"` with <dir>
// a plugin-dist folder, a .grid folder or the Grid home. Any other PATH line is the
// person's.
func isGridPathLine(line, home string) bool {
	const pre, post = "export PATH='", `/bin':"$PATH"`
	if !strings.HasPrefix(line, pre) || !strings.HasSuffix(line, post) {
		return false
	}
	dir := strings.TrimSuffix(strings.TrimPrefix(line, pre), post)
	if dir == "" || strings.Contains(dir, "'") {
		return false
	}
	if home != "" && dir == home {
		return true
	}
	switch filepath.Base(dir) {
	case "plugin-dist", ".grid":
		return true
	}
	return false
}

func replaceEnvBlock(text, block, home string) (string, bool) {
	legacy := map[string]bool{}
	for _, l := range strings.Split(block, "\n") {
		if l != "" && l != envBlockStart && l != envBlockEnd {
			legacy[l] = true
		}
	}
	isLegacy := func(l string) bool { return legacy[l] || isGridPathLine(l, home) }
	lines := strings.SplitAfter(text, "\n")
	var out strings.Builder
	found := false
	pending := "" // blank lines seen since the last kept line
	dropped := false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		if lines[i] == "\n" {
			pending += lines[i]
			continue
		}
		if isLegacy(strings.TrimSuffix(lines[i], "\n")) {
			pending, dropped = "", true
			continue
		}
		if !dropped {
			out.WriteString(pending)
		}
		pending = ""
		afterDropped := dropped
		dropped = false
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
					// Keep the blank line that set the old copies apart from the lines before.
					if s := out.String(); afterDropped && s != "" && !strings.HasSuffix(s, "\n\n") {
						out.WriteString("\n")
					}
					out.WriteString(block)
				}
				found = true
				i = end
				continue
			}
		}
		out.WriteString(lines[i])
	}
	if !dropped {
		out.WriteString(pending)
	}
	return out.String(), found
}

func sessionContext(r where.Result, resolver where.Resolver, note string) string {
	home := gridhome.Tilde(resolver.GridHome, resolver.Home)
	text := strings.Join([]string{
		"Grid tracks tasks, Jobs and shared knowledge in one Grid home.",
		"GRID_HOME is the Grid data folder " + home + "; GRID_PROJECTS is " + gridhome.Tilde(filepath.Join(resolver.GridHome, "projects"), resolver.Home) + ".",
		ProjectLine(r, resolver.Home) + note,
		"Task files live under the Grid home; use the grid verbs for task.md, the plan, the history and Jobs, and write any other task file straight into its folder. Project records and task files live outside repositories.",
		"The Grid brain holds shared topics, knowledge and decisions at home and project level.",
		"Approvals come from a person or an agent through the Grid app or grid approve; an agent Grid did not launch runs grid approve --agent <name>.",
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
		// The guards could not read this event. At plan-guard block it is denied;
		// at warn it passes with a warning and at off silently, never with an
		// allow: Claude Code's own permission flow still decides.
		if guard.Guarded(in.ToolName) && o.Stdout != nil {
			reason := fmt.Sprintf("This tool call is larger than %d bytes, more than the Grid guards read, so they cannot check it. Make the change in smaller steps.", hookio.InputLimit)
			d := o.overLimitDecision(*in, reason)
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
		brainAdopt(o, *in)
		history(o, *in)
	}
	if decision.Behavior == "allow" {
		CountHomeAllow(env, *in)
		o.note("home-allow", hookevents.OK)
	}
	if in.HookEventName == "PermissionRequest" && decision.Behavior != "" {
		decision.Event = "PermissionRequest"
	}
	if o.Stdout != nil {
		o.log("guard output", decision.Write(o.Stdout))
	}
}

// overLimitDecision is the answer to a guarded tool call the guards could not
// read. It follows the plan-guard level like the plan guard's own denials: block
// (or no Grid home, or settings that cannot be read) denies, warn says the reason
// as a warning and lets the call through, off says nothing.
func (o Options) overLimitDecision(in hookio.Input, reason string) hookio.Decision {
	deny := hookio.Deny(reason)
	if o.Resolver.GridHome == "" {
		return deny
	}
	level, err := o.hookLevel("plan-guard", in)
	if err != nil {
		deny.Warnings = append(deny.Warnings, err.Error())
		o.note("plan-guard", hookevents.Blocked)
		return deny
	}
	switch level {
	case "off":
		return hookio.Decision{}
	case "warn":
		o.note("plan-guard", hookevents.Warned)
		return hookio.Decision{Warnings: []string{"plan-guard: " + reason}}
	}
	o.note("plan-guard", hookevents.Blocked)
	return deny
}

// evaluateGuards asks the plan guard and the home guard, notes what each came to
// for the events log, and returns the stronger answer.
func evaluateGuards(o Options, in hookio.Input, env map[string]string) hookio.Decision {
	plan := guard.EvaluatePlan(in, o.Cwd, env)
	home := guard.EvaluateHome(in, o.Cwd, env)
	o.note("plan-guard", decisionOutcome(plan))
	return hookio.Strongest(plan, home)
}
