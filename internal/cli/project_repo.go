package cli

import (
	"errors"
	"flag"
	"fmt"

	"github.com/gridconsole/grid-plugins/internal/projects"
)

const projectRepoUsage = "usage: grid project repo add|edit|remove <project> [--id ID] [--where W] [--remote R] [--branch B] [--json]"

// projectRepo writes projects/<project>/repos.json, the list the app's Settings tab reads.
// It is not named run*: the contract inventory follows run* callees for routes, and the
// repo verbs are one route, "project repo".
func projectRepo(args []string, env Env) int {
	fs := newFlags("project repo", env)
	asJSON := fs.Bool("json", false, "print JSON")
	id := fs.String("id", "", "repository id")
	path := fs.String("where", "", "repository folder")
	remote := fs.String("remote", "", "git remote")
	branch := fs.String("branch", "", "branch")
	if err := parsePositionals(fs, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	given := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { given[f.Name] = true })
	operands := fs.Args()
	if len(operands) == 0 {
		fmt.Fprintln(env.Stderr, projectRepoUsage)
		return 2
	}
	verb := operands[0]
	if verb != "add" && verb != "edit" && verb != "remove" {
		fmt.Fprintf(env.Stderr, "grid project repo: unknown command %q\n", verb)
		return 2
	}
	if len(operands) != 2 {
		fmt.Fprintf(env.Stderr, "grid project repo %s: wrong number of arguments\n%s\n", verb, projectRepoUsage)
		return 2
	}
	project := operands[1]
	usage := func(format string, a ...any) int {
		fmt.Fprintf(env.Stderr, "grid project repo %s: %s\n", verb, fmt.Sprintf(format, a...))
		return 2
	}
	switch verb {
	case "add":
		if *path == "" {
			return usage("--where is required")
		}
	case "edit":
		if *id == "" {
			return usage("--id is required")
		}
		if !given["where"] && !given["remote"] && !given["branch"] {
			return usage("nothing to change; give --where, --remote or --branch")
		}
		if given["where"] && *path == "" {
			return usage("--where cannot be empty")
		}
	case "remove":
		if *id == "" {
			return usage("--id is required")
		}
		for _, name := range []string{"where", "remote", "branch"} {
			if given[name] {
				return usage("--%s does not apply", name)
			}
		}
	}
	if resolver(env).Caller.IsGuest() {
		return failure(env, errors.New(guestSettingsRefusal))
	}
	s := projectStore(env)
	var item projects.RepoItem
	var err error
	switch verb {
	case "add":
		item, err = s.RepoAdd(project, *id, *path, *remote, *branch)
	case "edit":
		change := projects.RepoChange{}
		if given["where"] {
			change.Where = path
		}
		if given["remote"] {
			change.Remote = remote
		}
		if given["branch"] {
			change.Branch = branch
		}
		item, err = s.RepoEdit(project, *id, change)
	case "remove":
		err = s.RepoRemove(project, *id)
	}
	if err != nil {
		return commandError(env, "grid project repo", err)
	}
	if *asJSON {
		answer := struct {
			State string             `json:"state"`
			Item  *projects.RepoItem `json:"item,omitempty"`
		}{State: "applied"}
		if verb != "remove" {
			answer.Item = &item
		}
		return writeJSON(env.Stdout, answer)
	}
	if verb == "remove" {
		fmt.Fprintf(env.Stdout, "removed: %s\n", *id)
		return 0
	}
	fmt.Fprintf(env.Stdout, "repo: %s\nwhere: %s\n", item.Get("id"), item.Get("where"))
	return 0
}
