package cli

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/projects"
)

// parsePositionals allows flags before or after the documented positional args.
func parsePositionals(fs *flag.FlagSet, args []string) error {
	flags, positional := []string{}, []string{}
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
		flags = append(flags, a)
		name, _, equal := strings.Cut(strings.TrimLeft(a, "-"), "=")
		f := fs.Lookup(name)
		if f == nil || equal {
			continue
		}
		isBool := false
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
			isBool = b.IsBoolFlag()
		}
		if !isBool {
			if i+1 == len(args) {
				return fs.Parse(flags)
			}
			i++
			flags = append(flags, args[i])
		}
	}
	return fs.Parse(append(append(flags, "--"), positional...))
}
func failure(env Env, err error) int {
	env.recordError(err)
	fmt.Fprintf(env.Stderr, "grid: %v\n", err)
	return 1
}
func pathFrom(env Env, path string) string {
	if path == "" {
		return env.Cwd
	}
	if !filepath.IsAbs(path) {
		return filepath.Join(env.Cwd, path)
	}
	return path
}
func projectStore(env Env) projects.Store {
	r := resolver(env)
	if machine := gridhome.Machine(env.GridHome); machine != "" {
		r.Machine = machine
	}
	return projects.Store{Resolver: r}
}
func runProject(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: grid project add|remove|list|scan|relink|repo|env")
		return 2
	}
	verb := args[0]
	switch verb {
	case "repo":
		return projectRepo(args[1:], env)
	case "env":
		return projectEnv(args[1:], env)
	}
	fs := newFlags("project "+verb, env)
	asJSON := fs.Bool("json", false, "print JSON")
	var name string
	var apply bool
	switch verb {
	case "add":
		fs.StringVar(&name, "name", "", "project name")
	case "scan":
		fs.BoolVar(&apply, "apply", false, "register the discovered projects")
	case "list", "remove", "relink":
	default:
		fmt.Fprintf(env.Stderr, "grid project: unknown command %q\n", verb)
		return 2
	}
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	count := fs.NArg()
	valid := (verb == "add" && count <= 1) || (verb == "list" && count == 0) || ((verb == "scan" || verb == "remove") && count == 1) || (verb == "relink" && count == 2)
	if !valid {
		fmt.Fprintf(env.Stderr, "grid project %s: wrong number of arguments\n", verb)
		return 2
	}
	if (verb == "add" || verb == "remove" || verb == "relink" || verb == "scan" && apply) && resolver(env).Caller.IsGuest() {
		return failure(env, ilGuestRefused)
	}
	s := projectStore(env)
	if verb == "list" {
		entries, err := s.List()
		if err != nil {
			return failure(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, entries)
		}
		for _, e := range entries {
			fmt.Fprintf(env.Stdout, "project: %s\nid: %s\n", e.Name, e.ID)
			for _, f := range e.Folders {
				fmt.Fprintf(env.Stdout, "folder: %s\nmachine: %s\n", gridhome.Tilde(f.Path, env.Home), f.Machine)
			}
		}
		return 0
	}
	if verb == "scan" {
		proposals, err := s.Scan(pathFrom(env, fs.Arg(0)))
		if err != nil {
			return failure(env, err)
		}
		if apply {
			proposals, err = s.Apply(proposals)
			if err != nil {
				return failure(env, err)
			}
		}
		failed := 0
		for _, p := range proposals {
			if p.Error != "" {
				failed++
			}
		}
		code := 0
		if *asJSON {
			code = writeJSON(env.Stdout, proposals)
		} else {
			for _, p := range proposals {
				fmt.Fprintf(env.Stdout, "project: %s\npath: %s\nrule: %s\nregistered: %t\n", p.Name, gridhome.Tilde(p.Path, env.Home), p.Rule, p.Registered)
				if p.Error != "" {
					fmt.Fprintf(env.Stdout, "error: %s\n", p.Error)
				}
			}
		}
		if failed > 0 {
			fmt.Fprintf(env.Stderr, "grid: %d of %d projects were not registered\n", failed, len(proposals))
			return 1
		}
		return code
	}
	var e projects.Entry
	var err error
	switch verb {
	case "add":
		e, err = s.Add(pathFrom(env, fs.Arg(0)), name)
	case "remove":
		e, err = s.Remove(fs.Arg(0))
	case "relink":
		e, err = s.Relink(fs.Arg(0), pathFrom(env, fs.Arg(1)))
	}
	if err != nil {
		return failure(env, err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, e)
	}
	fmt.Fprintf(env.Stdout, "project: %s\nid: %s\n", e.Name, e.ID)
	return 0
}
