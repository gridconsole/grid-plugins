// Package braincmd implements the brain command group independently of the
// shared CLI dispatcher and its environment type.
package braincmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

type Env struct {
	Resolver            where.Resolver
	Stdin               io.Reader
	Stdout, Stderr      io.Writer
	Cwd, Home, GridHome string
	TempRoots           []string
	// Role is the caller's GRID_ROLE, read from the CLI's environment.
	Role string
	// Member is the person the host runs the plugin for: GRID_WHO, else
	// GRID_ACTOR, else GRID_MEMBER, read by the CLI. drop and restore name this
	// person in the audit line; no flag lets a caller name someone else.
	Member string
	// Task, Job and Session are the caller's GRID_TASK, GRID_JOB and session id
	// (GRID_SESSION_ID, else CLAUDE_SESSION_ID), read by the CLI. propose names
	// the Job as its author when it runs in one, else the session; see
	// cbAuProposeOrigin. braincmd reads no environment variable itself.
	Task, Job, Session string
	// SessionTask is the session's task as the hooks read it: GRID_TASK, else
	// GRID_FEED_TASK. With no --project or --task it picks the project.
	SessionTask string
	// The CLI supplies its shared folder resolver and refusal contract.
	ResolveTask      func(string) (task.Task, error)
	TaskOperandError func(string, error) int
	// RecordError, when set, is told of a failure that names a project or task
	// the caller cannot see, so the CLI can answer --json with its standard error
	// envelope. Such a failure prints the plain line even under --json; every
	// other failure keeps the JSON object on stderr.
	RecordError func(error)
}

const usage = `Usage: grid brain <show|search|topics|index|propose|accept|keep|decide|check|changes|revert|drop|restore|write|rename|supersede> [flags]
  show <id> [--task <task>]  Show an accepted note and its links.
  search <words> [--task <task>] Find notes by description, summary and body.
  topics                     List built-in and custom topics.
  index                      Rebuild the SessionStart index and backlinks.
  propose --name <id> --topic <topic> --description <when> --summary <what> --source <source> --body <text> [--task <task>]
  keep [--task <task>]         List waiting proposals without writing the brain.
  keep --name <id> --topic <topic> --description <when> --summary <what> --source <source> --body <text> --link <JSON>
                             Record a proposal (same flags as propose); the curator decides it.
  keep accept <id>           File the proposal as a linked note (same as accept <id>).
  keep refuse <id> [--why <reason>]  Record a refusal (same as accept <id> --reject).
  decide <task> <proposal-id> accept|reject|merge-into [<note-id>] [--note <reason>] [--link <json>]...
  accept <id>                File a proposal through the keeper gates.
  accept <id> --reject <why>  Record a rejection.
  check [<proposal-id>]      Read-only gate: flag proposals, skills and agents that change what runs.
                             Exit 0 clean, 3 findings, 1 unreadable brain.
  changes [--since <RFC3339>] [--limit <n>]  List what changed in the brain, newest first.
  revert <topic>/<name> --to <n> [--reason <why>]  Publish version n of an entry again as a new version; the reason shows in changes.
                             Exit 0 done, 1 store error or no such version, 3 role refused.
  drop <topic>/<name> --expect <rev> [--reason <why>]  Hide a knowledge entry from search, the index and job sources; nothing is deleted.
                             Exit 0 done, 1 store error, 2 usage, 3 role refused, 6 stale rev, 8 no such note, 9 refused.
  restore <topic>/<name> --expect <rev> [--reason <why>]  Bring a dropped entry back.
                             Exit 0 done, 1 store error, 2 usage, 3 role refused, 6 stale rev, 8 no such note, 9 refused.
  write <topic>/<name> --expect <16hex|none> --file <path|-> [--reason <why>]  Create or replace a knowledge entry as a new version; none means it must not exist.
                             Exit 0 done, 1 store error, 2 usage or a bad note, 3 role refused, 6 stale rev, 7 not versioned, 9 refused.
  rename <topic>/<name> <new-name> --expect <rev> [--reason <why>]  Rename a versioned knowledge entry and relink the entries of its level that name it.
                             Exit 0 done (entries whose relink failed are listed as kept), 1 store error, 2 usage, 3 role refused, 6 stale rev, 7 not versioned, 8 no such note, 9 refused.
  supersede <old> --by <new> --reason <why>  Mark an accepted note replaced by another, after the fact.
                             Exit 0 done or already done, 1 refused or store error, 2 usage, 3 role refused.

<task> is an id, exact folder name, or folder path inside a project tasks directory.

Common: --json, --project <name|id>, --level home|project
Show: --proposal             Read a proposal instead of an accepted note.
Search: --topic <name>, --limit <n> (default 5)
Propose: --file <Markdown|->, --link <JSON> (repeatable), --external
Index: --read-only           Render without updating generated files.
Unqualified note ids prefer the project; brain:home/<id> selects a personal note.
Proposals use the project level when a project is resolved, otherwise home.
With --task, propose writes <task folder>/brain-proposals/<name>.md, where decide reads it, and adds a history line.
`

