package hook

import (
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func brainStore(o Options, dir string) brain.Store {
	project := filepath.Base(filepath.Dir(filepath.Dir(dir)))
	if project == where.Personal {
		project = ""
	}
	return brain.Store{GridHome: o.Resolver.GridHome, Project: project}.WithConfig(o.Resolver.Home, o.Getenv)
}
func brainRead(o Options, in hookio.Input) {
	if in.HookEventName != "PostToolUse" || in.SessionID == "" {
		return
	}
	switch in.ToolName {
	case "Read":
	case "Bash":
		brainShow(o, in)
		brainSearch(o, in)
		return
	default:
		return
	}
	if !o.hookEnabled("history-log", in) {
		return
	}
	path := str(in.ToolInput["file_path"])
	if !strings.HasSuffix(path, ".md") {
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
	actual := gridhome.Real(absolute(in.Cwd, path))
	if store.Vault() {
		brainReadVault(o, in, store, dir, path)
		return
	}
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	if !brain.ValidName(name) {
		return
	}
	// Explicit levels avoid a project note with the same name shadowing a home read.
	for _, level := range []string{"project", "home"} {
		if level == "project" && store.Project == "" {
			continue
		}
		e, err := store.Show("brain:" + level + "/" + name)
		if err != nil {
			continue
		}
		if actual != gridhome.Real(filepath.Join(o.Resolver.GridHome, filepath.FromSlash(e.Path))) {
			continue
		}
		o.log("brain read", recordBrainRead(o, in, dir, e.ID))
		return
	}
}

// brainReadVault records a Read of a note in a vault. The id is the path of the
// file below its level root, so Ideas/Café plan.md is brain:home/Ideas/Café plan;
// the note must resolve to the very file that was read.
func brainReadVault(o Options, in hookio.Input, store brain.Store, dir, path string) {
	abs := absolute(in.Cwd, path)
	level, id, ok := store.VaultID(abs)
	if !ok {
		return
	}
	e, err := store.Show("brain:" + level + "/" + id)
	if err != nil {
		return
	}
	if gridhome.Real(abs) != gridhome.Real(filepath.Join(store.Root(), filepath.FromSlash(e.Path))) {
		return
	}
	o.log("brain read", recordBrainRead(o, in, dir, e.ID))
}

// recordBrainRead logs one `brain read` line per note and session. The Read and
// the `grid brain show` paths share the read:<id> key. The via field names the
// Job when the agent runs inside one, otherwise the session.
func recordBrainRead(o Options, in hookio.Input, dir, id string) error {
	return recordBrainReadWhy(o, in, dir, id, "")
}

// recordBrainReadWhy is recordBrainRead with a reason that ends the line, as
// `grid brain search` gives the words it searched for. Inside a Job, the why
// of that Job's link to the note wins over the given reason.
func recordBrainReadWhy(o Options, in hookio.Input, dir, id, why string) error {
	jobID := o.Getenv("GRID_JOB")
	// Decide and mark under the session lock, then append once it is
	// released: .task.lock is never taken while the session lock is held.
	s := session.Store{TaskDir: dir}
	key := "read:" + id
	first := false
	err := s.UpdateActivity(in.SessionID, func(a *session.Activity) error {
		first = !a.Seen[key]
		a.Seen[key] = true
		return nil
	})
	if err == nil && first {
		if w := brainLinkWhy(dir, jobID, id); w != "" {
			why = w
		}
		err = task.AppendHistoryWhy(dir, "brain read", id, writer(in), brainReadVia(jobID, in.SessionID), why)
	}
	return err
}

// brainShow records a note read through `grid brain show <id>` in a Bash
// command. It only records: an unparseable command, a proposal, an unknown
// note or an unresolved task is ignored without output.
func brainShow(o Options, in hookio.Input) {
	id, ok := parseBrainShow(str(in.ToolInput["command"]))
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
	e, err := brainStore(o, dir).Show(id)
	if err != nil {
		return
	}
	o.log("brain read", recordBrainRead(o, in, dir, e.ID))
}

// shellWords splits the first simple command of a shell line into words. It
// stops at an unquoted && || ; | & or newline. Quotes group words; nothing is
// expanded or run.
func shellWords(s string) []string {
	var words []string
	var cur strings.Builder
	has := false
	var quote rune
	flush := func() {
		if has {
			words = append(words, cur.String())
		}
		cur.Reset()
		has = false
	}
	runes := []rune(s)
	for i, r := range runes {
		// A redirection such as 2>&1 or &>file is part of the command, not a
		// separator.
		redirect := r == '&' && ((i > 0 && (runes[i-1] == '>' || runes[i-1] == '<')) || (i+1 < len(runes) && runes[i+1] == '>'))
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, has = r, true
		case redirect:
			cur.WriteRune(r)
			has = true
		case r == '&' || r == '|' || r == ';' || r == '\n' || r == '\r':
			flush()
			return words
		case r == ' ' || r == '\t':
			flush()
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	flush()
	return words
}

// parseBrainShow returns the note id of a `grid brain show` command, with the
// --level flag folded in as the CLI does. A proposal read is not a brain read.
func parseBrainShow(command string) (string, bool) {
	w := shellWords(command)
	if len(w) < 4 || w[1] != "brain" || w[2] != "show" {
		return "", false
	}
	if base := strings.ToLower(filepath.Base(strings.ReplaceAll(w[0], `\`, "/"))); base != "grid" && base != "grid.exe" {
		return "", false
	}
	id, level := "", ""
	args := w[3:]
	for i := 0; i < len(args); i++ {
		a := args[i]
		if r := strings.TrimLeft(a, "0123456789"); strings.HasPrefix(r, ">") || strings.HasPrefix(r, "<") || strings.HasPrefix(r, "&>") {
			// A redirection: skip it, and its target when it is a separate word.
			if strings.Trim(r, "<>&") == "" && !strings.Contains(r, "&") && i+1 < len(args) {
				i++
			}
			continue
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			if id != "" {
				return "", false
			}
			id = a
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(a, "-"), "=")
		switch name {
		case "proposal":
			return "", false
		case "json":
		case "task", "project", "level":
			if !hasValue {
				if i+1 >= len(args) {
					return "", false
				}
				i++
				value = args[i]
			}
			if name == "level" {
				level = value
			}
		default:
			return "", false
		}
	}
	if id == "" {
		return "", false
	}
	if level != "" && !strings.HasPrefix(id, "brain:") {
		id = "brain:" + level + "/" + id
	}
	return id, true
}
