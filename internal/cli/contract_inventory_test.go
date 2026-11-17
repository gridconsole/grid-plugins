package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/version"
)

// The inventory is rebuilt from command dispatch expressions and live parser
// probes. Annotations describe operands and output schemas, which flag.FlagSet
// does not expose; they are never the source of command or flag names.
type contractParserSource struct {
	funcs   map[string]*ast.FuncDecl
	imports map[string]string
}

type contractInventorySource struct {
	t        *testing.T
	packages map[string]contractParserSource
	flags    map[string]bool // candidate names found in parser source
}

func contractLiteral(e ast.Expr) string {
	if b, ok := e.(*ast.BasicLit); ok && b.Kind == token.STRING {
		s, _ := strconv.Unquote(b.Value)
		return s
	}
	return ""
}

var contractFlagName = regexp.MustCompile(`^--[a-z][a-z0-9-]*$`)

func (s *contractInventorySource) pkg(dir string) contractParserSource {
	if p, ok := s.packages[dir]; ok {
		return p
	}
	p := contractParserSource{funcs: map[string]*ast.FuncDecl{}, imports: map[string]string{}}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		s.t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			s.t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path := contractLiteral(imp.Path)
			if !strings.HasPrefix(path, "github.com/gridconsole/grid-plugins/internal/") {
				continue
			}
			name := filepath.Base(path)
			if imp.Name != nil {
				name = imp.Name.Name
			}
			p.imports[name] = filepath.Join("..", "..", strings.TrimPrefix(path, "github.com/gridconsole/grid-plugins/"))
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil {
				p.funcs[fn.Name.Name] = fn
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok {
				name := contractLiteral(lit)
				if contractFlagName.MatchString(name) {
					if _, exists := s.flags[name]; !exists {
						s.flags[name] = false
					}
				}
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			method := sel.Sel.Name
			index := 0
			if strings.HasSuffix(method, "Var") || method == "Var" {
				index = 1
			}
			switch strings.TrimSuffix(method, "Var") {
			case "Bool", "String", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration", "":
				if len(call.Args) > index {
					if name := contractLiteral(call.Args[index]); name != "" {
						s.flags["--"+name] = true
					}
				}
			}
			return true
		})
	}
	s.packages[dir] = p
	return p
}

// Discover the literal alternatives tested against the first argument, following
// CLI delegates (including jobcmd, braincmd, question and evalcmd). This catches
// newly added routes even when the human-facing summary has not changed.
func (s *contractInventorySource) routes(dir, name string, seen map[string]bool) []string {
	key := dir + "/" + name
	if seen[key] {
		return nil
	}
	seen[key] = true
	p := s.pkg(dir)
	fn := p.funcs[name]
	if fn == nil {
		s.t.Fatalf("cannot find parser %s", key)
	}
	aliases := map[string]bool{}
	// Once args is consumed, args[0] denotes an operand, not a route.
	var consumed token.Pos
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok {
			for i, rhs := range a.Rhs {
				if i >= len(a.Lhs) {
					continue
				}
				lhs, ok := a.Lhs[i].(*ast.Ident)
				slice, sliced := rhs.(*ast.SliceExpr)
				if !ok || lhs.Name != "args" || !sliced {
					continue
				}
				id, ok := slice.X.(*ast.Ident)
				if ok && id.Name == "args" && (consumed == 0 || a.End() < consumed) {
					consumed = a.End()
				}
			}
		}
		return true
	})
	first := func(e ast.Expr) bool {
		if e == nil || consumed != 0 && e.Pos() > consumed {
			return false
		}
		idx, ok := e.(*ast.IndexExpr)
		if !ok {
			return false
		}
		n, ok := idx.Index.(*ast.BasicLit)
		if !ok || n.Value != "0" {
			return false
		}
		switch x := idx.X.(type) {
		case *ast.Ident:
			return x.Name == "args"
		case *ast.SelectorExpr:
			return x.Sel.Name == "args"
		}
		return false
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if a, ok := n.(*ast.AssignStmt); ok {
			for i, rhs := range a.Rhs {
				if i < len(a.Lhs) && first(rhs) {
					if id, ok := a.Lhs[i].(*ast.Ident); ok {
						aliases[id.Name] = true
					}
				}
			}
		}
		return true
	})
	selector := func(e ast.Expr) bool {
		id, ok := e.(*ast.Ident)
		return first(e) || ok && aliases[id.Name]
	}
	var names []string
	add := func(e ast.Expr) {
		v := contractLiteral(e)
		if v != "" && v != "help" && !strings.HasPrefix(v, "-") {
			names = append(names, v)
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BinaryExpr:
			if n.Op == token.EQL || n.Op == token.NEQ {
				if selector(n.X) {
					add(n.Y)
				}
				if selector(n.Y) {
					add(n.X)
				}
			}
		case *ast.SwitchStmt:
			if selector(n.Tag) {
				for _, stmt := range n.Body.List {
					for _, e := range stmt.(*ast.CaseClause).List {
						add(e)
					}
				}
			}
		case *ast.CallExpr:
			switch call := n.Fun.(type) {
			case *ast.Ident:
				// These backends receive an already selected route. Usage
				// configuration is expanded at its second dispatch level below;
				// the orchestrator is shared by job and feed, whose wrappers
				// choose which verbs are reachable from each command.
				if strings.HasPrefix(call.Name, "run") && call.Name != "runOrchestrator" && p.funcs[call.Name] != nil {
					names = append(names, s.routes(dir, call.Name, seen)...)
				}
			case *ast.SelectorExpr:
				if id, ok := call.X.(*ast.Ident); ok && call.Sel.Name == "Run" && p.imports[id.Name] != "" {
					names = append(names, s.routes(p.imports[id.Name], "Run", seen)...)
				}
			}
		}
		return true
	})
	return names
}

