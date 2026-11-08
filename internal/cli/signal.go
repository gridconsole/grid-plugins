package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"

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
		return 2
	}
	if (verb == "add" && fs.NArg() != 0) || (verb != "add" && fs.NArg() != 1) {
		fmt.Fprintf(env.Stderr, "grid signal %s: wrong number of arguments\n", verb)
		return 2
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
	case "act":
		got, err = s.Act(fs.Arg(0))
	case "dismiss":
		got, err = s.Dismiss(fs.Arg(0))
	}
	if err != nil {
		return failure(env, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, got)
	}
	fmt.Fprintf(env.Stdout, "result: %s\nid: %s\nproject: %s\nnote: %s\nstatus: %s\n", got.Result, got.ID, got.Project, got.Note, got.Status)
	return 0
}
