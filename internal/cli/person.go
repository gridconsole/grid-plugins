package cli

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/people"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func runPerson(args []string, env Env) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprintln(env.Stdout, "Usage: grid person list [--project P] [--json]\n       grid person show <handle> [--project P] [--json]\n       grid person approvers --area <area> [--task <folder or id>] [--project P] [--json]\n       grid person check [--json]\n\nReads the synced people cache. Without --project, reads home, or the supplied task's project then home.\nNo match requires --task with an owner in task.md.\ncheck answers whether the caller is a person: {\"ok\":true}, or exit 3 with person_required and a reason.\nExit codes: 0 success, 1 read failure or unknown handle, 2 usage error, 3 check: not a person.")
		return 0
	}
	verb := args[0]
	if verb == "check" {
		return runPersonCheck(args[1:], env)
	}
	fs := newFlags("person "+verb, env)
	project := fs.String("project", "", "project name or id; reads project then home")
	asJSON := fs.Bool("json", false, "print JSON")
	var area, taskArg string
	switch verb {
	case "list", "show":
	case "approvers":
		fs.StringVar(&area, "area", "", "exact approval area")
		fs.StringVar(&taskArg, "task", "", "task folder or id supplying the default owner")
	default:
		fmt.Fprintf(env.Stderr, "person: unknown command %q\n", verb)
		return 2
	}
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	count := 0
	if verb == "show" {
		count = 1
	}
	if fs.NArg() != count {
		fmt.Fprintf(env.Stderr, "person %s: expected %d arguments\n", verb, count)
		return 2
	}
	if verb == "approvers" && strings.TrimSpace(area) == "" {
		fmt.Fprintln(env.Stderr, "person approvers: --area is required")
		return 2
	}
	owner := ""
	if *project != "" || taskArg != "" {
		res, err := resolver(env).Resolve(where.Query{Cwd: env.Cwd, Project: *project, Task: taskArg})
		if err != nil {
			return failure(env, err)
		}
		*project = res.Project
		if *project == gridhome.Personal {
			*project = ""
		}
		if taskArg != "" {
			t, err := task.Read(res.TaskPath)
			if err != nil {
				return failure(env, err)
			}
			owner = t.Owner
		}
	}
	store := people.Store{GridHome: env.GridHome, Project: *project}
	if verb == "show" {
		entry, err := store.Get(fs.Arg(0))
		if err != nil {
			return failure(env, err)
		}
		if *asJSON {
			return writeJSON(env.Stdout, entry)
		}
		fmt.Fprintf(env.Stdout, "%s  %s  %s\n", entry.Handle, entry.Name, entry.Role)
		for _, link := range entry.Links {
			fmt.Fprintf(env.Stdout, "%s  %s  %s  %s\n", link.Rel, link.To, link.Summary, link.Why)
		}
		fmt.Fprint(env.Stdout, entry.Body)
		return 0
	}
	entries, problems, err := store.List()
	if err != nil {
		return failure(env, err)
	}
	if verb == "list" && *asJSON {
		return writeJSON(env.Stdout, struct {
			People   []people.Entry   `json:"people"`
			Problems []people.Problem `json:"problems"`
		}{entries, problems})
	}
	for _, problem := range problems {
		fmt.Fprintf(env.Stderr, "person %s: %s\n", problem.Name, problem.Reason)
	}
	if verb == "list" {
		for _, entry := range entries {
			fmt.Fprintf(env.Stdout, "%s  %s  %s\n", entry.Handle, entry.Name, entry.Role)
		}
		return 0
	}
	// An empty sentinel distinguishes a people match from fallback even when
	// the matched person happens to be the task owner.
	approvers := people.Entries(entries).Approvers(area, "")
	source := "people"
	if len(approvers) == 1 && approvers[0] == "" {
		if strings.TrimSpace(owner) == "" {
			fmt.Fprintln(env.Stderr, "person approvers: no matching people; supply --task with an owner in task.md")
			return 2
		}
		approvers = people.Entries(entries).Approvers(area, owner)
		source = "default"
	}
	if *asJSON {
		return writeJSON(env.Stdout, struct {
			Approvers []string `json:"approvers"`
			Source    string   `json:"source"`
		}{approvers, source})
	}
	for _, handle := range approvers {
		fmt.Fprintln(env.Stdout, handle)
	}
	fmt.Fprintf(env.Stdout, "source: %s\n", source)
	return 0
}
