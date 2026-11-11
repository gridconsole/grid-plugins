package hook

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/guard"
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
	return brain.Store{GridHome: o.Resolver.GridHome, Project: project}
}
func brainRead(o Options, in hookio.Input) {
	if in.HookEventName != "PostToolUse" || in.SessionID == "" {
		return
	}
	switch in.ToolName {
	case "Read":
	case "Bash":
		brainShow(o, in)
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
	name := strings.TrimSuffix(filepath.Base(path), ".md")
	if !brain.ValidName(name) {
		return
	}
	actual := gridhome.Real(absolute(in.Cwd, path))
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

// recordBrainRead logs one `brain read` line per note and session. The Read and
// the `grid brain show` paths share the read:<id> key. The Job id, when the
// agent runs inside one, goes in the via field.
func recordBrainRead(o Options, in hookio.Input, dir, id string) error {
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
		err = task.AppendHistory(dir, "brain read", id, writer(in), o.Getenv("GRID_JOB"))
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

var namedTask = regexp.MustCompile(`\bT-[0-9]+(?:-[A-Za-z0-9_.-]+)?\b`)

func namedTaskDir(o Options, in hookio.Input, text string) (string, error) {
	name := namedTask.FindString(text)
	if name == "" {
		return taskDir(o, in)
	}
	roots, err := o.Resolver.TasksDirs()
	if err != nil {
		return "", err
	}
	found := ""
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			if !e.IsDir() || !(e.Name() == name || strings.HasPrefix(e.Name(), name+"-")) {
				continue
			}
			dir, err := task.FolderAt(o.Resolver, filepath.Join(root, e.Name()))
			if err != nil {
				return "", err
			}
			if found != "" && dir != "" {
				return "", fmt.Errorf("ambiguous task %s", name)
			}
			found = dir
		}
	}
	return found, nil
}

func recall(o Options, in hookio.Input) {
	if o.Resolver.GridHome == "" {
		return
	}
	text := ""
	switch in.HookEventName {
	case "PreToolUse":
		if (in.ToolName != "Agent" && in.ToolName != "Task") || contract.NormalizeRole(guard.AgentName(str(in.ToolInput["subagent_type"]))) != "planner" || in.SessionID == "" {
			return
		}
		dir, err := namedTaskDir(o, in, str(in.ToolInput["prompt"])+" "+str(in.ToolInput["description"]))
		if err != nil {
			o.log("planner task", err)
			return
		}
		if dir == "" {
			return
		}
		o.log("planner task", (session.Store{TaskDir: o.Resolver.GridHome}).UpdateActivity(in.SessionID, func(a *session.Activity) error { a.PlannerTask = dir; return nil }))
		return
	case "UserPromptSubmit":
		if !o.hookEnabled("user-prompt", in) {
			return
		}
		trimmed := strings.TrimSpace(in.Prompt)
		command, _, _ := strings.Cut(trimmed, " ")
		if command != "/grid:plan" && command != "/grid:execute" && command != "/grid:resume" {
			return
		}
		text = in.Prompt
	case "PostToolUse":
		if in.ToolName != "Skill" {
			return
		}
		skill := strings.TrimPrefix(str(in.ToolInput["skill"]), "grid:")
		if skill != "plan" && skill != "execute" && skill != "resume" {
			return
		}
		text = str(in.ToolInput["args"])
	case "SubagentStart":
		if !o.hookEnabled("subagent-start", in) {
			return
		}
		if name := contract.NormalizeRole(guard.AgentName(in.AgentType)); name != "planner" && name != "builder" {
			return
		}
	default:
		return
	}
	dir, err := namedTaskDir(o, in, text)
	if err != nil {
		o.log("recall", err)
		return
	}
	if in.HookEventName == "SubagentStart" && contract.NormalizeRole(guard.AgentName(in.AgentType)) == "planner" && in.SessionID != "" {
		a, err := (session.Store{TaskDir: o.Resolver.GridHome}).Activity(in.SessionID)
		if err != nil {
			o.log("planner task", err)
			return
		}
		if a.PlannerTask != "" {
			dir, err = task.FolderAt(o.Resolver, a.PlannerTask)
			if err != nil {
				o.log("planner task", err)
				return
			}
		}
	}
	if dir == "" {
		return
	}
	doc, err := task.Document(dir)
	if err != nil {
		o.log("recall", err)
		return
	}
	history, _ := readBounded(filepath.Join(dir, "history.md"))
	plan, _ := readBounded(filepath.Join(dir, "plan.md"))
	if doc.Data["status"] != nil {
		original, _ := readBounded(filepath.Join(dir, "task.md"))
		stage := guard.PlanStage(original, history)
		if stage == "" || stage == "past" {
			return
		}
	}
	query := str(doc.Data["title"]) + " " + str(doc.Data["summary"])
	if intent, err := readBounded(filepath.Join(dir, "intent.md")); err == nil {
		query += " " + intent
	}
	hits, err := brainStore(o, dir).Search(query, "", 5)
	if err != nil {
		o.log("recall", err)
		return
	}
	context := "Brain recall: nothing found. Record Recall: nothing found in the plan when no note applies."
	if len(hits) > 0 {
		var lines []string
		lines = append(lines, "Brain pointers for this task; open only notes that apply:")
		used := len(lines[0]) + 200
		for i, h := range hits {
			line := fmt.Sprintf("- %s (%s): %s Why: %s", h.ID, h.Path, h.Summary, h.Description)
			if used+len(line) > 2400 {
				lines = append(lines, fmt.Sprintf("%d pointers cut; grid brain search lists more.", len(hits)-i))
				break
			}
			lines = append(lines, line)
			used += len(line) + 1
		}
		lines = append(lines, "Record Used: <pointer> in the plan for each note used.")
		context = strings.Join(lines, "\n")
	}
	logHistory := o.hookEnabled("history-log", in)
	emit := true
	if in.SessionID != "" {
		// Content identity also recalls after a plan edit without a numeric revision.
		key := fmt.Sprintf("recall:%s:%x", in.AgentID, sha256.Sum256([]byte(plan)))
		// The history line is appended after the session lock is released, so
		// .task.lock is never taken inside it (attach takes them the other way).
		appendLine := false
		err = (session.Store{TaskDir: dir}).UpdateActivity(in.SessionID, func(a *session.Activity) error {
			emit = !a.Seen[key]
			appendLine = emit && logHistory
			a.Seen[key] = true
			return nil
		})
		if err != nil {
			o.log("recall", err)
			return
		}
		if appendLine {
			o.log("recall", task.AppendHistory(dir, "brain recall", fmt.Sprintf("%d pointers", len(hits)), writer(in), ""))
		}
	}
	if emit && o.Stdout != nil {
		o.log("recall output", json.NewEncoder(o.Stdout).Encode(map[string]any{"hookSpecificOutput": map[string]string{"hookEventName": in.HookEventName, "additionalContext": context}}))
	}
}
