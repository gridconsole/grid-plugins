package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

const renameUsage = "Usage: grid brain rename <topic>/<name> <new-name> --expect <16hex> [--reason <why>] [--level home|project] [--project <p>] [--json]"

// runRename is grid brain rename, in the order of hyDrRun: parse, usage, role,
// store, then the rename under the brain lock. Exits: 0 done (the notes whose
// relink failed are printed as kept and do not change the exit), 1 store error,
// 2 usage, 3 role refused, 6 stale rev, 7 not versioned, 8 no such note, 9
// refused by the note's state. No flag names the person: the audit lines carry
// env.Member, else the session.
func runRename(args []string, env Env) int {
	f := flag.NewFlagSet("grid brain rename", flag.ContinueOnError)
	f.SetOutput(env.Stderr)
	f.Usage = func() {
		fmt.Fprintln(env.Stdout, renameUsage)
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
		fmt.Fprintf(env.Stderr, "brain rename: %s\n%s\n", msg, renameUsage)
		return 2
	}
	if code, done := l.check("rename", env); done {
		return code
	}
	if f.NArg() != 2 {
		return usageError("expected <topic>/<name> <new-name>")
	}
	topic, name, ok := strings.Cut(f.Arg(0), "/")
	if !ok || !brain.ValidName(topic) || !brain.ValidName(name) {
		return usageError("expected <topic>/<name> <new-name>")
	}
	newName := f.Arg(1)
	if !brain.ValidName(newName) {
		return usageError("the new name must be a plain name")
	}
	if !hyDrRev(*expect) {
		return usageError("--expect must be the entry's rev, 16 lower-case hex digits")
	}
	if !brain.RevertAllowed(env.Role) {
		return roleRefused(env, "rename", "rename brain entries", l.asJSON)
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
	res, err := store.Rename(level, topic, name, newName, *expect, origin, why)
	if err != nil && res.Status == "" {
		var conflict *brain.ConflictError
		var refused *brain.RefusedError
		switch {
		case errors.As(err, &conflict):
			return hyDrFail(env, 6, hyDrFailure{Error: "conflict", Rev: conflict.Rev})
		case errors.Is(err, brain.ErrRenameNotVersioned):
			return hyDrFail(env, 7, hyDrFailure{Error: "not_versioned", Reason: err.Error()})
		case errors.Is(err, brain.ErrNoSuchNote):
			return hyDrFail(env, 8, hyDrFailure{Error: "no_such_note"})
		case errors.As(err, &refused):
			return hyDrFail(env, 9, hyDrFailure{Error: "refused", Reason: refused.Reason})
		}
		return failWith(env, l.asJSON, err)
	}
	// err with a status means the rename is applied and a step after it failed:
	// report the result, then the error, and exit 1, as drop does.
	if l.asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		if encErr := enc.Encode(res); encErr != nil {
			return failWith(env, l.asJSON, errors.Join(encErr, err))
		}
	} else if perr := keRnPrint(env, res); perr != nil {
		return failWith(env, l.asJSON, errors.Join(perr, err))
	}
	if err != nil {
		return failWith(env, l.asJSON, fmt.Errorf("rename: %w", err))
	}
	return 0
}

// keRnPrint prints the plain result: the rename, then one line for each note
// that was linked, kept as it was, or skipped because it is dropped.
func keRnPrint(env Env, res brain.RenameResult) error {
	if _, err := fmt.Fprintf(env.Stdout, "%s %s -> %s v%d rev %s\n", res.Status, res.From, res.To, res.Version, res.Rev); err != nil {
		return err
	}
	for _, group := range []struct {
		word  string
		names []string
	}{{"linked", res.Linked}, {"kept", res.Kept}, {"skipped", res.Skipped}} {
		for _, n := range group.names {
			if _, err := fmt.Fprintf(env.Stdout, "%s %s\n", group.word, n); err != nil {
				return err
			}
		}
	}
	return nil
}
