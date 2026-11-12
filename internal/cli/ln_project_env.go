package cli

import (
	"errors"
	"flag"
	"fmt"
	iofs "io/fs"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/projectenv"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const projectEnvUsage = "usage: grid project env list|add|edit|remove <project> [--id ID] [--name N] [--kind local|remote|cloud] [--where W] [--runs R] [--test T] [--how H] [--from F] [--who P]... [--secret NAME:set|missing]... [--approval yes|no] [--json]"

// lnEnvFields are the flags that set a field of an environment.
var lnEnvFields = []string{"name", "kind", "where", "runs", "test", "how", "from", "who", "secret", "approval"}

// lnEnvRepeat collects a flag that may be given more than once. Set never fails,
// so the flag package never prints a value back.
type lnEnvRepeat []string

func (l *lnEnvRepeat) String() string     { return strings.Join(*l, ",") }
func (l *lnEnvRepeat) Set(v string) error { *l = append(*l, v); return nil }

// lnEnvItem is one environment in the field names the app's project_environments
// read answers (app internal/slices/projectsettings/read.go environmentOf): id,
// name, kind, where and runs always; test, how and from when set; a list when it
// has items; approval when the environment says.
type lnEnvItem struct {
	ID       string        `json:"id"`
	Name     string        `json:"name"`
	Kind     string        `json:"kind"`
	Where    string        `json:"where"`
	Runs     string        `json:"runs"`
	Test     string        `json:"test,omitempty"`
	How      string        `json:"how,omitempty"`
	From     string        `json:"from,omitempty"`
	Who      []string      `json:"who,omitempty"`
	Secrets  []lnEnvSecret `json:"secrets,omitempty"`
	Steps    []lnEnvStep   `json:"steps,omitempty"`
	Approval *bool         `json:"approval,omitempty"`
}

type lnEnvSecret struct {
	Name string `json:"name"`
	Set  bool   `json:"set"`
}

type lnEnvStep struct {
	WF   string `json:"wf"`
	Step string `json:"step"`
	Text string `json:"text,omitempty"`
}

func lnEnvItemOf(e projectenv.Entry) lnEnvItem {
	it := lnEnvItem{ID: e.ID, Name: e.Name, Kind: e.Kind, Where: e.Where, Runs: e.Runs, Test: e.Test, How: e.How, From: e.From, Who: e.Who, Approval: e.Approval}
	for _, s := range e.Secrets {
		it.Secrets = append(it.Secrets, lnEnvSecret{Name: s.Name, Set: s.Set})
	}
	for _, s := range e.Steps {
		it.Steps = append(it.Steps, lnEnvStep{WF: s.WF, Step: s.Step, Text: s.Text})
	}
	return it
}

// lnEnvClearable is a repeated flag's values; one empty value clears the list.
func lnEnvClearable(vals []string) []string {
	if len(vals) == 1 && vals[0] == "" {
		return nil
	}
	return vals
}

// projectEnv stores a project's environments in the brain through
// internal/projectenv, with the person as the audit author. It is not named
// run*: the contract inventory follows run* callees for routes, and the env
// verbs are one route, "project env".
func projectEnv(args []string, env Env) int {
	fs := newFlags("project env", env)
	asJSON := fs.Bool("json", false, "print JSON")
	id := fs.String("id", "", "environment id")
	name := fs.String("name", "", "environment name")
	kind := fs.String("kind", "", "local, remote or cloud")
	place := fs.String("where", "", "where it runs")
	runs := fs.String("runs", "", "what it runs")
	test := fs.String("test", "", "how it is tested")
	how := fs.String("how", "", "how to reach it")
	from := fs.String("from", "", "what it is made from")
	approval := fs.String("approval", "", "yes or no")
	var who, secret lnEnvRepeat
	fs.Var(&who, "who", "a person who may use it; repeat for more")
	fs.Var(&secret, "secret", "NAME:set or NAME:missing; repeat for more")
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
		fmt.Fprintln(env.Stderr, projectEnvUsage)
		return 2
	}
	verb := operands[0]
	if verb != "list" && verb != "add" && verb != "edit" && verb != "remove" {
		fmt.Fprintf(env.Stderr, "grid project env: unknown command %q\n", verb)
		return 2
	}
	if len(operands) != 2 {
		fmt.Fprintf(env.Stderr, "grid project env %s: wrong number of arguments\n%s\n", verb, projectEnvUsage)
		return 2
	}
	project := operands[1]
	usage := func(format string, a ...any) int {
		fmt.Fprintf(env.Stderr, "grid project env %s: %s\n", verb, fmt.Sprintf(format, a...))
		return 2
	}
	fields := 0
	for _, f := range lnEnvFields {
		if given[f] {
			fields++
		}
	}
	switch verb {
	case "list":
		if given["id"] || fields > 0 {
			return usage("only --json applies")
		}
	case "add":
		for _, f := range []string{"id", "name", "kind"} {
			if !given[f] {
				return usage("--%s is required", f)
			}
		}
	case "edit":
		if !given["id"] {
			return usage("--id is required")
		}
		if fields == 0 {
			return usage("nothing to change; give a field flag such as --name or --test")
		}
	case "remove":
		if !given["id"] {
			return usage("--id is required")
		}
		if fields > 0 {
			return usage("only --id and --json apply")
		}
	}
	// A secret is named with its state, never given a value. The message names
	// the flag only, so a refused value is not printed.
	var secrets []projectenv.Secret
	for _, v := range lnEnvClearable(secret) {
		n, state, ok := strings.Cut(v, ":")
		if !ok || (state != "set" && state != "missing") {
			return usage("--secret takes NAME:set or NAME:missing; a secret value is never a flag")
		}
		secrets = append(secrets, projectenv.Secret{Name: n, Set: state == "set"})
	}
	if *approval != "" && *approval != "yes" && *approval != "no" {
		return usage("--approval takes yes or no")
	}
	if resolver(env).Caller.IsGuest() {
		return failure(env, ilGuestRefused)
	}
	store, err := lnEnvStore(env, project)
	if err != nil {
		return commandError(env, "grid project env", err)
	}
	if verb == "list" {
		entries, err := projectenv.List(store)
		if err != nil {
			return commandError(env, "grid project env", err)
		}
		items := []lnEnvItem{}
		for _, e := range entries {
			items = append(items, lnEnvItemOf(e))
		}
		if *asJSON {
			return writeJSON(env.Stdout, struct {
				Items []lnEnvItem `json:"items"`
			}{items})
		}
		if len(items) == 0 {
			fmt.Fprintln(env.Stdout, "no environments")
		}
		for _, it := range items {
			fmt.Fprintf(env.Stdout, "%s: %s\n", it.ID, it.Name)
		}
		return 0
	}
	origin, err := lnEnvOrigin(env)
	if err != nil {
		return commandError(env, "grid project env", err)
	}
	var stored projectenv.Entry
	switch verb {
	case "add", "edit":
		// The stored entry is read, merged with the flags and written inside one
		// brain lock, so the add and edit decisions are made on the bytes that the
		// write replaces.
		var err error
		stored, err = projectenv.EditBy(store, *id, origin, func(e projectenv.Entry, exists bool) (projectenv.Entry, error) {
			switch {
			case exists && verb == "add":
				return e, fmt.Errorf("environment %s already exists; use edit", *id)
			case !exists && verb == "edit":
				return e, fmt.Errorf("environment %s: %w", *id, iofs.ErrNotExist)
			}
			// A flag that is given replaces its field; every other field, the steps
			// included, keeps what is stored. who is the environment's people list,
			// not an author.
			pick := func(flag string, value *string, kept string) string {
				if given[flag] {
					return *value
				}
				return kept
			}
			people, named, approve := e.Who, e.Secrets, e.Approval
			if given["who"] {
				people = lnEnvClearable(who)
			}
			if given["secret"] {
				named = secrets
			}
			if given["approval"] {
				approve = nil
				if *approval != "" {
					v := *approval == "yes"
					approve = &v
				}
			}
			return projectenv.Entry{ID: e.ID, Name: pick("name", name, e.Name), Kind: pick("kind", kind, e.Kind),
				Where: pick("where", place, e.Where), Runs: pick("runs", runs, e.Runs), Test: pick("test", test, e.Test),
				How: pick("how", how, e.How), From: pick("from", from, e.From), Who: people, Secrets: named, Steps: e.Steps, Approval: approve}, nil
		})
		if err != nil {
			return commandError(env, "grid project env", err)
		}
	case "remove":
		if err := projectenv.RemoveBy(store, *id, origin); err != nil {
			return commandError(env, "grid project env", err)
		}
	}
	if *asJSON {
		answer := struct {
			State string     `json:"state"`
			Item  *lnEnvItem `json:"item,omitempty"`
		}{State: "applied"}
		if verb != "remove" {
			it := lnEnvItemOf(stored)
			answer.Item = &it
		}
		return writeJSON(env.Stdout, answer)
	}
	if verb == "remove" {
		fmt.Fprintf(env.Stdout, "removed: %s\n", *id)
		return 0
	}
	fmt.Fprintf(env.Stdout, "environment: %s\nname: %s\n", stored.ID, stored.Name)
	return 0
}

