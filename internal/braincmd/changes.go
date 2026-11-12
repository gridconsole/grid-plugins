package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const (
	changesUsage = "Usage: grid brain changes [--level home|project] [--project <p>] [--since <RFC3339>] [--limit <n>] [--json]"
	revertUsage  = "Usage: grid brain revert <topic>/<name> --to <n> [--reason <why>] [--level home|project] [--project <p>] [--json]"
)

// failWith prints a store error the way every brain verb does. A project or
// task the caller cannot see prints the plain line and goes to RecordError, so
// the CLI's standard error envelope carries its code; any other error keeps the
// JSON object under --json.
func failWith(env Env, asJSON bool, err error) int {
	if env.RecordError != nil && hasCode(err) {
		env.RecordError(err)
		asJSON = false
	}
	if asJSON {
		_ = json.NewEncoder(env.Stderr).Encode(map[string]string{"error": err.Error()})
	} else {
		fmt.Fprintf(env.Stderr, "brain: %v\n", err)
	}
	return 1
}

// hasCode reports whether the CLI maps err to a stable error code: the where
// sentinels that errorCode in internal/cli/error_codes.go turns into
// no_such_project and no_such_task. A refusal of the Personal project has no
// code there, so it keeps the JSON object.
func hasCode(err error) bool {
	return errors.Is(err, where.ErrProjectUnregistered) || errors.Is(err, where.ErrTaskNotFound)
}

// codeIndexRebuildFailed is the stable code of contract/README.md for a revert
// or supersede that wrote its change but could not rebuild index.md.
const codeIndexRebuildFailed = "index_rebuild_failed"

// partialCode returns the stable code of a written change whose index rebuild
// failed, matched from the brain sentinel, and "" for any other error.
func partialCode(err error) string {
	if errors.Is(err, brain.ErrIndexRebuild) {
		return codeIndexRebuildFailed
	}
	return ""
}

// partialFailure is the --json stderr object of a written change whose index
// rebuild failed: the store's message and the stable code.
func partialFailure(env Env, err error) int {
	obj := map[string]string{"error": err.Error()}
	if code := partialCode(err); code != "" {
		obj["code"] = code
	}
	_ = json.NewEncoder(env.Stderr).Encode(obj)
	return 1
}

// levelStore resolves the store and the level a verb acts on, the way Run does
// for propose and accept: an explicit --level, else the resolved project, else
// home. A write into a project needs its Grid record.
func levelStore(env Env, level, project string, writes bool) (brain.Store, string, error) {
	store := brain.Store{GridHome: env.GridHome, ReadLevel: level}
	if env.GridHome == "" {
		return store, "", fmt.Errorf("cannot find the Grid home; set GRID_HOME")
	}
	if level != "home" {
		res, err := env.Resolver.Resolve(where.Query{Cwd: env.Cwd, Project: project})
		if err != nil {
			return store, "", err
		}
		if res.Rule != where.RulePersonal && res.Project != where.Personal {
			store.Project = res.Project
			if writes && !res.Record && res.Rule != where.RuleTask {
				return store, "", fmt.Errorf("project %q has no Grid record; create its first task or use --level home", res.Project)
			}
		}
	}
	if level == "" {
		level = "home"
		if store.Project != "" {
			level = "project"
		}
	}
	if level == "project" && store.Project == "" {
		return store, "", fmt.Errorf("project level needs a project; select --project or use --level home")
	}
	return store, level, nil
}

// leveled parses the flags both verbs share. A non-zero code ends the command:
// 0 after help, 2 after a usage mistake.
type leveled struct {
	asJSON         bool
	project, level string
}

func (l *leveled) add(f *flag.FlagSet) {
	f.BoolVar(&l.asJSON, "json", false, "print JSON")
	f.StringVar(&l.project, "project", "", "project name or id")
	f.StringVar(&l.level, "level", "", "home or project")
}

func parseVerb(f *flag.FlagSet, args []string, env Env) (int, bool) {
	ordered, err := interspersed(f, args)
	if err != nil {
		fmt.Fprintf(env.Stderr, "brain: %v\n", err)
		return 2, true
	}
	if err := f.Parse(ordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, true
		}
		return 2, true
	}
	return 0, false
}

func (l *leveled) check(verb string, env Env) (int, bool) {
	msg := ""
	switch {
	case l.level != "" && l.level != "home" && l.level != "project":
		msg = "level must be home or project"
	case l.level == "home" && l.project != "":
		msg = "--level home cannot be combined with --project"
	}
	if msg == "" {
		return 0, false
	}
	fmt.Fprintf(env.Stderr, "brain %s: %s. Run: grid brain help\n", verb, msg)
	return 2, true
}

