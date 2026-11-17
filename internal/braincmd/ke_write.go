package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// writeUsage is the usage line of `grid brain write`, for the verb list the
// mount wires.
const writeUsage = "Usage: grid brain write <topic>/<name> --expect <16hex|none> --file <path|-> [--reason <why>] [--level home|project] [--project <p>] [--json]"

// keWrOrder moves the flags before the operands, as interspersed does, but lets
// an unknown flag through so that flag.Parse names it ("flag provided but not
// defined: -by"). A flag that needs a value and has none is left for Parse too.
func keWrOrder(f *flag.FlagSet, args []string) []string {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			continue
		}
		options = append(options, a)
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		fl := f.Lookup(name)
		if fl == nil || hasValue {
			continue
		}
		if b, ok := fl.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	return append(append(options, "--"), positional...)
}

// runWrite is `grid brain write <topic>/<name> --expect <rev|none> --file <path|->`,
// in the order of hyDrRun: flags and operand, role, input, store, then the write
// under the brain lock. Exits: 0 done, 1 store error, 2 usage, a bad note or a
// linked target, 3 role refused (the verb), 6 stale rev, 7
// not versioned, 9 refused (the name is held by another topic). Never 4 or 5.
// No flag names the actor: the audit line names env.Member, else the session.
func runWrite(args []string, env Env) int {
	f := flag.NewFlagSet("grid brain write", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	var l leveled
	l.add(f)
	expect := f.String("expect", "", "the note's current rev, 16 lower-case hex digits, or none for a new note")
	file := f.String("file", "", "the note's Markdown, a path or - for stdin")
	reason := f.String("reason", "", "why; logged with the audit line")
	usageError := func(msg string) int {
		fmt.Fprintf(env.Stderr, "grid brain write: %s\n%s\n", msg, writeUsage)
		return 2
	}
	if err := f.Parse(keWrOrder(f, args)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(env.Stdout, writeUsage)
			return 0
		}
		return usageError(err.Error())
	}
	if code, done := l.check("write", env); done {
		return code
	}
	if f.NArg() != 1 {
		return usageError("expected <topic>/<name>")
	}
	topic, name, ok := strings.Cut(f.Arg(0), "/")
	if !ok || !brain.ValidID(topic) || !brain.ValidID(name) || strings.Contains(name, "/") {
		return usageError("expected <topic>/<name>")
	}
	if *expect != "none" && !hyDrRev(*expect) {
		return usageError("--expect must be the note's rev, 16 lower-case hex digits, or none")
	}
	if *file == "" {
		return usageError("--file is required: a path, or - for stdin")
	}
	if !brain.RevertAllowed(env.Role) {
		return roleRefused(env, "write", "write brain entries", l.asJSON)
	}
	// The store comes before the input, so a project that is not registered answers
	// no_such_project (exit 1) whatever --file holds or whether stdin is empty.
	store, level, err := levelStore(env, l.level, l.project, true)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	var data []byte
	if *file == "-" {
		data, err = readInput(env.Stdin)
	} else {
		var in *os.File
		if in, err = os.Open(*file); err == nil {
			data, err = readInput(in)
			in.Close()
		}
	}
	if err != nil {
		return usageError(err.Error())
	}
	n, err := brain.Parse(data)
	if err == nil {
		err = n.Validate()
	}
	if err != nil {
		return usageError(err.Error())
	}
	origin, err := hyDrOrigin(env)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	why := strings.Join(strings.Fields(*reason), " ")
	// Anyone the verb lets in writes a workflow note too (owner answer 3, 11 Nov);
	// the author is in the audit line.
	res, err := store.Write(level, topic, name, *expect, data, origin, why, true)
	if err != nil && res.Status == "" {
		var conflict *brain.ConflictError
		var refused *brain.RefusedError
		switch {
		case errors.As(err, &conflict):
			return hyDrFail(env, 6, hyDrFailure{Error: "conflict", Rev: conflict.Rev})
		case errors.Is(err, brain.ErrNotVersioned):
			return hyDrFail(env, 7, hyDrFailure{Error: "not_versioned"})
		case errors.As(err, &refused):
			return hyDrFail(env, 9, hyDrFailure{Error: "refused", Reason: refused.Reason})
		case errors.Is(err, brain.ErrInvalidNote), errors.Is(err, brain.ErrLinkedTarget):
			return usageError(err.Error())
		}
		return failWith(env, l.asJSON, err)
	}
	// err with a status means the note and its line are written and only the index
	// rebuild failed: report the result, then the error, and exit 1, as revert does.
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
		return failWith(env, l.asJSON, fmt.Errorf("write: %w", err))
	}
	return 0
}
