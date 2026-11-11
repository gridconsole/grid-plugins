package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runAgent(args []string, env Env) int { return runSettings("agent", args, env) }
func runSkill(args []string, env Env) int { return runSettings("skill", args, env) }

func settingsUsage(kind string, env Env) {
	switch kind {
	case "agent":
		fmt.Fprintln(env.Stdout, "Usage: grid agent set <agent> [--model M] [--effort E] [--project P]\n       grid agent list [--json]\nField   Allowed values\nmodel   haiku, sonnet, opus\neffort  low, medium, high, xhigh, max")
	case "skill":
		fmt.Fprintln(env.Stdout, "Usage: grid skill enable|disable <skill> [--project P]\n       grid skill list [--json]")
	case "hook":
		fmt.Fprintln(env.Stdout, "Usage: grid hook level <hook> off|warn|block [--project P]\n       grid hook levels [--json]\nEvery hook takes any level; host-guard, only-writer and plan-guard default to warn.")
	}
}

// A write without --project is personal. A list resolves the current project,
// but an unregistered folder has only personal values to inherit. Resolve that
// fallback explicitly too: an unregistered folder does not grant Personal access.
func settingsStore(env Env, project string, listing bool) (settings.Store, error) {
	if !listing && project == "" {
		project = gridhome.Personal
	}
	r, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: project})
	if err != nil {
		return settings.Store{}, err
	}
	if !r.Record && r.Project != gridhome.Personal {
		r, err = resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: gridhome.Personal})
		if err != nil {
			return settings.Store{}, err
		}
	}
	if !listing && resolver(env).Caller.IsGuest() {
		return settings.Store{}, errors.New(guestSettingsRefusal)
	}
	return settings.Store{Home: env.GridHome, Project: r.Project}, nil
}

func runSettings(kind string, args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		settingsUsage(kind, env)
		return 0
	}
	op := args[0]
	listing := op == "list" || (kind == "hook" && op == "levels")
	valid := kind == "agent" && (op == "set" || op == "list") || kind == "skill" && (op == "enable" || op == "disable" || op == "list") || kind == "hook" && (op == "level" || op == "levels")
	if !valid {
		fmt.Fprintf(env.Stderr, "%s: unknown command %q\n", kind, op)
		return 2
	}
	fs := newFlags(kind+" "+op, env)
	project := new(string)
	asJSON := fs.Bool("json", false, "print JSON")
	model, effort := new(string), new(string)
	if listing {

	} else {
		project = fs.String("project", "", "registered project name or id; defaults to personal")
	}
	if kind == "agent" && !listing {
		model = fs.String("model", "", "haiku, sonnet, opus")
		effort = fs.String("effort", "", "low, medium, high, xhigh, max")
	}
	operands, code, stop := parseMixed(fs, args[1:])
	if stop {
		return code
	}
	count := 1
	if listing {
		count = 0
	} else if kind == "hook" {
		count = 2
	}
	if len(operands) != count {
		fmt.Fprintf(env.Stderr, "%s %s: expected %d operands\n", kind, op, count)
		return 2
	}
	if kind == "agent" && !listing {
		invalid := false
		fs.Visit(func(f *flag.Flag) {
			if (f.Name == "model" || f.Name == "effort") && f.Value.String() == "" {
				invalid = true
			}
		})
		if invalid {
			fmt.Fprintln(env.Stderr, "agent: model and effort must not be empty")
			return 2
		}
	}
	store, err := settingsStore(env, *project, listing)
	if err != nil {
		return commandError(env, kind, err)
	}
	if listing {
		if kind == "hook" {
			levels, err := settings.ReadHookLevels(store.Home, store.Project)
			if err != nil {
				return commandError(env, kind, err)
			}
			if *asJSON {
				return writeJSON(env.Stdout, levels)
			}
			for _, name := range settings.HookNames() {
				fmt.Fprintf(env.Stdout, "%s %s\n", name, levels[name])
			}
			return 0
		}
		doc, err := store.Effective()
		if err != nil {
			return commandError(env, kind, err)
		}
		if kind == "agent" {
			if *asJSON {
				return writeJSON(env.Stdout, doc.Agents)
			}
			for _, name := range settings.AgentNames() {
				a := doc.Agents[name]
				fmt.Fprintf(env.Stdout, "%s model=%s effort=%s\n", name, a.Model, a.Effort)
			}
		} else {
			if *asJSON {
				return writeJSON(env.Stdout, doc.Skills)
			}
			for _, name := range settings.SkillNames() {
				state := "disabled"
				if doc.Skills[name] {
					state = "enabled"
				}
				fmt.Fprintf(env.Stdout, "%s %s\n", name, state)
			}
		}
		return 0
	}
	switch kind {
	case "agent":
		err = store.SetAgent(operands[0], settings.Agent{Model: *model, Effort: *effort})
	case "skill":
		err = store.SetSkill(operands[0], op == "enable")
	case "hook":
		err = store.SetHook(operands[0], operands[1])
	}
	if err != nil {
		return commandError(env, kind, err)
	}
	scope := store.Project
	if scope == "" {
		scope = gridhome.Personal
	}
	if *asJSON {
		return writeJSON(env.Stdout, map[string]string{"kind": kind, "name": operands[0], "project": scope})
	}
	fmt.Fprintf(env.Stdout, "%s %s updated in %s\n", kind, operands[0], scope)
	return 0
}

