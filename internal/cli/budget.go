package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// budgetOptions builds the options from the environment grid was started in;
// the env read is that of the process, which is what Grid starts agents from.
func budgetOptions(env Env) settings.BudgetOptions {
	return settings.BudgetOptions{Home: env.Home, GridHome: env.GridHome, ConfigDir: env.getenv("CLAUDE_CONFIG_DIR"),
		ProjectDir: env.Cwd, Getenv: env.getenv}
}

func budgetUsage(env Env) {
	fmt.Fprintln(env.Stdout, "Usage: grid agent budget show [--json]\n       grid agent budget apply <economic|balanced|high> [--dry-run] [--with compact,advisor] [--expect <digest>] [--json]\n       grid agent budget undo [--json]\nPrints JSON. Apply writes the overlay, the model and effort lines of Grid's own agent files and, with --with, settings.json.")
}

// runAgentBudget serves the budget panel: show, apply (and its dry run) and undo.
func runAgentBudget(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		budgetUsage(env)
		return 0
	}
	op := args[0]
	if op != "show" && op != "apply" && op != "undo" {
		fmt.Fprintf(env.Stderr, "agent budget: unknown command %q\n", op)
		return 2
	}
	fs := newFlags("agent budget "+op, env)
	fs.Bool("json", false, "print JSON (always on)")
	dry, with, expect := new(bool), new(string), new(string)
	if op == "apply" {
		dry = fs.Bool("dry-run", false, "show what would change and write nothing")
		with = fs.String("with", "", "settings.json rows to write too: compact, advisor")
		expect = fs.String("expect", "", "digest from the dry run; refuse if the files changed since")
	}
	operands, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	want := 0
	if op == "apply" {
		want = 1
	}
	if len(operands) != want {
		fmt.Fprintf(env.Stderr, "agent budget %s: expected %d operands\n", op, want)
		return 2
	}
	// A write goes through the caller-aware store, which refuses a guest; a read or a dry run does not write.
	store, err := settingsStore(env, gridhome.Personal, op == "show" || *dry)
	if err != nil {
		return commandError(env, "agent", err)
	}
	o := store.WithBudget(budgetOptions(env))
	var out any
	switch op {
	case "show":
		out, err = settings.BudgetShow(o)
	case "undo":
		out, err = settings.BudgetUndo(o)
	default:
		o.Preset, o.Expect = operands[0], *expect
		if *with != "" {
			o.With = strings.Split(*with, ",")
		}
		if *dry {
			out, err = settings.BudgetPreviewFor(o)
		} else {
			out, err = settings.BudgetApply(o)
		}
	}
	if err != nil {
		return commandError(env, "agent", err)
	}
	return writeJSON(env.Stdout, out)
}

// runClaude serves "grid claude settings show|set": the person's Claude Code
// settings rows, read with their env overrides and written one key at a time.
func runClaude(args []string, env Env) int {
	if len(args) < 2 || args[0] != "settings" || args[1] != "show" && args[1] != "set" {
		fmt.Fprintln(env.Stdout, "Usage: grid claude settings show [--project <p>] [--json]\n       grid claude settings set <key> <json-value|null> [--expect <json-value|null>] [--json]\nKeys: "+strings.Join(settings.ClaudeRowNames(), ", ")+". Prints JSON.")
		if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
			return 0
		}
		return 2
	}
	op := args[1]
	fs := newFlags("claude settings "+op, env)
	fs.Bool("json", false, "print JSON (always on)")
	projectDir, expect := new(string), new(string)
	hasExpect := false
	if op == "show" {
		projectDir = fs.String("project", "", "registered project name or id whose .claude settings apply")
	} else {
		expect = fs.String("expect", "", "the raw value the person saw, or null")
	}
	operands, code, stop := parseMixed(fs, args[2:])
	if stop {
		return code
	}
	fs.Visit(func(f *flag.Flag) { hasExpect = hasExpect || f.Name == "expect" })
	want := 0
	if op == "set" {
		want = 2
	}
	if len(operands) != want {
		fmt.Fprintf(env.Stderr, "claude settings %s: expected %d operands\n", op, want)
		return 2
	}
	configDir := env.getenv("CLAUDE_CONFIG_DIR")
	if op == "show" {
		dir := ""
		if *projectDir != "" { // the project's own .claude layers; a hidden or unknown project is refused like any other
			r, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: *projectDir})
			if err != nil {
				return commandError(env, "claude", err)
			}
			dir = r.Folder
		}
		res, err := settings.ClaudeShow(settings.ClaudeShowOptions{Home: env.Home, ConfigDir: configDir, ProjectDir: dir, Getenv: env.getenv})
		if err != nil {
			return commandError(env, "claude", err)
		}
		return writeJSON(env.Stdout, res)
	}
	if resolver(env).Caller.IsGuest() {
		return commandError(env, "claude", ilGuestRefused)
	}
	o := settings.ClaudeSetOptions{Home: env.Home, ConfigDir: configDir, Key: operands[0], Value: jsonOrText(operands[1]), HasExpect: hasExpect, Expect: jsonOrText(*expect)}
	res, err := settings.ClaudeSet(o)
	if err != nil {
		return commandError(env, "claude", err)
	}
	return writeJSON(env.Stdout, res)
}

// jsonOrText accepts a JSON value, or a bare word such as opus, which is a string.
func jsonOrText(s string) json.RawMessage {
	if s != "" && json.Valid([]byte(s)) && !bytes.HasPrefix([]byte(s), []byte(" ")) {
		return json.RawMessage(s)
	}
	b, _ := json.Marshal(s)
	return b
}