type linkFlags []brain.Link

func (l *linkFlags) String() string { return "" }
func (l *linkFlags) Set(raw string) error {
	var link brain.Link
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if !json.Valid([]byte(raw)) {
		return fmt.Errorf("link must be a JSON object")
	}
	if err := d.Decode(&link); err != nil {
		return err
	}
	if err := link.Validate(); err != nil {
		return err
	}
	*l = append(*l, link)
	return nil
}

// Run returns 0 on success, 1 for a store/gate error, 2 for usage errors and 3
// when check finds something, changes and revert refuse the caller's role, or the
// accept-time check refuses a proposal. drop, restore, write and rename also
// return 6 for a stale --expect rev and 9 when the note's state refuses the
// operation, drop, restore and rename 8 for no such note, and write and rename 7
// when the note has no stored version, with the failure as JSON on stderr. None
// of them returns 4 or 5; drop and restore never return 7, write never 8.
func Run(args []string, env Env) int {
	if env.Resolver.GridHome == "" {
		env.Resolver.GridHome = env.GridHome
		env.Resolver.Home = env.Home
		env.Resolver.TempRoots = env.TempRoots
	}
	if env.ResolveTask == nil {
		env.ResolveTask = func(operand string) (task.Task, error) {
			return (task.Store{Resolver: env.Resolver}).Show(env.Cwd, operand)
		}
	}
	if env.TaskOperandError == nil {
		env.TaskOperandError = func(command string, err error) int {
			fmt.Fprintf(env.Stderr, "%s: %v\n", command, err)
			return 1
		}
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(env.Stdout, usage)
		return 0
	}
	verb := args[0]
	if verb == "decide" {
		return runDecide(args[1:], env)
	}
	if verb == "changes" {
		return runChanges(args[1:], env)
	}
	if verb == "revert" {
		return runRevert(args[1:], env)
	}
	if verb == "drop" {
		return runDrop(args[1:], env)
	}
	if verb == "restore" {
		return runRestore(args[1:], env)
	}
	if verb == "write" {
		return runWrite(args[1:], env)
	}
	if verb == "rename" {
		return runRename(args[1:], env)
	}
	if verb == "supersede" {
		return runSupersede(args[1:], env)
	}
	if verb == "keep" {
		args = keepArgs(args)
		verb = args[0]
	}
	switch verb {
	case "show", "search", "topics", "index", "propose", "accept", "keep", "check":
	default:
		fmt.Fprintf(env.Stderr, "brain: unknown command %q. Run: grid brain help\n", verb)
		return 2
	}
	f := flag.NewFlagSet("grid brain "+verb, flag.ContinueOnError)
	f.SetOutput(helpOut(args, env))
	f.Usage = func() {
		out := f.Output()
		switch verb {
		case "show":
			fmt.Fprintln(out, "Usage: grid brain show <id> [--task <task>] [flags]")
		case "search":
			fmt.Fprintln(out, "Usage: grid brain search <words> [--task <task>] [flags]")
		case "keep":
			fmt.Fprintln(out, "Usage: grid brain keep [--task <task>] [flags]")
		default:
			fmt.Fprint(out, usage)
			return
		}
		fmt.Fprintln(out, "<task> is an id, exact folder name, or folder path inside a project tasks directory.")
		f.PrintDefaults()
	}
	asJSON := f.Bool("json", false, "print JSON")
	project := f.String("project", "", "project name or id")
	level := f.String("level", "", "home or project")
	var topic, name, description, summary, source, body, file, reject, taskID, taskDir string
	var links linkFlags
	var proposeTask task.Task // set when --task resolved
	var proposal, external, readOnly bool
	limit := 5
	proposeFlags := func() {
		f.StringVar(&name, "name", "", "note name")
		f.StringVar(&topic, "topic", "", "topic")
		f.StringVar(&description, "description", "", "when to read the note")
		f.StringVar(&summary, "summary", "", "what the note contains")
		f.StringVar(&source, "source", "", "source task, Job or person")
		f.StringVar(&body, "body", "", "Markdown body (otherwise stdin)")
		f.StringVar(&file, "file", "", "Markdown with front matter, or - for stdin")
		f.BoolVar(&external, "external", false, "quarantine externally sourced text")
		f.Var(&links, "link", "link object with to, rel, summary and why")
	}
	switch verb {
	case "keep":
		f.StringVar(&taskID, "task", "", "include proposals from this task")
		// With --name or --file, keep records a proposal as propose does.
		proposeFlags()
	case "show":
		f.StringVar(&taskID, "task", "", "resolve this task")
		f.BoolVar(&proposal, "proposal", false, "show a proposal")
	case "search":
		f.StringVar(&taskID, "task", "", "resolve this task")
		f.StringVar(&topic, "topic", "", "topic filter")
		f.IntVar(&limit, "limit", 5, "maximum hits (1 to 100)")
	case "index":
		f.BoolVar(&readOnly, "read-only", false, "do not write generated files")
	case "propose":
		f.StringVar(&taskID, "task", "", "write the proposal in this task's folder, where decide reads it")
		proposeFlags()
	case "accept":
		f.StringVar(&reject, "reject", "", "reject the proposal with a reason")
	}
	ordered, err := interspersed(f, args[1:])
	if err != nil {
		fmt.Fprintf(env.Stderr, "brain: %v\n", err)
		return 2
	}
	if err := f.Parse(ordered); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if verb == "keep" && (name != "" || file != "") {
		verb = "propose"
	}
	usageError := func(msg string) int {
		fmt.Fprintf(env.Stderr, "brain %s: %s. Run: grid brain help\n", verb, msg)
		return 2
	}
	if *level != "" && *level != "home" && *level != "project" {
		return usageError("level must be home or project")
	}
	if *level == "home" && taskID != "" {
		return usageError("--level home cannot be combined with --task")
	}
	if *level == "home" && *project != "" {
		return usageError("--level home cannot be combined with --project")
	}
	switch verb {
	case "show", "accept":
		if f.NArg() != 1 {
			return usageError("expected one id")
		}
	case "check":
		if f.NArg() > 1 {
			return usageError("expected at most one proposal id")
		}
	case "search":
		if f.NArg() == 0 || strings.TrimSpace(strings.Join(f.Args(), " ")) == "" {
			return usageError("expected search words")
		}
	default:
		if f.NArg() != 0 {
			return usageError("unexpected positional argument")
		}
	}
	if limit < 1 || limit > 100 {
		return usageError("limit must be between 1 and 100")
	}
	if file != "" && body != "" {
		return usageError("choose --file or --body")
	}
	fail := func(err error) int { return failWith(env, *asJSON, err) }
	if env.GridHome == "" {
		return fail(fmt.Errorf("cannot find the Grid home; set GRID_HOME"))
	}
	store := brain.Store{GridHome: env.GridHome, ReadLevel: *level}
	if *level != "home" {
		resolver := env.Resolver
		if taskID != "" {
			t, err := env.ResolveTask(taskID)
			if err != nil {
				return env.TaskOperandError("brain", err)
			}
			proposeTask, taskID = t, t.Path
		}
		res, err := resolver.Resolve(where.Query{Cwd: env.Cwd, Project: *project, Task: taskID})
		if *project == "" && taskID == "" && env.SessionTask != "" {
			// A Job session runs from a worktree that names no task; its task, the
			// one the hooks record against, picks the project. It does not pick a
			// task folder: only --task does that.
			if s, serr := resolver.Resolve(where.Query{Cwd: env.Cwd, Task: env.SessionTask}); serr == nil && (err != nil || s.Project != res.Project) {
				s.TaskPath = ""
				res, err = s, nil
			}
		}
		if err != nil {
			return fail(err)
		}
		taskDir = res.TaskPath
		if res.Rule != where.RulePersonal && res.Project != where.Personal {
			store.Project = res.Project
			writesProject := verb == "propose" || (verb == "accept" && !strings.HasPrefix(f.Arg(0), "brain:home/")) || (verb == "index" && !readOnly)
			if !res.Record && res.Rule != where.RuleTask && writesProject {
				return fail(fmt.Errorf("project %q has no Grid record; create its first task or use --level home", res.Project))
			}
		}
	}
	writeLevel := *level
	if writeLevel == "" {
		writeLevel = "home"
		if store.Project != "" {
			writeLevel = "project"
		}
	}
	if writeLevel == "project" && store.Project == "" {
		return fail(fmt.Errorf("project level needs a project; select --project or use --level home"))
	}
	var result any
	switch verb {
	case "check":
		if f.NArg() == 1 {
			var id string
			id, writeLevel, err = proposalID(f.Arg(0), writeLevel, *level)
			if err == nil {
				result, err = store.CheckProposal(writeLevel, id)
			}
		} else {
			result, err = store.Check()
		}
	case "keep":
		result, err = store.Waiting(taskDir)
	case "show":
		id := f.Arg(0)
		if proposal {
			// With --task, a plain id or task:brain-proposals/<name>.md is a
			// proposal in the task's own folder, where decide reads it. A brain:
			// id is always a level proposal, and a plain id the task folder lacks
			// is read from the level store as before.
			if name, prefixed, ok := taskProposalName(id); ok && taskID != "" && taskDir != "" {
				var entry brain.Entry
				entry, err = taskProposal(taskDir, name)
				if err == nil {
					result = entry
					break
				}
				if prefixed || !errors.Is(err, fs.ErrNotExist) {
					break
				}
			}
			id, writeLevel, err = proposalID(id, writeLevel, *level)
			if err == nil {
				result, err = store.Proposal(writeLevel, id)
			}
		} else {
			if strings.HasPrefix(id, "brain:") {
				_, _, err = proposalID(id, writeLevel, *level)
				if err != nil {
					break
				}
			}
			if *level != "" && !strings.HasPrefix(id, "brain:") {
				id = "brain:" + *level + "/" + id
			}
			result, err = store.Show(id)
		}
	case "search":
		var hits []brain.Hit
		hits, err = store.Search(strings.Join(f.Args(), " "), topic, limit)
		result = struct {
			Query string      `json:"query"`
			Hits  []brain.Hit `json:"hits"`
		}{strings.Join(f.Args(), " "), hits}
	case "topics":
		result, err = store.Topics()
	case "index":
		if readOnly {
			result, err = store.Index()
		} else {
			result, err = store.RebuildIndex()
		}
	case "propose":
		n := brain.Note{}
		if file != "" {
			var b []byte
			if file == "-" {
				b, err = readInput(env.Stdin)
			} else {
				var input *os.File
				input, err = os.Open(file)
				if err == nil {
					b, err = readInput(input)
					input.Close()
				}
			}
			if err == nil {
				n, err = brain.Parse(b)
			}
		} else if body == "" {
			var b []byte
			b, err = readInput(env.Stdin)
			body = string(b)
		}
		if err != nil {
			break
		}
		// Supplied note text can never select the write scope or the author.
		n.Level = writeLevel
		if name != "" {
			n.Name = name
		}
		if topic != "" {
			n.Topic = topic
		}
		if description != "" {
			n.Description = description
		}
		if summary != "" {
			n.Summary = summary
		}
		if source != "" {
			n.Source = source
		}
		if file == "" {
			n.Body = body
		}
		if len(links) > 0 {
			n.Links = links
		}
		var origin brain.Origin
		origin, err = cbAuProposeOrigin(env)
		if external {
			origin.Kind = "import"
		}
		if err == nil && proposeTask.Path != "" {
			// The task's own folder is where decide and deliver look.
			tasks := task.Store{Resolver: env.Resolver}
			history := func(text string) error {
				_, err := tasks.Note(task.WriteOptions{Cwd: proposeTask.Path, ID: proposeTask.ID, Note: text, Who: contract.Author{ID: "grid-brain", Role: "agent"}})
				return err
			}
			result, err = store.ProposeTask(brain.TaskRef{ID: proposeTask.ID, Path: proposeTask.Path}, n, origin, history)
		} else if err == nil {
			result, err = store.Propose(n, origin)
		}
	case "accept":
		var id string
		id, writeLevel, err = proposalID(f.Arg(0), writeLevel, *level)
		if err == nil {
			if reject != "" {
				result, err = store.Reject(writeLevel, id, reject)
			} else {
				result, err = store.Accept(writeLevel, id)
			}
		}
	}
	if err != nil {
		return fail(err)
	}
	exit := 0
	if checked, ok := result.(brain.CheckResult); ok && len(checked.Findings) > 0 {
		exit = 3
	}
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(result); err != nil {
			return fail(err)
		}
	} else if rows, ok := result.([]brain.WaitingProposal); ok {
		for _, row := range rows {
			fmt.Fprintf(env.Stdout, "%s -> %s: %s\n", row.Path, row.Topic, row.Reason)
		}
	} else if checked, ok := result.(brain.CheckResult); ok {
		plainCheck(env.Stdout, checked)
	} else if err := plain(env.Stdout, result); err != nil {
		return fail(err)
	}
	return exit
}

