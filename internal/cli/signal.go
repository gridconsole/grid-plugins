package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

func runSignal(args []string, env Env) int {
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "usage: grid signal add|act|dismiss|move")
		return 2
	}
	verb := args[0]
	if verb != "add" && verb != "act" && verb != "dismiss" && verb != "move" {
		fmt.Fprintf(env.Stderr, "grid signal: unknown command %q\n", verb)
		return 2
	}
	fs := newFlags("signal "+verb, env)
	asJSON := fs.Bool("json", false, "print JSON")
	proj := fs.String("project", "", "project name or id; defaults to this folder's project")
	var to string
	if verb == "move" {
		fs.StringVar(&to, "to", "", "destination project; - means the workspace inbox (Personal)")
	}
	var in signal.Input
	var bodyFile string
	var act signal.ActOptions
	if verb == "act" {
		fs.StringVar(&act.Act, "act", "note", "turn|add|note|snooze|dismiss|wake|reopen")
		fs.StringVar(&act.Task, "task", "", "task id for add (T-<n>)")
		fs.StringVar(&act.Until, "until", "", "snooze until RFC3339 or YYYY-MM-DDTHH:MM")
		fs.BoolVar(&act.Later, "later", false, "reserved; currently refused")
	}
	if verb == "add" {
		fs.StringVar(&in.Source, "source", "typed", "source name")
		fs.StringVar(&in.SourceID, "source-id", "", "stable id in this source for deduplication")
		fs.StringVar(&in.Title, "title", "", "signal title")
		fs.StringVar(&in.Sender, "sender", "", "who sent it, as the source names them")
		fs.StringVar(&in.Recipient, "recipient", "", "the address or channel it came to")
		fs.StringVar(&in.Body, "body", "", "external text, or - to read stdin")
		fs.StringVar(&bodyFile, "body-file", "", bodyFileUsage)
	}
	if err := parsePositionals(fs, args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		if verb != "add" {
			signalErrorJSON(env, errorJSONRequested(fs, args[1:]), "invalid", "invalid signal arguments", CodeInvalidArgument)
		}
		return 2
	}
	if (verb == "add" && fs.NArg() != 0) || (verb != "add" && fs.NArg() != 1) {
		fmt.Fprintf(env.Stderr, "grid signal %s: wrong number of arguments\n", verb)
		if verb != "add" {
			signalErrorJSON(env, *asJSON, "invalid", "wrong number of arguments", CodeInvalidArgument)
		}
		return 2
	}
	if resolver(env).Caller.IsGuest() {
		if verb == "add" {
			return failure(env, errors.New(guestSettingsRefusal))
		}
		return signalFailure(env, &signal.RefusedError{Message: "guests cannot act on signals", Code: CodeGuestRefused}, *asJSON)
	}
	if verb == "move" {
		inbox := to == "-"
		if inbox {
			to = where.Personal
		}
		from := *proj
		if from == "" {
			from = where.Personal
		}
		if err := signal.ValidateMove(fs.Arg(0), from, to); err != nil {
			return signalFailure(env, err, *asJSON)
		}
		if inbox {
			var err error
			to, err = workflow.InboxWorkspace(env.GridHome)
			if err != nil {
				fmt.Fprintf(env.Stderr, "grid signal move: %s\n", workflow.ErrInboxWorkspace)
				if *asJSON {
					writeJSON(env.Stdout, struct {
						Error   string `json:"error"`
						Key     string `json:"key"`
						Message string `json:"message"`
						Code    string `json:"code"`
					}{"invalid_policy", "inbox_workspace", workflow.ErrInboxWorkspace.Error(), CodeInvalidPolicy})
				}
				return 1
			}
		}
	}
	if verb == "act" || verb == "dismiss" {
		if verb == "dismiss" {
			act.Act = "dismiss"
		}
		fs.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "task":
				act.TaskSet = true
			case "until":
				act.UntilSet = true
			case "later":
				act.Later = true
			}
		})
		if err := act.Validate(fs.Arg(0)); err != nil {
			return signalFailure(env, err, *asJSON)
		}
		act.Cwd, act.Who, act.Role = env.Cwd, env.getenv("GRID_WHO"), env.getenv("GRID_ROLE")
		if act.Who == "" {
			act.Who = env.getenv("GRID_ACTOR")
		}
	}
	if bodyFile != "" {
		text, err := readBodyFile(env, fs, bodyFile, signal.MaxBytes+1)
		if err != nil {
			return failure(env, err)
		}
		if len(text) > signal.MaxBytes {
			return failure(env, fmt.Errorf("signal body exceeds %d bytes", signal.MaxBytes))
		}
		in.Body = text
	} else if in.Body == "-" {
		data, err := io.ReadAll(io.LimitReader(env.Stdin, signal.MaxBytes+1))
		if err != nil {
			return failure(env, err)
		}
		if len(data) > signal.MaxBytes {
			return failure(env, fmt.Errorf("signal body exceeds %d bytes", signal.MaxBytes))
		}
		in.Body = string(data)
	}
	if verb == "add" {
		if err := signal.Validate(in); err != nil {
			return failure(env, err)
		}
	}
	ps := projectStore(env)
	res, err := ps.Resolver.Resolve(where.Query{Cwd: env.Cwd, Project: *proj})
	if err != nil {
		if verb == "move" && (errors.Is(err, where.ErrProjectUnregistered) || errors.Is(err, where.ErrPersonalRefused)) {
			return signalFailure(env, &signal.RefusedError{Message: "no such project", Code: CodeNoSuchProject, Cause: where.ErrProjectUnregistered}, *asJSON)
		}
		return failure(env, err)
	}
	if !res.Record && (res.Rule == where.RulePersonal || verb == "move" && res.Project == where.Personal) {
		entries, err := ps.List()
		if err != nil {
			return failure(env, err)
		}
		for _, e := range entries {
			if e.Name == where.Personal {
				res.Record = true
			}
		}
	}

	if !res.Record {
		if verb == "move" {
			return signalFailure(env, &signal.RefusedError{Message: "no such project", Code: CodeNoSuchProject, Cause: where.ErrProjectUnregistered}, *asJSON)
		}
		if res.Rule != where.RulePersonal {
			return failure(env, fmt.Errorf("project is not registered; run grid project add"))
		}
		if verb != "add" {
			return failure(env, fmt.Errorf("Personal has no signals yet"))
		}
		e, err := ps.Add(env.Cwd, "")
		if err != nil {
			return failure(env, err)
		}
		res.Project = e.Name
	}
	s := signal.Store{GridHome: env.GridHome, Project: res.Project, Resolver: projectStore(env).Resolver}
	var got signal.Result
	switch verb {
	case "add":
		got, err = s.Add(in)
	case "act", "dismiss":
		got, err = s.Apply(fs.Arg(0), act)
	case "move":
		got, err = s.Move(fs.Arg(0), res.Project, to)
	}
	if err != nil {
		return signalFailure(env, err, *asJSON)
	}
	// A turn made a task through task.Store.New, as grid task new does, so it gets J-1
	// the same way: a failure to post it is said out loud, never left as a task nothing starts.
	var postErr error
	if verb == "act" && act.Act == "turn" && got.Became != "" {
		postErr = signalPostFirstJob(env, got.Became)
	}
	if *asJSON {
		code := writeJSON(env.Stdout, got)
		if code == 0 && postErr != nil {
			return failure(env, postErr)
		}
		return code
	}
	fmt.Fprintf(env.Stdout, "result: %s\nid: %s\nproject: %s\nnote: %s\nstatus: %s\n", got.Result, got.ID, got.Project, got.Note, got.Status)
	if postErr != nil {
		return failure(env, postErr)
	}
	return 0
}