func runSetup(args []string, env Env) int {
	if resolver(env).Caller.IsGuest() {
		return commandError(env, "setup", errors.New(guestSettingsRefusal))
	}
	if settings.ClaudeSetupCommand(args) {
		fs := newFlags("setup", env)
		fs.Usage = func() {
			fmt.Fprintln(env.Stdout, "Usage: grid setup [--sandbox] [--remove] [--yes] [--json]\n       grid setup <project> [--answers <file|->] [--json]")
		}
		yes := fs.Bool("yes", false, "write without prompting")
		remove := fs.Bool("remove", false, "remove the Claude Code entries added by setup")
		sandbox := fs.Bool("sandbox", false, "switch on Claude Code's own sandbox for this user")
		asJSON := fs.Bool("json", false, "print JSON")
		_, code, stop := parseMixed(fs, args)
		if stop {
			return code
		}
		if *sandbox && *remove {
			fmt.Fprintln(env.Stderr, "Usage: grid setup [--sandbox] [--remove] [--yes] [--json]; --remove already takes out the sandbox entry setup wrote")
			return 2
		}
		projects := env.getenv("GRID_PROJECTS")
		if projects != "" && !filepath.IsAbs(projects) {
			projects = filepath.Join(env.Cwd, projects)
		}
		return settings.RunClaudeSetup(settings.ClaudeSetupOptions{
			Home: env.Home, GridHome: env.GridHome, ConfigDir: env.getenv("CLAUDE_CONFIG_DIR"),
			Projects: projects, Yes: *yes, Remove: *remove, Sandbox: *sandbox, JSON: *asJSON,
			Stdin: env.Stdin, Stdout: env.Stdout, Stderr: env.Stderr,
		})
	}
	fs := newFlags("setup", env)
	asJSON := fs.Bool("json", false, "print JSON")
	source := fs.String("answers", "setup-answers.json", "answers JSON file relative to the current folder, or - for stdin")
	operands, code, stop := parseMixed(fs, args)
	if stop {
		return code
	}
	if len(operands) != 1 {
		fmt.Fprintln(env.Stderr, "Usage: grid setup <project> [--answers <file|->] [--json]")
		return 2
	}
	store, err := settingsStore(env, operands[0], false)
	if err != nil {
		return commandError(env, "setup", err)
	}
	var r io.Reader = env.Stdin
	if *source != "-" {
		path := *source
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Cwd, path)
		}
		info, err := os.Stat(path)
		if err != nil {
			return commandError(env, "setup", err)
		}
		if !info.Mode().IsRegular() {
			return commandError(env, "setup", fmt.Errorf("answers must be a regular JSON file"))
		}
		f, err := os.Open(path)
		if err != nil {
			return commandError(env, "setup", err)
		}
		defer f.Close()
		r = f
	}
	if r == nil {
		return commandError(env, "setup", fmt.Errorf("no answers input"))
	}
	const limit = 1 << 20
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return commandError(env, "setup", err)
	}
	if len(data) > limit {
		return commandError(env, "setup", fmt.Errorf("answers exceed %d bytes", limit))
	}
	answers, err := store.RecordSetup(data)
	if err != nil {
		return commandError(env, "setup", err)
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Project string                `json:"project"`
			Answers settings.SetupAnswers `json:"answers"`
		}{store.Project, answers})
	}
	fmt.Fprintf(env.Stdout, "Setup answers recorded for %s\n", store.Project)
	return 0
}