func contractCommandInventory(t *testing.T) contractCommandDocument {
	t.Helper()
	source := contractInventorySource{t: t, packages: map[string]contractParserSource{}, flags: map[string]bool{}}
	paths := map[string][]string{}
	table := commands()
	for _, c := range table {
		if c.run == nil {
			continue
		}
		fn := runtime.FuncForPC(reflect.ValueOf(c.run).Pointer()).Name()
		fn = fn[strings.LastIndexByte(fn, '.')+1:]
		routes := source.routes(".", fn, map[string]bool{})
		if c.name == "usage" {
			// usage has a flag-only fold besides post and show.
			routes = append(routes, "")
		}
		if len(routes) == 0 {
			routes = []string{""}
		}
		unique := map[string]bool{}
		for _, route := range routes {
			unique[route] = true
		}
		for route := range unique {
			paths[c.name] = append(paths[c.name], route)
		}
		sort.Strings(paths[c.name])
	}
	// runSettings serves agent, skill and hook; follow-through of its shared
	// operation names would invent routes such as "agent enable".
	// runAgentCommand also serves check, sync and budget. The budget routes are listed by hand:
	// runAgentBudget compares its op (show, apply, undo) in budget.go, which the parser-source
	// follow-through cannot see. The app calls "agent budget show".
	paths["agent"] = []string{"budget apply", "budget show", "budget undo", "check", "list", "set", "sync", "unset"}
	paths["skill"] = []string{"disable", "enable", "list"}
	paths["hook"] = []string{"", "events", "level", "levels"}
	// runClaude has one level of its own ("settings") and a second of show and set.
	paths["claude"] = []string{"settings set", "settings show"}
	// "answer" is the top-level command; question.Run keeps no such route.
	paths["question"] = []string{"add", "ask", "list", "sync", "tick", "withdraw"}
	candidates := []string{"--__contract_unknown"}
	for flag := range source.flags {
		if flag != "--help" {
			candidates = append(candidates, flag)
		}
	}
	sort.Strings(candidates)
	h := newHarness(t)
	doc := contractCommandDocument{Contract: version.Contract, Commands: []contractCommandEntry{}}
	annotations := contractCommandAnnotations()
	for _, c := range table {
		entry := contractCommandEntry{Name: c.name, Summary: c.summary, Built: c.run != nil, Subcommands: []contractCommandSub{}}
		for _, path := range paths[c.name] {
			key := strings.TrimSpace(c.name + " " + path)
			annotation, ok := annotations[key]
			if !ok {
				t.Fatalf("new parser route %q needs operand/output annotations", key)
			}
			delete(annotations, key)
			sub := contractCommandSub{Name: path, Args: annotation.args, Flags: []contractCommandFlag{}, Output: annotation.output, Stream: annotation.stream}
			if sub.Args == nil {
				sub.Args = []string{}
			}
			prefix := append([]string{c.name}, strings.Fields(path)...)
			if c.name == "lease" {
				// The only lease left is the task lease; its parser follows the word task.
				prefix = append(prefix, "task")
			}

			// Two unknown flags fence off command execution. A value flag consumes
			// the first sentinel (or rejects it as an invalid value); a boolean flag
			// leaves it for the parser to reject. This also handles IsBoolFlag and
			// hand-written parsers without guessing their arity from help text.
			probeAt := func(prefix []string, flag string) (bool, string) {
				const value = "--__contract_value"
				const stop = "--__contract_stop"
				args := append(append([]string(nil), prefix...), flag, value, stop)
				code := h.run(h.home, args...)
				output := h.errb.String() + h.out.String()
				line, _, _ := strings.Cut(output, "\n")
				unknown := func(name string) bool {
					return strings.Contains(line, "flag provided but not defined: -"+strings.TrimPrefix(name, "--")) || strings.Contains(line, "unknown flag "+name)
				}
				if code != 2 {
					t.Fatalf("%s: unsafe parser probe %s returned %d: %s", key, flag, code, output)
				}
				if unknown(flag) {
					return false, ""
				}
				if unknown(value) {
					return true, "none"
				}
				return true, "value"
			}
			probe := func(flag string) (bool, string) { return probeAt(prefix, flag) }
			if key == "setup" {
				// runSetup picks its parser by operand (settings.ClaudeSetupCommand): no
				// operand is the Claude Code branch, a project operand is the answers
				// branch. List a flag when either branch knows it; the two must agree on
				// what it takes, and both must refuse the unknown-flag probe.
				probe = func(flag string) (bool, string) {
					const project = "contractproject"
					noProject, noProjectTakes := probeAt(prefix, flag)
					withProject, withProjectTakes := probeAt(append(append([]string(nil), prefix...), project), flag)
					if noProject && withProject && noProjectTakes != withProjectTakes {
						t.Fatalf("%s: flag %s takes %q without a project and %q with one", key, flag, noProjectTakes, withProjectTakes)
					}
					if noProject {
						return true, noProjectTakes
					}
					return withProject, withProjectTakes
				}
			}
			// The hook event handler deliberately ignores invalid input and has no flags;
			// hook level and hook levels use the runSettings parser, which the probe can read.
			// answer takes <task> first, so a leading probe flag is swallowed as the task.
			if key == "workflow write" {
				// Its parser prints one fixed usage line for any bad flag, so the unknown-flag probe cannot read arity.
				for _, f := range []struct{ name, takes string }{{"--file", "value"}, {"--json", "none"}, {"--project", "value"}, {"--rev", "value"}} {
					sub.Flags = append(sub.Flags, contractCommandFlag{Name: f.name, Takes: f.takes, Required: annotation.required[f.name]})
				}
				sub.JSON = true
			} else if c.name == "answer" {
				sub.Flags = append(sub.Flags, contractCommandFlag{Name: "--json", Takes: "none"})
				sub.JSON = true
			} else if key != "hook" {
				for _, flag := range candidates {
					recognized, takes := probe(flag)
					if recognized {
						if flag == "--__contract_unknown" {
							t.Fatalf("%s accepts the unknown-flag probe; cannot inspect this parser safely", key)
						}
						sub.Flags = append(sub.Flags, contractCommandFlag{Name: flag, Takes: takes, Required: annotation.required[flag]})
						sub.JSON = sub.JSON || flag == "--json"
					}
				}
			}
			entry.Subcommands = append(entry.Subcommands, sub)
		}
		doc.Commands = append(doc.Commands, entry)
	}
	if len(annotations) != 0 {
		t.Fatalf("annotations for removed parser routes: %v", annotations)
	}
	return doc
}