// lnEnvStore opens the brain of the registered project named by name or id, as
// project repo finds it (internal/projects/repos.go repoChange). An unknown
// project answers the unregistered lookup error and nothing is created.
func lnEnvStore(env Env, name string) (brain.Store, error) {
	r := projectStore(env).Resolver
	if r.GridHome == "" {
		return brain.Store{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	records, err := r.Projects()
	if err != nil {
		return brain.Store{}, err
	}
	project, matches := "", 0
	for _, rec := range records {
		if (rec.Name == name || rec.ID == name) && r.PersonalProjectAllowed(rec.Name) {
			project, matches = rec.Name, matches+1
		}
	}
	switch {
	case matches == 0:
		return brain.Store{}, &where.LookupError{Kind: where.ErrProjectUnregistered, Message: fmt.Sprintf("project %q is not registered", name)}
	case matches > 1:
		return brain.Store{}, fmt.Errorf("project %q is ambiguous", name)
	}
	return brain.Store{GridHome: r.GridHome, Project: project}, nil
}

// lnEnvOrigin names the person the host runs the plugin for, else the session,
// as grid brain drop does (internal/braincmd/hy_drop.go hyDrOrigin). No flag
// reaches it.
func lnEnvOrigin(env Env) (brain.Origin, error) {
	if member := brainMember(env); brain.ValidName(member) {
		return brain.Origin{Kind: "person", ID: member}, nil
	}
	return brain.SessionOrigin()
}