// signalPostFirstJob reads the task a turn made and posts its first Job.
func signalPostFirstJob(env Env, id string) error {
	t, err := (task.Store{Resolver: resolver(env)}).Show(env.Cwd, id)
	if err != nil {
		return fmt.Errorf("%s was created but its first Job was not posted: %w", id, err)
	}
	// The Feed author roles are lead, owner, member and cli. A signal can be turned by
	// an agent, whose role the Feed does not take, so the record is written by the CLI.
	switch t.OwnerRole {
	case "lead", "owner", "member", "cli":
	default:
		t.OwnerRole = "cli"
	}
	if err := postFirstJob(env, t, task.NewOptions{}); err != nil {
		return fmt.Errorf("%s was created but its first Job was not posted: %w", id, err)
	}
	return nil
}

// signalFailure preserves the existing stderr prefix while classifying act errors.
func signalFailure(env Env, err error, asJSON bool) int {
	code := failure(env, err)
	var bad *signal.BadInputError
	var refused *signal.RefusedError
	switch {
	case errors.As(err, &bad):
		message := bad.Message
		// An unknown act can contain caller-supplied paths; keep those on stderr.
		if strings.HasPrefix(message, "unknown signal act ") && strings.ContainsAny(message, `/\`) {
			message = "unknown signal act"
		}
		signalErrorJSON(env, asJSON, "invalid", message, CodeInvalidArgument)
		return 2
	case errors.As(err, &refused):
		signalErrorJSON(env, asJSON, "refused", refused.Message, errorCode(err))
		return 3
	default:
		return code
	}
}

func signalErrorJSON(env Env, asJSON bool, kind, message, code string) {
	if asJSON {
		writeJSON(env.Stdout, struct {
			Error   string `json:"error"`
			Message string `json:"message"`
			Code    string `json:"code,omitempty"`
		}{kind, message, code})
	}
}
