package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/updates"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// updateLines collects repeated --line values.
type updateLines []string

func (l *updateLines) String() string { return strings.Join(*l, "\n") }
func (l *updateLines) Set(v string) error {
	*l = append(*l, v)
	return nil
}

// runUpdate acts on a project's update file, as the app's Updates tab does
// through update_act: draft, edit, discard, skip or send. Send marks the
// update sent in the Grid home; it mails nothing.
func runUpdate(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: grid update draft|edit|discard|skip|send <project>")
		return 2
	}
	verb := args[0]
	fs := newFlags("update "+verb, env)
	asJSON := fs.Bool("json", false, "print JSON")
	var title string
	var raw updateLines
	switch verb {
	case "draft":
		fs.StringVar(&title, "title", "", "the update's title (default \"Update <today>\")")
	case "edit":
		fs.Var(&raw, "line", "a line \"<task> · <text>\"; repeatable, replaces the draft's lines")
	case "discard", "skip", "send":
	default:
		fmt.Fprintf(env.Stderr, "grid update: unknown command %q\n", verb)
		return 2
	}
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintf(env.Stderr, "grid update %s: wrong number of arguments\n", verb)
		return 2
	}
	var lines []updates.Line
	if verb == "edit" {
		if len(raw) == 0 {
			fmt.Fprintln(env.Stderr, "grid update edit: give the lines with --line \"<task> · <text>\"")
			return 2
		}
		if len(raw) > updates.MaxLines {
			fmt.Fprintf(env.Stderr, "grid update edit: an update holds at most %d lines\n", updates.MaxLines)
			return 2
		}
		for _, s := range raw {
			line, ok := updates.ParseLine(s)
			if !ok {
				fmt.Fprintf(env.Stderr, "grid update edit: line %q is not \"<task> · <text>\"\n", s)
				return 2
			}
			lines = append(lines, line)
		}
	}
	if verb == "draft" && title != "" {
		if err := updates.CheckTitle(title); err != nil {
			fmt.Fprintf(env.Stderr, "grid update draft: %v\n", err)
			return 2
		}
	}
	r := resolver(env)
	if r.Caller.IsGuest() {
		// Resolve first, so an unknown and an uninvited project answer alike
		// (guest_parity_test.go); an invited one is refused. Nothing is written.
		if _, err := r.Resolve(where.Query{Cwd: env.Cwd, Project: fs.Arg(0)}); err != nil {
			return commandError(env, "update "+verb, err)
		}
		return failure(env, errors.New(guestSettingsRefusal))
	}
	res, err := updates.Do(r, updates.Act{Act: verb, Project: fs.Arg(0), Cwd: env.Cwd, Title: title, Lines: lines})
	if err != nil {
		return commandError(env, "update "+verb, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, res)
	}
	status := "discarded"
	for _, u := range res.Updates {
		if u.K+".md" == res.Name {
			status = u.Status
		}
	}
	fmt.Fprintf(env.Stdout, "update: %s\nstatus: %s\n", res.Name, status)
	return 0
}