// keepArgs maps the curator's two decisions onto accept: `keep accept <id>` is
// accept, `keep refuse <id> [--why <reason>]` is accept --reject. `keep` with
// --name or --file records a proposal as propose does (see Run); bare `keep`
// lists.
func keepArgs(args []string) []string {
	rest := args[1:]
	if len(rest) > 0 && rest[0] == "accept" {
		return append([]string{"accept"}, rest[1:]...)
	}
	if len(rest) > 0 && rest[0] == "refuse" {
		out := []string{"accept"}
		why := false
		for _, a := range rest[1:] {
			switch {
			case a == "--why" || a == "-why":
				a, why = "--reject", true
			case strings.HasPrefix(a, "--why=") || strings.HasPrefix(a, "-why="):
				a, why = "--reject="+a[strings.Index(a, "=")+1:], true
			}
			out = append(out, a)
		}
		if !why {
			out = append(out, "--reject", "Refused by the curator.")
		}
		return out
	}
	return args
}

func readInput(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, nil
	}
	b, err := io.ReadAll(io.LimitReader(r, (1<<20)+1))
	if err == nil && len(b) > 1<<20 {
		err = fmt.Errorf("input exceeds 1 MiB")
	}
	return b, err
}

// taskProposalName names the task-folder proposal an id can mean: the plain id,
// or task:brain-proposals/<name>.md as keep prints it (prefixed). A brain: id is
// never one. The name is checked when the proposal is read.
func taskProposalName(id string) (name string, prefixed, ok bool) {
	if rest, found := strings.CutPrefix(id, "task:brain-proposals/"); found {
		name, found = strings.CutSuffix(rest, ".md")
		return name, true, found
	}
	if strings.Contains(id, ":") {
		return "", false, false
	}
	return id, false, true
}