func TestContractCommandsParserInventory(t *testing.T) {
	doc := contractCommandInventory(t)
	if os.Getenv("UPDATE_CONTRACT_COMMANDS") == "1" {
		contractCommandWrite(t, "../../contract/commands.json", doc)
		return
	}
	// Use the same canonical encoder as fixture generation, then compare the
	// entire document, not only summaries and built status.
	data := contractdata.Commands
	got := contractCommandJSON(t, doc)
	if !bytes.Equal(data, got) {
		t.Fatalf("commands.json differs from the current parsers; run UPDATE_CONTRACT_COMMANDS=1 go test ./internal/cli -run '^TestContractCommandsParserInventory$' -count=1\n%s", contractInventoryDifference(data, got))
	}
}

func contractInventoryDifference(want, got []byte) string {
	a, b := strings.Split(string(want), "\n"), strings.Split(string(got), "\n")
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return fmt.Sprintf("line %d: inventory %s; parser %s", i+1, a[i], b[i])
		}
	}
	return "document length changed"
}

func contractCommandJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(b, '\n')
}

type contractCommandAnnotation struct {
	args     []string
	required map[string]bool
	output   string
	stream   bool
}

func contractCommandAnnotations() map[string]contractCommandAnnotation {
	return map[string]contractCommandAnnotation{
		"version":              {output: "cmd-version.json"},
		"where":                {output: "cmd-where.json"},
		"open":                 {args: []string{"[task]"}},
		"doctor":               {},
		"hook":                 {args: []string{"<event>"}},
		"hook events":          {},
		"hook level":           {args: []string{"<hook>", "<off|warn|block>"}},
		"hook levels":          {},
		"account list":         {output: "cmd-account-status.json"},
		"account status":       {output: "cmd-account-status.json"},
		"account scan":         {output: "cmd-account-scan.json"},
		"account folder":       {required: map[string]bool{"--kind": true, "--id": true}, output: "cmd-account-folder.json"},
		"account windows":      {output: "cmd-account-windows.json"},
		"account note":         {},
		"account sign-in":      {},
		"account statusline":   {output: "cmd-account-statusline.json"},
		"agent list":           {},
		"agent set":            {args: []string{"<agent>"}, output: "cmd-settings-receipt.json"},
		"agent unset":          {args: []string{"<agent>"}},
		"agent check":          {},
		"agent sync":           {},
		"agent budget show":    {output: "cmd-agent-budget-show.json"},
		"agent budget apply":   {args: []string{"<economic|balanced|high>"}},
		"agent budget undo":    {},
		"claude settings show": {output: "cmd-claude-settings-show.json"},
		"claude settings set":  {args: []string{"<key>", "<value>"}, output: "cmd-claude-settings-set.json"},
		"person show":          {args: []string{"<handle>"}},
		"person approvers":     {required: map[string]bool{"--area": true}},
		"person check":         {},
		"skill list":           {output: "cmd-skill-list.json"},
		"skill enable":         {args: []string{"<skill>"}, output: "cmd-settings-receipt.json"},
		"skill disable":        {args: []string{"<skill>"}},
		"task new":             {args: []string{"<words>"}, output: "cmd-task-new.json"},
		"task show":            {args: []string{"[<task>]"}, output: "cmd-task-show.json"},
		"task list":            {output: "cmd-task-list.json"},
		"task status":          {args: []string{"<task>", "<state>"}},
		"task lane":            {args: []string{"<task>", "<lane>"}},
		"task stop":            {args: []string{"<task>"}},
		"task close":           {args: []string{"<task>"}},
		"task reopen":          {args: []string{"<task>"}},
		"task rename":          {args: []string{"<task>"}, required: map[string]bool{"--title": true}},
		"task decline":         {args: []string{"<task>"}, required: map[string]bool{"--reason": true}},
		"task summary":         {args: []string{"<task>"}, required: map[string]bool{"--for": true, "--text": true}},
		"task delete":          {args: []string{"<task>"}, output: "cmd-task-trash.json"},
		"task restore":         {args: []string{"<T-NN>"}, required: map[string]bool{"--project": true}, output: "cmd-task-trash.json"},
		"task history":         {args: []string{"<task>"}},
		"task merge":           {required: map[string]bool{"--task": true}},
		"task attach":          {args: []string{"<task>"}},
		"task move":            {args: []string{"[machine-or-task]"}, output: "cmd-task-move.json"},
		"plan write":           {args: []string{"<file>"}},
		"plan check":           {args: []string{"[T-NN]"}},
		"plan items":           {},
		"plan hash":            {output: "cmd-plan-hash.json"},
		"plan migrate":         {},
		"job next":             {args: []string{"[job]"}, output: "cmd-job-next.json", stream: true},
		"job post":             {args: []string{"[job]"}},
		"job ack":              {args: []string{"[job]"}},
		"job result":           {args: []string{"[job]"}},
		"job cancel":           {args: []string{"[job]"}, required: map[string]bool{"--reason": true}},
		"job tree":             {args: []string{"[job]"}},
		"job blocked":          {args: []string{"[job]"}},
		"job render":           {args: []string{"<task>", "<item>"}},
		"job lint":             {args: []string{"<file>"}},
		"job show":             {args: []string{"[task job | job | file]"}},
		"job cycles":           {args: []string{"<items.json>"}},
		"lease take":           {args: []string{"task"}},
		"lease release":        {args: []string{"task"}},
		"lease status":         {args: []string{"task"}, output: "cmd-lease-status.json"},
		"feed tick":            {args: []string{"[job]"}},
		"feed post":            {args: []string{"[task]"}, required: map[string]bool{"--body": true, "--type": true}, output: "cmd-feed-post.json"},
		"feed read":            {args: []string{"[task]"}},
		"feed state":           {args: []string{"[task]"}, output: "cmd-feed-state.json"},
		"feed types":           {args: []string{"[task]"}},
		"feed handover":        {args: []string{"[task]"}},
		"mcp":                  {},
		"question add":         {args: []string{"[question]"}},
		"question ask":         {args: []string{"[question]"}},
		"question sync":        {args: []string{"[task]"}},
		"answer":               {args: []string{"<task>", "<Q-N>", "<text>"}},
		"question withdraw":    {args: []string{"<Q-id>"}},
		"question list":        {},
		"question tick":        {},
		"approve":              {args: []string{"<task>"}, output: "cmd-approve.json"},
		"deliver":              {args: []string{"[outcome]", "[files...]"}},
		"proof add":            {args: []string{"<item>", "[file]"}},
		"proof index":          {},
		"history add":          {args: []string{"<line>"}},
		"brain show":           {args: []string{"<id>"}},
		"brain search":         {args: []string{"<words...>"}, output: "cmd-brain-search.json"},
		"brain topics":         {},
		"brain list":           {},
		"brain index":          {},
		"brain propose":        {},
		"brain accept":         {args: []string{"<id>"}},
		"brain keep":           {},
		"brain check":          {args: []string{"[proposal-id]"}},
		"brain changes":        {output: "cmd-brain-changes.json"},
		"brain revert":         {args: []string{"<topic>/<name>"}, required: map[string]bool{"--to": true}},
		"brain drop":           {args: []string{"<topic>/<name>"}, required: map[string]bool{"--expect": true}, output: "cmd-brain-drop.json"},
		"brain restore":        {args: []string{"<topic>/<name>"}, required: map[string]bool{"--expect": true}},
		"brain write":          {args: []string{"<topic>/<name>"}, required: map[string]bool{"--expect": true, "--file": true}, output: "cmd-brain-write.json"},
		"brain rename":         {args: []string{"<topic>/<name>", "<new-name>"}, required: map[string]bool{"--expect": true}},
		"brain supersede":      {args: []string{"<old-note>"}, required: map[string]bool{"--by": true, "--reason": true}},
		"brain decide":         {args: []string{"<task>", "<proposal-id>", "<accept|reject|merge-into>", "[note-id]"}},
		"env up":               {args: []string{"<J>"}},
		"env down":             {args: []string{"<J>"}},
		"routine add":          {args: []string{"<name>"}, required: map[string]bool{"--every": true}, output: "cmd-routine-add.json"},
		"routine list":         {output: "cmd-routine-list.json"},
		"routine remove":       {args: []string{"<name>"}},
		"routine run":          {args: []string{"<name>"}},
		"routine schedule":     {args: []string{"[<name>]"}},
		"routine unschedule":   {args: []string{"[<name>]"}},
		"worker list":          {output: "cmd-worker-list.json"},
		"worker next":          {required: map[string]bool{"--pid": true, "--pid-start": true}, stream: true},
		"run":                  {stream: true},
		"session list":         {output: "cmd-session-list.json"},
		"session show":         {args: []string{"<session-id>"}, output: "cmd-session-show.json"},
		"session start":        {output: "cmd-session-start.json"},
		"session stop":         {output: "cmd-session-stop.json"},
		"session send":         {args: []string{"[text]"}, output: "cmd-session-send.json"},
		"session pause":        {output: "cmd-session-pause.json"},
		"session resume":       {output: "cmd-session-resume.json"},
		"session handback":     {output: "cmd-session-handback.json"},
		"session switch":       {args: []string{"[agent]"}, output: "cmd-session-switch.json"},
		"intent list":          {output: "cmd-intent-list.json"},
		"intent claim":         {args: []string{"<id>"}},
		"intent renew":         {args: []string{"<id>"}},
		"intent done":          {args: []string{"<id>"}},
		"intent release":       {args: []string{"<id>"}},
		"usage":                {},
		"project add":          {args: []string{"[path]"}, output: "cmd-project-add.json"},
		"project remove":       {args: []string{"<name-or-id>"}, output: "cmd-project-remove.json"},
		"project list":         {},
		"project scan":         {args: []string{"<root>"}, output: "cmd-project-scan.json"},
		"project relink":       {args: []string{"<name-or-id>", "<path>"}},
		"project env":          {args: []string{"<list|add|edit|remove>", "<project>"}, output: "cmd-project-env.json"},
		"project repo":         {args: []string{"<add|edit|remove>", "<project>"}, output: "cmd-project-repo.json"},
		"update draft":         {args: []string{"<project>"}},
		"update edit":          {args: []string{"<project>"}, required: map[string]bool{"--line": true}},
		"update discard":       {args: []string{"<project>"}},
		"update skip":          {args: []string{"<project>"}},
		"update send":          {args: []string{"<project>"}},
		"signal add":           {required: map[string]bool{"--title": true}},
		"signal act":           {args: []string{"<id>"}},
		"signal move":          {args: []string{"<id>"}, required: map[string]bool{"--to": true}},
		"signal dismiss":       {args: []string{"<id>"}},
		"artifact import":      {args: []string{"<file>"}},
		"artifact link":        {args: []string{"<https-url>"}},
		"artifact list":        {},
		"artifact publish":     {args: []string{"<id>"}},
		"shot crop":            {args: []string{"<png>"}},
		"shot diff":            {args: []string{"<png-a>", "<png-b>"}},
		"eval list":            {},
		"eval run":             {args: []string{"[task]"}},
		"eval show":            {args: []string{"<run-id>"}},
		"eval export-suite":    {},
		"migrate":              {},
		"job verify":           {args: []string{"<job>"}},
		"job trace":            {args: []string{"<pointer>"}},
		"land":                 {args: []string{"<task>"}},
		"project-md get":       {args: []string{"<project>", "<key>"}},
		"project-md set":       {args: []string{"<project>", "<key>", "<value>"}, output: "cmd-project-md-set.json"},
		"setup":                {args: []string{"[project]"}, output: "cmd-setup.json"},
		"init":                 {output: "cmd-init.json"},
		"workflow show":        {output: "cmd-workflow-show.json"},
		"workflow write":       {required: map[string]bool{"--file": true, "--rev": true}, output: "cmd-workflow-write.json"},
		"usage post":           {required: map[string]bool{"--job": true}},
		"usage show":           {},
	}
}