func runChanges(args []string, env Env) int {
	f := flag.NewFlagSet("grid brain changes", flag.ContinueOnError)
	f.SetOutput(env.Stderr)
	f.Usage = func() {
		fmt.Fprintln(env.Stdout, changesUsage)
		f.PrintDefaults()
	}
	var l leveled
	l.add(f)
	since := f.String("since", "", "only changes at or after this RFC 3339 time")
	limit := f.Int("limit", 0, "show at most n changes, newest first (0 or less: all)")
	if code, done := parseVerb(f, args, env); done {
		return code
	}
	usageError := func(msg string) int {
		fmt.Fprintf(env.Stderr, "brain changes: %s. Run: grid brain help\n", msg)
		return 2
	}
	if code, done := l.check("changes", env); done {
		return code
	}
	if f.NArg() != 0 {
		return usageError("unexpected positional argument")
	}
	var from time.Time
	if *since != "" {
		var err error
		if from, err = time.Parse(time.RFC3339, *since); err != nil {
			return usageError("--since must be an RFC 3339 time")
		}
	}
	if !brain.ChangesReadAllowed(env.Role) {
		return roleRefused(env, "changes", "read brain changes", l.asJSON)
	}
	store, level, err := levelStore(env, l.level, l.project, false)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	changes, err := store.Changes(level, from, *limit)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	if l.asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(struct {
			Changes []brain.Change `json:"changes"`
		}{changes}); err != nil {
			return failWith(env, l.asJSON, err)
		}
		return 0
	}
	for _, c := range changes {
		if _, err := fmt.Fprintln(env.Stdout, changeLine(c)); err != nil {
			return failWith(env, l.asJSON, err)
		}
	}
	return 0
}

// changeLine is at, status, topic/name, v<n>, hash, reason. The log can be
// edited by hand, so control characters never reach the terminal.
func changeLine(c brain.Change) string {
	id := c.Name
	if c.Topic != "" {
		id = c.Topic + "/" + c.Name
	}
	hash := c.Hash
	if hash == "" {
		hash = "-"
	}
	fields := []string{c.At, c.Status, id, fmt.Sprintf("v%d", c.Version), hash}
	if c.Reason != "" {
		fields = append(fields, c.Reason)
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '?'
		}
		return r
	}, strings.Join(fields, " "))
}

func runRevert(args []string, env Env) int {
	f := flag.NewFlagSet("grid brain revert", flag.ContinueOnError)
	f.SetOutput(env.Stderr)
	f.Usage = func() {
		fmt.Fprintln(env.Stdout, revertUsage)
		f.PrintDefaults()
	}
	var l leveled
	l.add(f)
	to := f.Int("to", 0, "version to publish again (1 or more)")
	reason := f.String("reason", "", "why the entry is put back; logged as \"revert to v<n>: <why>\"")
	if code, done := parseVerb(f, args, env); done {
		return code
	}
	usageError := func(msg string) int {
		fmt.Fprintf(env.Stderr, "brain revert:%s. Run: grid brain help\n", msg)
		return 2
	}
	if code, done := l.check("revert", env); done {
		return code
	}
	if f.NArg() != 1 {
		return usageError("expected <topic>/<name>")
	}
	topic, name, ok := strings.Cut(f.Arg(0), "/")
	if !ok || !brain.ValidName(topic) || !brain.ValidName(name) {
		return usageError("expected <topic>/<name>")
	}
	if *to < 1 {
		return usageError("--to must be a version number, 1 or more")
	}
	if !brain.RevertAllowed(env.Role) {
		return roleRefused(env, "revert", "revert brain entries", l.asJSON)
	}
	store, level, err := levelStore(env, l.level, l.project, true)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	origin, err := hyDrOrigin(env)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	// The store says whether it wrote, under the level lock; noop means it did not.
	why := strings.Join(strings.Fields(*reason), " ")
	res, err := store.RevertWith(level, topic, name, *to, origin, brain.RevertOptions{Reason: why})
	if err != nil && !res.Wrote {
		return failWith(env, l.asJSON, err)
	}
	// err with Wrote set means the version is stored and logged but the index
	// rebuild failed: report the version, then the error, and exit 1.
	version, noop := res.Version, !res.Wrote
	if l.asJSON {
		out := struct {
			Version int    `json:"version"`
			Noop    bool   `json:"noop"`
			Reason  string `json:"reason,omitempty"`
			Error   string `json:"error,omitempty"`
			Code    string `json:"code,omitempty"`
		}{Version: version, Noop: noop, Reason: why}
		if err != nil {
			out.Error, out.Code = err.Error(), partialCode(err)
		}
		if encErr := json.NewEncoder(env.Stdout).Encode(out); encErr != nil {
			return failWith(env, l.asJSON, errors.Join(encErr, err))
		}
		if err != nil {
			return partialFailure(env, err)
		}
		return 0
	}
	if err != nil {
		if _, perr := fmt.Fprintf(env.Stdout, "v%d\n", version); perr != nil {
			return failWith(env, l.asJSON, errors.Join(perr, err))
		}
		return failWith(env, l.asJSON, err)
	}
	text := fmt.Sprintf("v%d", version)
	if noop {
		text = fmt.Sprintf("already at v%d", version)
	} else if why != "" {
		text += " (reason: " + why + ")"
	}
	if _, err := fmt.Fprintln(env.Stdout, text); err != nil {
		return failWith(env, l.asJSON, err)
	}
	return 0
}
