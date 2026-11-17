package postfile

import (
	"bytes"
	"encoding/json"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Render is the markdown under a post's header (docs/research/brain-board.md Q8, "Post
// body and links"): a title, a facts list, the type's text, then a "## Links" section.
// Its inputs are the header, the post's own path from the brain root (which gives the
// task folder and the Job node), and names, the file names of other posts by id. It
// reads no file, so the same inputs always give the same bytes. An id with no name in
// names is shown as text, with no link.
func Render(h Header, own string, names map[string]string) []byte {
	task := path.Dir(path.Dir(own))
	r := renderer{h: h, task: task, names: names}
	var body map[string]any
	dec := json.NewDecoder(bytes.NewReader(h.Body))
	dec.UseNumber()
	if dec.Decode(&body) != nil {
		body = nil
	}
	summary, text, named := r.typeText(body)
	subject := h.Job
	if subject == "" {
		subject = "task"
	}
	var b strings.Builder
	b.WriteString("# " + subject + " " + h.Type + ": " + cut(summary) + "\n\n")
	if h.Job != "" {
		line := "- Job: " + r.link(task+"/jobs/"+h.Job, h.Job)
		if h.Attempt != nil {
			line += ", attempt " + itoa(*h.Attempt)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("- From: " + h.Author.Role + " " + h.Author.ID + ", by " + h.By + ", " + utc(h.At) + "\n")
	if h.Re != "" {
		b.WriteString("- About: " + r.ref(h.Re) + "\n")
	}
	if body == nil && len(h.Body) > 0 {
		text = append(text, "- body: "+string(h.Body))
	} else {
		var rest []string
		for k := range body {
			if !named[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		if len(rest) > 0 {
			lines := make([]string, len(rest))
			for i, k := range rest {
				lines[i] = "- " + k + ": " + oneLine(body[k])
			}
			text = append(text, strings.Join(lines, "\n"))
		}
	}
	for _, t := range text {
		if t != "" {
			b.WriteString("\n" + t + "\n")
		}
	}
	b.WriteString("\n## Links\n\n")
	b.WriteString("- " + r.link(task+"/task", path.Base(task)) + " - related: the task this board belongs to\n")
	if h.Job != "" {
		b.WriteString("- " + r.link(task+"/jobs/"+h.Job, h.Job) + " - continues: the Job this " + h.Type + " comes from\n")
	}
	if h.Re != "" {
		rel, why := "continues", "the "+r.refType(h.Re)+" the "+h.Type+" is about"
		if h.Type == "answer" {
			rel, why = "decides", "the question this answer decides"
		}
		b.WriteString("- " + r.refLine(h.Re, "re") + " - " + rel + ": " + why + "\n")
	}
	for _, a := range h.After {
		b.WriteString("- " + r.refLine(a, "after") + " - depends-on: the " + r.refType(a) + " the " + h.Type + " waits for\n")
	}
	return []byte(b.String())
}

type renderer struct {
	h     Header
	task  string
	names map[string]string
}

func (r renderer) link(target, label string) string { return "[[" + target + "|" + label + "]]" }

// ref is a post by id: a link when its file name is known, the id otherwise.
func (r renderer) ref(id string) string {
	name, ok := r.names[id]
	if !ok {
		return id
	}
	return r.link(r.task+"/board/"+strings.TrimSuffix(name, ".md"), label(name))
}

func (r renderer) refLine(id, key string) string {
	if _, ok := r.names[id]; !ok {
		return key + ": " + id
	}
	return r.ref(id)
}

func (r renderer) refType(id string) string {
	if name, ok := r.names[id]; ok {
		if _, typ := splitName(name); typ != "" {
			return typ
		}
	}
	return "post"
}

// label is "<job> <type>" of a post file name.
func label(name string) string {
	job, typ := splitName(name)
	if typ == "" {
		return strings.TrimSuffix(name, ".md")
	}
	return job + " " + typ
}

// splitName splits <seq>-<job>-<type>.md at the first dash after which a registered
// type follows, so a Job id and a type may both hold dashes.
func splitName(name string) (string, string) {
	_, rest, ok := strings.Cut(strings.TrimSuffix(name, ".md"), "-")
	if !ok {
		return "", ""
	}
	for i := 0; i < len(rest); i++ {
		if rest[i] != '-' {
			continue
		}
		if _, known := contract.LookupType(rest[i+1:]); known {
			return rest[:i], rest[i+1:]
		}
	}
	return "", ""
}

// typeText is the title summary, the text blocks and the body keys the type's row in
// Q8 names. Every other key is one "- key: value" line after the text.
func (r renderer) typeText(b map[string]any) (string, []string, map[string]bool) {
	named := map[string]bool{}
	get := func(k string) string {
		v, ok := b[k]
		if !ok {
			return ""
		}
		named[k] = true
		return value(v)
	}
	has := func(k string) bool { _, ok := b[k]; return ok }
	labeled := func(title, k string) string {
		if v := get(k); v != "" {
			return title + ": " + v
		}
		return ""
	}
	list := func(title, k string) string {
		v, ok := b[k]
		if !ok {
			return ""
		}
		named[k] = true
		items, isList := v.([]any)
		if !isList {
			items = []any{v}
		}
		if len(items) == 0 || v == nil {
			return ""
		}
		lines := make([]string, 0, len(items)+1)
		if title != "" {
			lines = append(lines, title+":")
		}
		for _, it := range items {
			lines = append(lines, "- "+oneLine(it))
		}
		return strings.Join(lines, "\n")
	}
	attempt := ""
	if r.h.Attempt != nil {
		attempt = itoa(*r.h.Attempt)
	}
	typ := r.h.Type
	if typ == "control" && value(b["action"]) == "assign" {
		typ = "assign"
	}
	switch typ {
	case "assign":
		named["action"] = true
		if has("attempt") {
			attempt = get("attempt")
		}
		s := "assigned to " + get("to") + ", attempt " + attempt
		return s, []string{labeled("Start", "start"), labeled("Retry of", "retry_of")}, named
	case "started":
		return "started, attempt " + attempt, nil, named
	case "exit":
		return "exited: " + get("reason") + ", code " + get("code"), []string{labeled("Cost", "cost")}, named
	case "result":
		s := get("outcome")
		return s, []string{text(s), list("Changed", "changed"), list("Acceptance", "acceptance"),
			list("Must fix", "must_fix"), list("Decisions needed", "decisions_needed")}, named
	case "checkpoint":
		s := get("step")
		return s, []string{text(s), labeled("Commit", "commit"), list("Acceptance", "acceptance")}, named
	case "question":
		s := get("question")
		blocking := ""
		if has("blocking") {
			blocking = "Blocking: " + get("blocking")
		}
		return s, []string{text(get("why")), list("", "options"), labeled("Who", "who"), blocking}, named
	case "answer":
		s := get("answer")
		grant := ""
		if r.h.For != "" {
			grant = "For: " + r.ref(r.h.For)
		}
		return s, []string{text(get("note")), labeled("Reason", "reason"), grant}, named
	case "waiting":
		return "waiting on " + get("unmet"), nil, named
	case "cancel", "wake":
		return typ, []string{text(get("reason"))}, named
	case "task-move":
		return "moved from " + get("from") + " to " + get("to") + ", gen " + itoa64(r.h.Gen), nil, named
	case "post", "found":
		s := get("text")
		first, _, _ := strings.Cut(s, "\n")
		return first, []string{text(s)}, named
	}
	if has("action") {
		return get("action"), nil, named
	}
	return typ, nil, named
}

// text is a long text field as written, with a "## Links" line demoted so the post
// keeps one Grid Links section.
func text(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == "## Links" {
			lines[i] = "### Links"
		}
	}
	return strings.Join(lines, "\n")
}

func value(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "yes"
		}
		return "no"
	case []any:
		parts := make([]string, len(t))
		for i, e := range t {
			parts[i] = oneLine(e)
		}
		return strings.Join(parts, ", ")
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n")
}

// oneLine is a value on one line: a string with a line break is written quoted.
func oneLine(v any) string {
	s := value(v)
	if strings.ContainsAny(s, "\r\n") {
		q, _ := json.Marshal(s)
		return string(q)
	}
	return s
}

// cut is the summary on one line, at most 80 characters.
func cut(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	s = strings.TrimSpace(s)
	if rs := []rune(s); len(rs) > 80 {
		return string(rs[:80])
	}
	return s
}

func utc(at string) string {
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return at
	}
	return t.UTC().Format("2006-01-02 15:04") + " UTC"
}

func itoa(n int) string     { return strconv.Itoa(n) }
func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
