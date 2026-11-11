package hook

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// hdSearch is a parsed `grid brain search` command: the words the CLI joins into
// the query, and the flags that change what it searches.
type hdSearch struct {
	words, topic, level, project, task string
	limit                              int
}

// parseBrainSearch reads the first simple command of a Bash line as the CLI reads
// `grid brain search`: the same flags, the same limit range and level rules. A
// command the CLI would refuse, or one that is not a search, is not ok.
func parseBrainSearch(command string) (hdSearch, bool) {
	w := shellWords(command)
	if len(w) < 4 || w[1] != "brain" || w[2] != "search" {
		return hdSearch{}, false
	}
	if base := strings.ToLower(filepath.Base(strings.ReplaceAll(w[0], `\`, "/"))); base != "grid" && base != "grid.exe" {
		return hdSearch{}, false
	}
	s := hdSearch{limit: 5}
	var positional []string
	args := hdSStripRedirects(w[3:])
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
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		if name == "json" {
			if _, err := strconv.ParseBool(value); hasValue && err != nil {
				return hdSearch{}, false
			}
			continue
		}
		switch name {
		case "project", "level", "task", "topic", "limit":
		default:
			return hdSearch{}, false
		}
		if !hasValue {
			if i+1 >= len(args) {
				return hdSearch{}, false
			}
			i++
			value = args[i]
		}
		switch name {
		case "project":
			s.project = value
		case "level":
			s.level = value
		case "task":
			s.task = value
		case "topic":
			s.topic = value
		case "limit":
			n, err := strconv.ParseInt(value, 0, strconv.IntSize)
			if err != nil {
				return hdSearch{}, false
			}
			s.limit = int(n)
		}
	}
	s.words = strings.Join(positional, " ")
	if strings.TrimSpace(s.words) == "" || s.limit < 1 || s.limit > 100 {
		return hdSearch{}, false
	}
	if s.level != "" && s.level != "home" && s.level != "project" {
		return hdSearch{}, false
	}
	if s.level == "home" && (s.task != "" || s.project != "") {
		return hdSearch{}, false
	}
	return s, true
}

// hdSStripRedirects drops redirections (`> file`, `2>&1`, `&>file`) from the
// words after the verb, as parseBrainShow skips them.
func hdSStripRedirects(args []string) []string {
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if r := strings.TrimLeft(a, "0123456789"); strings.HasPrefix(r, ">") || strings.HasPrefix(r, "<") || strings.HasPrefix(r, "&>") {
			if strings.Trim(r, "<>&") == "" && !strings.Contains(r, "&") && i+1 < len(args) {
				i++
			}
			continue
		}
		out = append(out, a)
	}
	return out
}

// hdSSameProject reports whether an explicit --project or --task names the
// project the hook's task belongs to, so the search read the notes the hook reads.
func hdSSameProject(o Options, in hookio.Input, project string, q where.Query) bool {
	if q.Project == "" && q.Task == "" {
		return true
	}
	if q.Project != "" && q.Project == project {
		return true
	}
	if q.Cwd == "" {
		q.Cwd = in.Cwd
	}
	res, err := o.Resolver.Resolve(q)
	if err != nil {
		return false
	}
	if res.Project == where.Personal {
		return project == ""
	}
	return res.Project == project
}

// brainSearch records the notes a `grid brain search` in a Bash command showed
// the agent: one `brain read` line per hit, with the words as the why. It runs
// the search again on the hook's own store and never reads the tool's output.
// Like brainShow it only records: a command the CLI would refuse, a search of
// another project, a miss or an unresolved task is ignored without output.
func brainSearch(o Options, in hookio.Input) {
	s, ok := parseBrainSearch(str(in.ToolInput["command"]))
	if !ok || !o.hookEnabled("history-log", in) {
		return
	}
	dir, err := taskDir(o, in)
	if err != nil {
		o.log("brain read", err)
		return
	}
	if dir == "" {
		return
	}
	store := brainStore(o, dir)
	if !hdSSameProject(o, in, store.Project, where.Query{Project: s.project}) || !hdSSameProject(o, in, store.Project, where.Query{Task: s.task}) {
		return
	}
	store.ReadLevel = s.level
	hits, err := store.Search(s.words, s.topic, s.limit)
	if err != nil {
		return
	}
	for _, h := range hits {
		o.log("brain read", recordBrainReadWhy(o, in, dir, h.ID, "search: "+s.words))
	}
}
