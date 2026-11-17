package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

const (
	dropUsage    = "Usage: grid brain drop <topic>/<name> --expect <16hex> [--reason <why>] [--level home|project] [--project <p>] [--json]"
	restoreUsage = "Usage: grid brain restore <topic>/<name> --expect <16hex> [--reason <why>] [--level home|project] [--project <p>] [--json]"
)

func runDrop(args []string, env Env) int { return hyDrRun(brain.OpDrop, args, env) }

func runRestore(args []string, env Env) int { return hyDrRun(brain.OpRestore, args, env) }

// hyDrFailure is the JSON a refused drop or restore prints on stderr: the stale
// rev carries the current rev, a refusal carries the reason.
type hyDrFailure struct {
	Error  string `json:"error"`
	Rev    string `json:"rev,omitempty"`
	Reason string `json:"reason,omitempty"`
}

func hyDrFail(env Env, code int, f hyDrFailure) int {
	enc := json.NewEncoder(env.Stderr)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(f)
	return code
}

// hyDrRev reports whether s is a rev as the app writes it: 16 lower-case hex.
func hyDrRev(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// hyDrOrigin names the person the host runs the plugin for, else the session, as
// revert does. No flag reaches it.
func hyDrOrigin(env Env) (brain.Origin, error) {
	if brain.ValidName(env.Member) {
		return brain.Origin{Kind: "person", ID: env.Member}, nil
	}
	return brain.SessionOrigin()
}

// hyDrRun is drop and restore, in the order of runRevert: parse, usage, role,
// store, then the write under the brain lock. Exits: 0 done, 1 store error, 2
// usage, 3 role refused, 6 stale rev, 8 no such note, 9 refused by the note's
// state. Never 4, 5 or 7.
func hyDrRun(op string, args []string, env Env) int {
	usage, past := dropUsage, "drop"
	if op == brain.OpRestore {
		usage, past = restoreUsage, "restore"
	}
	f := flag.NewFlagSet("grid brain "+op, flag.ContinueOnError)
	f.SetOutput(helpOut(args, env))
	f.Usage = func() {
		fmt.Fprintln(f.Output(), usage)
		f.PrintDefaults()
	}
	var l leveled
	l.add(f)
	expect := f.String("expect", "", "the entry's current rev, 16 lower-case hex digits")
	reason := f.String("reason", "", "why; logged with the audit line")
	if code, done := parseVerb(f, args, env); done {
		return code
	}
	usageError := func(msg string) int {
		fmt.Fprintf(env.Stderr, "brain %s: %s\n%s\n", op, msg, usage)
		return 2
	}
	if code, done := l.check(op, env); done {
		return code
	}
	if f.NArg() != 1 {
		return usageError("expected <topic>/<name>")
	}
	topic, name, ok := strings.Cut(f.Arg(0), "/")
	if !ok || !brain.ValidID(topic) || !brain.ValidID(name) || strings.Contains(name, "/") {
		return usageError("expected <topic>/<name>")
	}
	if !hyDrRev(*expect) {
		return usageError("--expect must be the entry's rev, 16 lower-case hex digits")
	}
	if !brain.RevertAllowed(env.Role) {
		return roleRefused(env, op, op+" brain entries", l.asJSON)
	}
	store, level, err := levelStore(env, l.level, l.project, true)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	origin, err := hyDrOrigin(env)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	why := strings.Join(strings.Fields(*reason), " ")
	res, err := store.Drop(level, topic, name, op, *expect, origin, why)
	if err != nil && res.Status == "" {
		var conflict *brain.ConflictError
		var refused *brain.RefusedError
		switch {
		case errors.As(err, &conflict):
			return hyDrFail(env, 6, hyDrFailure{Error: "conflict", Rev: conflict.Rev})
		case errors.Is(err, brain.ErrNoSuchNote):
			return hyDrFail(env, 8, hyDrFailure{Error: "no_such_note"})
		case errors.As(err, &refused):
			return hyDrFail(env, 9, hyDrFailure{Error: "refused", Reason: refused.Reason})
		}
		return failWith(env, l.asJSON, err)
	}
	// err with a status means the line is written and only the index rebuild
	// failed: report the result, then the error, and exit 1, as revert does.
	if l.asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		if encErr := enc.Encode(res); encErr != nil {
			return failWith(env, l.asJSON, errors.Join(encErr, err))
		}
	} else if _, perr := fmt.Fprintf(env.Stdout, "%s %s/%s v%d rev %s\n", res.Status, topic, name, res.Version, res.Rev); perr != nil {
		return failWith(env, l.asJSON, errors.Join(perr, err))
	}
	if err != nil {
		return failWith(env, l.asJSON, fmt.Errorf("%s: %w", past, err))
	}
	return 0
}
