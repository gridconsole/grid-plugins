package cli

import (
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/projectmd"
)

func runProjectMD(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid project-md get|set <project> <key> [value] [--json]")
		return 0
	}
	op := args[0]
	if op != "get" && op != "set" {
		fmt.Fprintln(env.Stderr, "project-md: expected get or set")
		return 2
	}
	f := newFlags("project-md "+op, env)
	asJSON := f.Bool("json", false, "print JSON")
	operands, code, stop := parseMixed(f, args[1:])
	if stop {
		return code
	}
	if len(operands) < 2 || len(operands) > 3 || (op == "get" && len(operands) != 2) {
		fmt.Fprintln(env.Stderr, "project-md: expected project, key and an optional set value")
		return 2
	}
	var value *string
	if op == "set" {
		v := ""
		if len(operands) == 3 {
			v = operands[2]
		}
		value = &v
	}
	result, err := projectmd.Key(env.GridHome, operands[0], operands[1], value)
	if err != nil {
		return commandError(env, "project-md", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, result)
	}
	_, err = fmt.Fprintf(env.Stdout, "%s: %s\n", result.Key, projectmd.Plain(result.Value))
	if err != nil {
		return commandError(env, "project-md", err)
	}
	return 0
}
