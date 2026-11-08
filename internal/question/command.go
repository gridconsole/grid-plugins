package question

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/where"
)

type Env struct {
	Cwd            string
	Resolver       where.Resolver
	Stdout, Stderr io.Writer
	Now            func() time.Time
	Writer         func(bool) (feed.Writer, error)
}

type options []string

func (o *options) String() string     { return strings.Join(*o, ", ") }
func (o *options) Set(v string) error { *o = append(*o, v); return nil }

// Run is the command adapter; the store is also callable by hooks and workers.
func Run(args []string, env Env) int {
	if len(args) > 0 && args[0] == "answer" {
		fmt.Fprintln(env.Stderr, "question: unknown action \"answer\"")
		return 2
	}
	return run(args, env)
}

// RunAnswer is the sole answer command, with an explicit task operand.
func RunAnswer(args []string, env Env) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(env.Stdout, `Usage: grid answer <task> Q-N "text" [--json]`)
		return 0
	}
	if len(args) < 3 {
		fmt.Fprintln(env.Stderr, `Usage: grid answer <task> Q-N "text" [--json]`)
		return 2
	}
	return run(append([]string{"answer", "--task", args[0]}, args[1:]...), env)
}

func run(args []string, env Env) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(env.Stdout, "Usage: grid question add|withdraw|list|tick|sync [--task <path>] [--json]")
		return 0
	}
	if len(args) == 0 {
		fmt.Fprintln(env.Stderr, "Usage: grid question add|withdraw|list|tick|sync [--task <path>] [--json]")
		return 2
	}
	action := args[0]
	if action != "add" && action != "answer" && action != "withdraw" && action != "list" && action != "tick" && action != "sync" {
		fmt.Fprintf(env.Stderr, "question: unknown action %q\n", action)
		return 2
	}
	name := "grid question " + action
	if action == "answer" {
		name = "grid answer"
	}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	task := fs.String("task", "", "task folder or id; otherwise the enclosing task")
	asJSON := fs.Bool("json", false, "print JSON")
	var in Input
	var opts options
	var applies, reason string
	if action == "add" {
		fs.StringVar(&in.Job, "job", "", "related Job id")
		fs.IntVar(&in.Item, "item", 0, "related plan item number")
		fs.StringVar(&in.Question, "question", "", "question text (or a positional argument)")
		fs.StringVar(&in.Who, "who", "owner", "person asked")
		fs.StringVar(&in.Why, "why", "", "why the answer matters (optional)")
		fs.Var(&opts, "option", "option label; repeat for each option")
		fs.StringVar(&in.Recommended, "recommended", "", "recommended option label")
		fs.StringVar(&in.Default, "default", "", "answer to apply at the deadline")
		fs.StringVar(&applies, "applies", "", "deadline in RFC3339 with timezone")
		fs.BoolVar(&in.Blocking, "blocking", false, "whether this question blocks work")
		fs.DurationVar(&in.AgeAfter, "age-after", DefaultAgeAfter, "raise priority after this interval, at most twice")
	}
	if action == "withdraw" {
		fs.StringVar(&reason, "reason", "No longer needed", "reason for withdrawing")
	}
	// Permit flags before or after operands. -- protects literal option/text
	// values that start with a dash. Values of flags remain adjacent to their flag.
	flags, positionals := []string{}, []string{}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positionals = append(positionals, a)
			continue
		}
		flags = append(flags, a)
		name, _, equals := strings.Cut(strings.TrimLeft(a, "-"), "=")
		f := fs.Lookup(name)
		if f != nil && !equals {
			b, ok := f.Value.(interface{ IsBoolFlag() bool })
			if !ok || !b.IsBoolFlag() {
				if i+1 < len(args) {
					i++
					flags = append(flags, args[i])
				}
			}
		}
	}
	if err := fs.Parse(flags); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	usage := func(msg string) int { fmt.Fprintln(env.Stderr, "question: "+msg); return 2 }
	var id, text string
	switch action {
	case "add":
		if len(positionals) == 1 && in.Question == "" {
			in.Question = positionals[0]
		} else if len(positionals) != 0 {
			return usage("add takes one question")
		}
		var err error
		in.Applies, err = time.Parse(time.RFC3339Nano, applies)
		if err != nil {
			return usage("--applies must be an RFC3339 time with timezone")
		}
		in.Options = opts
		if in.AgeAfter <= 0 {
			return usage("--age-after must be positive")
		}
		if err = validateInput(in); err != nil {
			return usage(err.Error())
		}
	case "answer":
		if len(positionals) != 2 {
			return usage("answer requires <Q-id> <option|text>")
		}
		id, text = positionals[0], positionals[1]
	case "withdraw":
		if len(positionals) != 1 {
			return usage("withdraw requires <Q-id>")
		}
		id = positionals[0]
	case "sync":
		if len(positionals) == 1 && *task == "" {
			*task = positionals[0]
		} else if len(positionals) != 0 {
			return usage("sync takes at most one task")
		}
	default:
		if len(positionals) != 0 {
			return usage(action + " takes no arguments")
		}
	}
	scope, err := env.Resolver.Resolve(where.Query{Cwd: env.Cwd, Task: *task})
	if err == nil && scope.TaskPath == "" {
		err = errors.New("no enclosing task; use --task <path>")
	}
	if err != nil {
		fmt.Fprintln(env.Stderr, "question:", err)
		return 1
	}
	store := Store{TaskDir: scope.TaskPath, Now: env.Now, Warn: env.Stderr}

	if env.Writer != nil && action != "list" && action != "sync" {
		writer, e := env.Writer(action == "withdraw" || action == "tick")
		if e != nil {
			fmt.Fprintln(env.Stderr, "question:", e)
			return 1
		}
		store.Writer = writer
		store.ControlWriter = writer
	}
	var result any
	switch action {
	case "add":
		result, err = store.Add(in)
	case "answer":
		result, err = store.Answer(id, text)
	case "withdraw":
		result, err = store.Withdraw(id, reason)
	case "list":
		result, err = store.List()
	case "sync":
		result, err = store.Sync()
	case "tick":
		result, err = store.Tick()
	}
	if err != nil {
		fmt.Fprintln(env.Stderr, "question:", err)
		return 1
	}
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		if err = enc.Encode(result); err != nil {
			fmt.Fprintln(env.Stderr, err)
			return 1
		}
		return 0
	}
	records, ok := result.([]Record)
	if !ok {
		records = []Record{result.(Record)}
	}
	if len(records) == 0 {
		if action == "tick" {
			fmt.Fprintln(env.Stdout, "No questions changed.")
		} else {
			fmt.Fprintln(env.Stdout, "No questions.")
		}
	}
	for _, r := range records {
		if r.Post == "" {
			fmt.Fprintf(env.Stdout, "%s | not in the Feed\n", r.ID)
			continue
		}
		block := "not blocking"
		if r.Blocking {
			block = "blocking"
		}
		fmt.Fprintf(env.Stdout, "%s | %s | to: %s | %s | aging: %d\n%s\nRecommended: %s\nDefault: %s\nApplies: %s\n", r.ID, r.State, r.Who, block, r.Aging, r.Question, r.Recommended, r.Gate.Default, r.Gate.At.UTC().Format(FeedTime))
		if r.Answer != "" {
			fmt.Fprintf(env.Stdout, "Answer: %s\n", r.Answer)
		}
	}
	return 0
}