// taskProposal reads <task folder>/brain-proposals/<name>.md as decide does:
// through the task folder's root, parsed strictly. The entry has the id and the
// task: path that propose --task prints for the same proposal.
func taskProposal(taskDir, name string) (brain.Entry, error) {
	if !brain.ValidName(name) {
		return brain.Entry{}, fmt.Errorf("proposal id must be a plain name")
	}
	root, err := os.OpenRoot(taskDir)
	if err != nil {
		return brain.Entry{}, err
	}
	defer root.Close()
	rel := filepath.Join("brain-proposals", name+".md")
	b, err := safefile.Read(root, rel)
	if err != nil {
		return brain.Entry{}, err
	}
	n, err := brain.Parse(b)
	if err != nil {
		return brain.Entry{}, err
	}
	if n.Name != name {
		return brain.Entry{}, fmt.Errorf("proposal name does not match its location")
	}
	return brain.Entry{ID: "brain:" + n.Level + "/" + n.Name, Path: "task:" + filepath.ToSlash(rel), Note: n}, nil
}

func proposalID(id, fallback, explicit string) (string, string, error) {
	if strings.HasPrefix(id, "brain:") {
		level, name, ok := strings.Cut(strings.TrimPrefix(id, "brain:"), "/")
		if !ok || (level != "home" && level != "project") || !brain.ValidName(name) {
			return "", "", fmt.Errorf("invalid brain id")
		}
		if explicit != "" && explicit != level {
			return "", "", fmt.Errorf("id level disagrees with --level")
		}
		return name, level, nil
	}
	return id, fallback, nil
}

