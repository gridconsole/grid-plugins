package contract

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// heMvDoc is the part of contract/commands.json this walk reads.
type heMvDoc struct {
	Commands []struct {
		Name        string `json:"name"`
		Subcommands []struct {
			Name string `json:"name"`
		} `json:"subcommands"`
	} `json:"commands"`
}

// heMvPair is one `grid <verb> <sub>` found in a Go string literal.
type heMvPair struct {
	File string // slash path under the walk root
	Line int
	Verb string
	Sub  string
}

// heMvProse lists the literals that name a `grid <verb> <sub>` that
// contract/commands.json does not have. Each key is "file:verb sub". The walk
// reports every pair it finds that is neither a real sub nor listed here, and
// TestHeMvProseListBothWays fails an entry that no literal names any more or
// that has become a real sub.
var heMvProse = map[string]string{
	// A dispatcher answers the sub, commands.json lists none.
	"internal/braincmd/changes.go:brain help":   "answered at braincmd/command.go:112, commands.json lists no help sub",
	"internal/braincmd/command.go:brain help":   "answered at braincmd/command.go:112, commands.json lists no help sub",
	"internal/braincmd/supersede.go:brain help": "answered at braincmd/command.go:112, commands.json lists no help sub",
	"internal/cli/cli.go:task help":             "answered by runTask in cli/task.go (args[0] == \"help\"), commands.json lists no help sub",
	"internal/cli/cli.go:brain help":            "the brain summary points at the dispatcher's help answered at braincmd/command.go:128, commands.json lists no help sub",
	"internal/cli/agent.go:agent budget":        "dispatched at cli/agent.go:21, commands.json lists no budget sub",
	"internal/cli/budget.go:agent budget":       "dispatched at cli/agent.go:21, commands.json lists no budget sub",
	"internal/cli/budget.go:claude settings":    "dispatched at cli/budget.go:90, commands.json lists no settings sub",
}

var (
	heMvCallRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./-])grid ([a-z][a-z0-9-]*) (\S+)`)
	heMvWordRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// heMvExpand returns the `grid <verb> <sub>` pairs one string names. An
// alternation `a|b|c` in the sub place gives one pair per word. A sub place
// that is a placeholder, a flag or a quoted value gives no pair. Pair.Line is
// the count of newlines before the match.
func heMvExpand(lit string) []heMvPair {
	var out []heMvPair
	for _, m := range heMvCallRe.FindAllStringSubmatchIndex(lit, -1) {
		verb := lit[m[2]:m[3]]
		line := strings.Count(lit[:m[0]], "\n")
		for _, part := range strings.Split(lit[m[4]:m[5]], "|") {
			word := strings.TrimRight(part, ".,;:!?)\"'`")
			if !heMvWordRe.MatchString(word) {
				break
			}
			out = append(out, heMvPair{Line: line, Verb: verb, Sub: word})
			if word != part {
				break // punctuation ended the sentence
			}
		}
	}
	return out
}

// heMvSubs maps each verb of commands.json that has named subs to its sub
// names. A verb with no named sub is left out, so the walk skips it.
func heMvSubs(doc heMvDoc) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, c := range doc.Commands {
		for _, s := range c.Subcommands {
			if s.Name == "" {
				continue
			}
			if out[c.Name] == nil {
				out[c.Name] = map[string]bool{}
			}
			out[c.Name][s.Name] = true
		}
	}
	return out
}

