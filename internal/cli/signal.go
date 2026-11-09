package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runSignal(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: grid signal add|act|dismiss")
		return 2
	}
	verb := args[0]
	if verb != "add" && verb != "act" && verb != "dismiss" {
		fmt.Fprintf(env.Stderr, "grid signal: unknown command %q\n", verb)
		return 2
	}
	fs := newFlags("signal "+verb, env)
	asJSON := fs.Bool("json", false, "print JSON")
	proj := fs.String("project", "", "project name or id; defaults to this folder's project")
	var in signal.Input
	var act signal.ActOptions
	if verb == "act" {
		fs.StringVar(&act.Act, "act", "note", "turn|add|note|snooze|dismiss|wake|reopen")
		fs.StringVar(&act.Task, "task", "", "task id for add (T-<n>)")
		fs.StringVar(&act.Until, "until", "", "snooze until RFC3339 or YYYY-MM-DDTHH:MM")
		fs.BoolVar(&act.Later, "later", false, "reserved; currently refused")
	}
	if verb == "add" {
		fs.StringVar(&in.Source, "source", "typed", "source name")
		fs.StringVar(&in.SourceID, "source-id", "", "stable id in this source for deduplication")
		fs.StringVar(&in.Title, "title", "", "signal title")
		fs.StringVar(&in.Body, "body", "", "external text, or - to read stdin")
	}
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		if verb != "add" {
			signalErrorJSON(env, signalJSONRequested(fs, args[1:]), "invalid", "invalid signal arguments")
		}
		return 2
	}
	if (verb == "add" && fs.NArg() != 0) || (verb != "add" && fs.NArg() != 1) {
		fmt.Fprintf(env.Stderr, "grid signal %s: wrong number of arguments\n", verb)
		if verb != "add" {
			signalErrorJSON(env, *asJSON, "invalid", "wrong number of arguments")
		}
		return 2
	}
	if resolver(env).Caller.IsGuest() {
		if verb == "add" {
			return failure(env, fmt.Errorf("guests cannot change project settings"))
		}
		return signalFailure(env, &signal.RefusedError{Message: "guests cannot act on signals"}, *asJSON)
	}
	if verb != "add" {
		if verb == "dismiss" {
			act.Act = "dismiss"
		}
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "task":
				act.TaskSet = true
			case "until":
				act.UntilSet = true
			case "later":
				act.Later = true
			}
		})
		if err := act.Validate(fs.Arg(0)); err != nil {
			return signalFailure(env, err, *asJSON)
		}
		act.Cwd, act.Who, act.Role = env.Cwd, env.getenv("GRID_WHO"), env.getenv("GRID_ROLE")
		if act.Who == "" {
			act.Who = env.getenv("GRID_ACTOR")
		}
	}
	if in.Body == "-" {
		data, err := io.ReadAll(io.LimitReader(env.Stdin, signal.MaxBytes+1))
		if err != nil {
			return failure(env, err)
		}
		if len(data) > signal.MaxBytes {
			return failure(env, fmt.Errorf("signal body exceeds %d bytes", signal.MaxBytes))
		}
		in.Body = string(data)
	}
	if verb == "add" {
		if err := signal.Validate(in); err != nil {
			return failure(env, err)
		}
	}
	ps := projectStore(env)
	res, err := ps.Resolver.Resolve(where.Query{Cwd: env.Cwd, Project: *proj})
	if err != nil {
		return failure(env, err)
	}
	if !res.Record && res.Rule == where.RulePersonal {
		entries, err := ps.List()
		if err != nil {
			return failure(env, err)
		}
		for _, e := range entries {
			if e.Name == where.Personal {
				res.Record = true
			}
		}
	}
	if !res.Record {
		if res.Rule != where.RulePersonal {
			return failure(env, fmt.Errorf("project is not registered; run grid project add"))
		}
		if verb != "add" {
			return failure(env, fmt.Errorf("Personal has no signals yet"))
		}
		e, err := ps.Add(env.Cwd, "")
		if err != nil {
			return failure(env, err)
		}
		res.Project = e.Name
	}
	s := signal.Store{GridHome: env.GridHome, Project: res.Project, Resolver: projectStore(env).Resolver}
	var got signal.Result
	switch verb {
	case "add":
		got, err = s.Add(in)
	case "act", "dismiss":
		got, err = s.Apply(fs.Arg(0), act)
	}
	if err != nil {
		return signalFailure(env, err, *asJSON)
	}
	if *asJSON {
		return writeJSON(env.Stdout, got)
	}
	fmt.Fprintf(env.Stdout, "result: %s\nid: %s\nproject: %s\nnote: %s\nstatus: %s\n", got.Result, got.ID, got.Project, got.Note, got.Status)
	return 0
}

// signalFailure preserves the existing stderr prefix while classifying act errors.
func signalFailure(env Env, err error, asJSON bool) int {
	code := failure(env, err)
	var bad *signal.BadInputError
	var refused *signal.RefusedError
	switch {
	case errors.As(err, &bad):
		message := bad.Message
		// An unknown act can contain caller-supplied paths; keep those on stderr.
		if strings.HasPrefix(message, "unknown signal act ") && strings.ContainsAny(message, `/\`) {
			message = "unknown signal act"
		}
		signalErrorJSON(env, asJSON, "invalid", message)
		return 2
	case errors.As(err, &refused):
		signalErrorJSON(env, asJSON, "refused", refused.Message)
		return 3
	default:
		return code
	}
}

func signalErrorJSON(env Env, asJSON bool, kind, message string) {
	if asJSON {
		writeJSON(env.Stdout, struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}{kind, message})
	}
}

// Parsing can stop before --json; inspect flags without treating option values
// or arguments after -- as flags, and retain the last valid JSON setting.
func signalJSONRequested(fs *flag.FlagSet, args []string) bool {
	asJSON := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if !strings.HasPrefix(arg, "-") {
			continue
		}
		name, value, equal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := fs.Lookup(name)
		if f == nil {
			continue
		}
		if name == "json" {
			if !equal {
				value = "true"
			}
			if enabled, err := strconv.ParseBool(value); err == nil {
				asJSON = enabled
			}
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); (!ok || !b.IsBoolFlag()) && !equal {
			i++
		}
	}
	return asJSON
}