// flag.FlagSet stops at the first positional argument. Reordering known flags
// permits the usual "show <id> --json" syntax without losing quoted values.
func interspersed(f *flag.FlagSet, args []string) ([]string, error) {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			positional = append(positional, a)
			continue
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		fl := f.Lookup(name)
		if fl == nil {
			if name == "h" || name == "help" {
				return []string{"-h"}, nil
			}
			return nil, fmt.Errorf("unknown flag --%s", name)
		}
		options = append(options, a)
		isBool := false
		if b, ok := fl.Value.(interface{ IsBoolFlag() bool }); ok {
			isBool = b.IsBoolFlag()
		}
		if !hasValue && !isBool {
			i++
			if i == len(args) {
				return nil, fmt.Errorf("flag --%s needs a value", name)
			}
			options = append(options, args[i])
		}
	}
	return append(append(options, "--"), positional...), nil
}

func plain(w io.Writer, value any) error {
	// The JSON shape is also the plain shape: one dotted field per fact. Newlines
	// within a scalar are escaped to keep each fact on one physical line. The
	// SessionStart text is the index's primary plain output.
	if idx, ok := value.(brain.Index); ok {
		levels := []string{"project", "home"}
		for _, level := range levels {
			if generation, ok := idx.Generations[level]; ok {
				if _, err := fmt.Fprintf(w, "generations.%s: %d\n", level, generation); err != nil {
					return err
				}
			}
		}
		_, err := io.WriteString(w, idx.Text)
		return err
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var data any
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	return facts(w, "", data)
}

// plainCheck fences every quoted text as data, so text from a proposal or
// skill can never read as the command's own output.
func plainCheck(w io.Writer, r brain.CheckResult) {
	fmt.Fprintf(w, "checked: %d\nfindings: %d\n", r.Checked, len(r.Findings))
	for _, f := range r.Findings {
		where := f.File
		if f.Line > 0 {
			where = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(w, "%s %s: %s\n", f.Code, where, f.Message)
		if f.Text != "" {
			fence := "```"
			for strings.Contains(f.Text, fence) {
				fence += "`"
			}
			fmt.Fprintf(w, "%s\n%s\n%s\n", fence, f.Text, fence)
		}
	}
}
