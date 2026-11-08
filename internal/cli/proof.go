package cli

import (
	"fmt"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/proof"
)

func runProof(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid proof add <item> <file|--text text> [--kind screenshot|video|log|check] [--by agent] [--note text] [--task <task>] | index [--task <task>] [--json]")
		fmt.Fprintln(env.Stdout, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		return 0
	}
	op := args[0]
	if op != "add" && op != "index" {
		fmt.Fprintln(env.Stderr, "proof: expected add or index")
		return 2
	}
	fs := newFlags("proof "+op, env)
	fs.Usage = func() {
		if op == "add" {
			fmt.Fprintln(env.Stdout, "Usage: grid proof add <item> <file|--text text> [--kind screenshot|video|log|check] [--by agent] [--note text] [--task <task>] [--json]")
		} else {
			fmt.Fprintln(env.Stdout, "Usage: grid proof index [--task <task>] [--json]")
		}
		fmt.Fprintln(env.Stdout, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		fs.PrintDefaults()
	}
	id := fs.String("task", "", "task id or folder; defaults to the current task")
	asJSON := fs.Bool("json", false, "print JSON")
	in := proof.Input{By: "grid"}
	if op == "add" {
		fs.StringVar(&in.Text, "text", "", "text evidence")
		fs.StringVar(&in.Kind, "kind", "check", "evidence kind")
		fs.StringVar(&in.By, "by", "grid", "evidence author")
		fs.StringVar(&in.Note, "note", "", "one-line note")
	}
	args, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	if op == "index" && len(args) != 0 || op == "add" && (len(args) < 1 || len(args) > 2) {
		fmt.Fprintln(env.Stderr, "proof: unexpected operands")
		return 2
	}
	t, err := resolveTaskOperand(env, *id)
	if err != nil {
		return taskOperandError(env, "proof", err)
	}
	store := proof.Store{TaskDir: t.Path}
	if op == "index" {
		idx, err := store.Rebuild()
		if err != nil {
			return commandError(env, "proof", err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, idx)
		}
		for _, item := range idx.Items {
			state := "missing"
			if item.Proven {
				state = "saved"
			}
			fmt.Fprintf(env.Stdout, "%s: %s proof %s\n", item.ID, item.Level, state)
		}
		return 0
	}
	in.Item = args[0]
	if len(args) == 2 {
		in.File = args[1]
		if !filepath.IsAbs(in.File) {
			in.File = filepath.Join(env.Cwd, in.File)
		}
	}
	r, err := store.Add(in)
	if err != nil {
		return commandError(env, "proof", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, r)
	}
	fmt.Fprintln(env.Stdout, r.File)
	return 0
}
