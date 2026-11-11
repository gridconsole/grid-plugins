package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/job"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/jobgraph"
)

func runJobGraph(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid job cycles <items.json> [--json]")
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	verb := args[0]
	if verb != "cycles" {
		fmt.Fprintf(env.Stderr, "grid job: unknown or unbuilt command %q\n", verb)
		return 2
	}
	fs := newFlags("job "+verb, env)
	asJSON := fs.Bool("json", false, "print JSON")
	// flag stops at a positional argument; collect options separately so the
	// documented trailing --json and ordinary leading options both work.
	options, files := []string{}, []string{}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(arg, "-") {
			options = append(options, arg)
		} else {
			files = append(files, arg)
		}
	}
	if err := fs.Parse(options); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(files) != 1 || fs.NArg() != 0 {
		fmt.Fprintf(env.Stderr, "Usage: grid job %s <items.json> [--json]\n", verb)
		return 2
	}
	var items []job.Job
	if err := readJobJSON(env, files[0], &items); err != nil {
		fmt.Fprintf(env.Stderr, "grid job %s: %v\n", verb, err)
		return 1
	}
	if items == nil {
		fmt.Fprintln(env.Stderr, "grid job: items must be a JSON list, not null")
		return 1
	}
	return runJobCycles(items, *asJSON, env)
}

func runJobCycles(items []job.Job, asJSON bool, env Env) int {
	g, err := jobgraph.New(items)
	if err != nil {
		fmt.Fprintf(env.Stderr, "grid job cycles: %v\n", err)
		return 1
	}
	cycle := g.Cycle()
	code := 0
	if len(cycle) > 0 {
		code = 1
	}
	if asJSON {
		if writeJSON(env.Stdout, struct {
			OK    bool     `json:"ok"`
			Cycle []string `json:"cycle"`
		}{code == 0, cycle}) != 0 {
			return 1
		}
	} else if code == 0 {
		fmt.Fprintln(env.Stdout, "No Job cycles.")
	} else {
		fmt.Fprintln(env.Stdout, (&jobgraph.CycleError{Path: cycle}).Error())
	}
	return code
}

func readJobJSON(env Env, name string, value any) error {
	filename := name
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(env.Cwd, filename)
	}
	f, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return fmt.Errorf("decode %s: %w", name, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return fmt.Errorf("decode %s: %w", name, err)
	}
	return nil
}
