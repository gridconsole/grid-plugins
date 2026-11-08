package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func runDecide(args []string, env Env) int {
	f := flag.NewFlagSet("grid brain decide", flag.ContinueOnError)
	f.SetOutput(env.Stderr)
	f.Usage = func() {
		fmt.Fprintln(env.Stdout, "Usage: grid brain decide <task> <proposal-id> accept|reject|merge-into [<note-id>] [--note <reason>] [--json]")
		fmt.Fprintln(env.Stdout, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		f.PrintDefaults()
	}
	note := f.String("note", "", "reason for the decision")
	asJSON := f.Bool("json", false, "print JSON")
	ordered, err := interspersed(f, args)
	if err != nil {
		fmt.Fprintln(env.Stderr, err)
		return 2
	}
	if err = f.Parse(ordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if f.NArg() < 3 || f.NArg() > 4 || (f.Arg(2) == "merge-into" && f.NArg() != 4) {
		fmt.Fprintln(env.Stderr, "Usage: grid brain decide <task> <proposal-id> accept|reject|merge-into [<note-id>] [--note <reason>] [--json]")
		return 2
	}
	action := f.Arg(2)
	if action != "accept" && action != "reject" && action != "merge-into" {
		fmt.Fprintln(env.Stderr, "brain decide: invalid decision")
		return 2
	}
	fail := func(err error) int { fmt.Fprintf(env.Stderr, "brain decide: %v\n", err); return 1 }
	t, err := env.ResolveTask(f.Arg(0))
	if err != nil {
		return env.TaskOperandError("brain decide", err)
	}
	s := brain.Store{GridHome: env.GridHome, Project: t.Project}
	if t.Project == gridhome.Personal {
		s.Project = ""
	}
	target := f.Arg(3)
	if target == "" {
		target = f.Arg(1)
	}
	tasks := task.Store{Resolver: env.Resolver}
	history := func(text string) error {
		_, err := tasks.Note(task.WriteOptions{Cwd: env.Cwd, ID: t.ID, Note: text, Who: contract.Author{ID: "grid-curator", Role: "agent"}})
		return err
	}
	result, err := s.DecideTask(brain.TaskRef{ID: t.ID, Path: t.Path}, f.Arg(1), action, target, *note, history)
	if err != nil {
		return fail(err)
	}
	if *asJSON {
		err = json.NewEncoder(env.Stdout).Encode(result)
	} else {
		err = plain(env.Stdout, result)
	}
	if err != nil {
		return fail(err)
	}
	return 0
}
