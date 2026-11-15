package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

const supersedeUsage = `Usage: grid brain supersede <old-note> --by <new-note> --reason <why> [--level home|project] [--project <p>] [--json]
  Mark an accepted note replaced by another accepted note after the fact. The old
  note gets superseded_by and the new note a supersedes link, each as a new
  version with one audit line. Nothing is deleted; the old note leaves search and
  the index and stays readable with show. Both notes must sit at one level.
  --reason is 8 to 160 characters on one line, in more than one word.
  Exit 0 done or already done, 1 refused or store error, 2 usage, 3 role refused.`

func runSupersede(args []string, env Env) int {
	f := flag.NewFlagSet("grid brain supersede", flag.ContinueOnError)
	f.SetOutput(helpOut(args, env))
	f.Usage = func() {
		fmt.Fprintln(f.Output(), supersedeUsage)
		f.PrintDefaults()
	}
	var l leveled
	l.add(f)
	by := f.String("by", "", "the note that replaces it")
	reason := f.String("reason", "", "why, 8 to 160 characters")
	if code, done := parseVerb(f, args, env); done {
		return code
	}
	usageError := func(msg string) int {
		fmt.Fprintf(env.Stderr, "brain supersede: %s. Run: grid brain help\n", msg)
		return 2
	}
	if code, done := l.check("supersede", env); done {
		return code
	}
	if f.NArg() != 1 {
		return usageError("expected one <old-note> id")
	}
	given := map[string]bool{}
	f.Visit(func(fl *flag.Flag) { given[fl.Name] = true })
	if !given["by"] {
		return usageError("--by <new-note> is required")
	}
	if !given["reason"] {
		return usageError("--reason <why> is required")
	}
	for _, id := range []string{f.Arg(0), *by} {
		if rest, ok := strings.CutPrefix(id, "brain:"); ok {
			if level, _, _ := strings.Cut(rest, "/"); l.level != "" && level != l.level {
				return usageError("id level disagrees with --level")
			}
		}
	}
	if !brain.CurateAllowed(env.Role) {
		return roleRefused(env, "supersede", "supersede brain entries", l.asJSON)
	}
	store, _, err := levelStore(env, l.level, l.project, true)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	origin, err := hyDrOrigin(env)
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	// The store decides under the level lock and says whether it wrote.
	res, err := store.Supersede(f.Arg(0), *by, *reason, origin)
	if err != nil && !res.Wrote {
		return failWith(env, l.asJSON, err)
	}
	// err with Wrote set means both files are written and logged but the index
	// rebuild failed: report the result, then the error, and exit 1.
	if l.asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		if encErr := enc.Encode(res); encErr != nil {
			return failWith(env, l.asJSON, errors.Join(encErr, err))
		}
		if err != nil {
			return partialFailure(env, err)
		}
		return 0
	}
	text := fmt.Sprintf("%s superseded by %s (v%d, v%d)", res.Old, res.New, res.Versions.Old, res.Versions.New)
	if !res.Wrote {
		text = fmt.Sprintf("already superseded: %s by %s (v%d, v%d)", res.Old, res.New, res.Versions.Old, res.Versions.New)
	}
	if _, perr := fmt.Fprintln(env.Stdout, text); perr != nil {
		return failWith(env, l.asJSON, errors.Join(perr, err))
	}
	if err != nil {
		return failWith(env, l.asJSON, err)
	}
	return 0
}