func heMvLoad(t *testing.T) map[string]map[string]bool {
	t.Helper()
	b, err := os.ReadFile("../../contract/commands.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc heMvDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	return heMvSubs(doc)
}

// heMvWalk parses every non-test .go file under root/dir for each dir and
// returns the pairs its string literals name for the verbs in subs. Comments
// are not read. Files are returned in path order.
func heMvWalk(subs map[string]map[string]bool, root string, dirs ...string) ([]heMvPair, error) {
	var out []heMvPair
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == "testdata" || d.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			ast.Inspect(f, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				val, err := strconv.Unquote(lit.Value)
				if err != nil {
					val = lit.Value
				}
				raw := strings.HasPrefix(lit.Value, "`")
				start := fset.Position(lit.Pos()).Line
				for _, p := range heMvExpand(val) {
					if subs[p.Verb] == nil {
						continue
					}
					p.File = rel
					if raw {
						p.Line += start
					} else {
						p.Line = start
					}
					out = append(out, p)
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func heMvKey(p heMvPair) string { return p.File + ":" + p.Verb + " " + p.Sub }

func heMvUnknown(subs map[string]map[string]bool, pairs []heMvPair) []heMvPair {
	var out []heMvPair
	for _, p := range pairs {
		if !subs[p.Verb][p.Sub] {
			out = append(out, p)
		}
	}
	return out
}

func TestHeMvMessagesNameRealSubs(t *testing.T) {
	subs := heMvLoad(t)
	pairs, err := heMvWalk(subs, "../..", "cmd", "internal")
	if err != nil {
		t.Fatal(err)
	}
	distinct := map[string]bool{}
	for _, p := range pairs {
		distinct[heMvKey(p)] = true
	}
	unknown := heMvUnknown(subs, pairs)
	listed := map[string]bool{}
	for _, p := range unknown {
		if _, ok := heMvProse[heMvKey(p)]; ok {
			listed[heMvKey(p)] = true
			continue
		}
		t.Errorf("%s:%d: `grid %s %s`: %q is not a sub of `grid %s` in contract/commands.json; fix the message, or list %q in heMvProse if it is prose",
			p.File, p.Line, p.Verb, p.Sub, p.Sub, p.Verb, heMvKey(p))
	}
	t.Logf("%d pairs found (%d distinct file:pair) in the non-test Go files under cmd/ and internal/; %d verbs with subs checked; %d pairs are listed prose",
		len(pairs), len(distinct), len(subs), len(listed))
	if len(pairs) < 20 {
		t.Fatalf("found only %d pairs; the scan is broken", len(pairs))
	}
	// The feed usage literal names state; state is a real sub of grid feed.
	state := false
	for _, p := range pairs {
		if p.File == "internal/cli/feed.go" && p.Verb == "feed" && p.Sub == "state" {
			state = true
		}
	}
	if !state {
		t.Errorf("no `grid feed state` pair in internal/cli/feed.go; the usage literal should name it")
	}
	if !subs["feed"]["state"] {
		t.Errorf("`state` is not a sub of `grid feed` in contract/commands.json")
	}
}

func TestHeMvAlternation(t *testing.T) {
	cases := []struct {
		lit  string
		want []string
	}{
		{"usage: grid feed post|read|fold [flags]", []string{"feed post", "feed read", "feed fold"}},
		{"grid plan <task>", nil},
		{"grid plan", nil},
		{"grid feed --json", nil},
		{"grid feed post|read|state|", []string{"feed post", "feed read", "feed state"}},
		{"run `grid job ack`.", []string{"job ack"}},
		{"grid job ack, then grid job result.", []string{"job ack", "job result"}},
		{"grid brain <changes|revert>: no", nil},
		{"grid feed post|<kind>", []string{"feed post"}},
		{"Usage: grid eval list [flags]\n       grid eval run [task]", []string{"eval list", "eval run"}},
		{"mygrid job ack and grid-job ack and /bin/grid job ack", nil},
		{"grid job ack_x", nil},
	}
	for _, c := range cases {
		var got []string
		for _, p := range heMvExpand(c.lit) {
			got = append(got, p.Verb+" "+p.Sub)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("heMvExpand(%q) = %q, want %q", c.lit, got, c.want)
		}
	}
	if got := heMvExpand("a\nb grid job ack"); len(got) != 1 || got[0].Line != 1 {
		t.Errorf("line offset of a match on the second line: %+v", got)
	}
}

func TestHeMvFinder(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "pkg", name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.go", "package pkg\n\n"+
		"// grid feed commentonly is in a comment, not a literal.\n"+
		"var real = \"run grid feed post\"\n"+
		"var unknown = \"usage: grid feed frobnicate <x>\"\n"+
		"var noSubs = \"grid version check\"\n"+
		"var raw = `first\nsecond grid feed rawunknown`\n")
	write("a_test.go", "package pkg\n\nvar other = \"grid feed zork\"\n")
	subs := map[string]map[string]bool{"feed": {"post": true, "read": true}}

	pairs, err := heMvWalk(subs, root, "pkg")
	if err != nil {
		t.Fatal(err)
	}
	var seen []string
	for _, p := range pairs {
		seen = append(seen, p.File+":"+strconv.Itoa(p.Line)+" "+p.Verb+" "+p.Sub)
	}
	wantSeen := []string{
		"pkg/a.go:4 feed post",
		"pkg/a.go:5 feed frobnicate",
		"pkg/a.go:8 feed rawunknown", // the raw literal starts on line 7
	}
	if strings.Join(seen, "|") != strings.Join(wantSeen, "|") {
		t.Fatalf("walk saw %q, want %q", seen, wantSeen)
	}
	unknown := heMvUnknown(subs, pairs)
	if len(unknown) != 2 || unknown[0].Sub != "frobnicate" || unknown[1].Sub != "rawunknown" {
		t.Fatalf("unknown = %+v, want frobnicate and rawunknown from pkg/a.go only", unknown)
	}
	for _, p := range unknown {
		if p.File != "pkg/a.go" {
			t.Errorf("unknown pair from %s, want pkg/a.go (the _test.go file is not read)", p.File)
		}
	}
}

func TestHeMvProseListBothWays(t *testing.T) {
	subs := heMvLoad(t)
	pairs, err := heMvWalk(subs, "../..", "cmd", "internal")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, p := range pairs {
		found[heMvKey(p)] = true
	}
	keys := make([]string, 0, len(heMvProse))
	for k := range heMvProse {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if heMvProse[k] == "" {
			t.Errorf("heMvProse entry %q has no reason", k)
		}
		if !found[k] {
			t.Errorf("heMvProse entry %q matches no string literal; remove it", k)
		}
		_, pair, ok := strings.Cut(k, ":")
		verb, sub, ok2 := strings.Cut(pair, " ")
		if !ok || !ok2 {
			t.Errorf("heMvProse entry %q is not file:verb sub", k)
			continue
		}
		if subs[verb][sub] {
			t.Errorf("heMvProse entry %q is a real sub of `grid %s` now; remove it", k, verb)
		}
	}
}
