// Package updates reads and writes a project's update files,
// projects/<project>/updates/<day>-<handle>.md in the Grid home, in the grammar
// the app's Updates tab parses (gridconsole-app internal/slices/projectupdates,
// updates.go: namePattern :38-42, parseUpdate :84-168, lineOf :169-184). Send
// only marks a file sent; nothing here mails anything.
package updates

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// Now is the clock every act reads; tests set it.
var Now = time.Now

// MinuteLayout is the app's minute value, YYYY-MM-DDTHH:MM.
const MinuteLayout = "2006-01-02T15:04"

// MaxLines is the most body lines the app parses in one update (updates.go:158).
const MaxLines = 100

// NothingNew is the draft's one line when no task closed, as the app's fake.
var NothingNew = Line{Task: "T-00", Text: "Nothing new since the last update."}

var (
	namePattern   = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})-([a-z0-9][a-z0-9-]{0,59})\.md$`)
	minutePattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}$`)
	keyPattern    = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_-]*):(.*)$`)
	taskPattern   = regexp.MustCompile(`^(?:T-[0-9]+|[0-7][0-9A-HJKMNP-TV-Z]{25})$`)
)

// Line is one body line, "- <task> · <text>"; keys as the app's updateLine.
type Line struct {
	Task string `json:"task"`
	Text string `json:"text"`
}

// Update is one update file. K, Day, At, Title, Read, Bounced and Lines carry
// the keys of the app's sentUpdate (read.go:24-32); Status, To and Drafted are
// the file's own keys, since a draft or a skipped update has no sent minute.
type Update struct {
	K       string   `json:"k"`
	Day     string   `json:"day"`
	Status  string   `json:"status"`
	Title   string   `json:"title"`
	To      []string `json:"to"`
	Drafted string   `json:"drafted,omitempty"`
	At      string   `json:"at,omitempty"`
	Read    bool     `json:"read"`
	Bounced bool     `json:"bounced,omitempty"`
	Lines   []Line   `json:"lines"`

	name, handle string
}

// Result is the answer of every act: the project's updates, newest first.
type Result struct {
	State   string   `json:"state"`
	Name    string   `json:"-"`
	Updates []Update `json:"updates"`
}

// Act names the project and the act; Title is for draft, Lines for edit.
type Act struct {
	Act, Project, Cwd, Title string
	Lines                    []Line
}

func changed(format string, a ...any) error {
	return &settings.CodedError{Code: "changed", Msg: fmt.Sprintf(format, a...)}
}

// Do performs one act under the project's updates lock. A draft when one
// exists, or any other act when none exists, fails with code changed.
func Do(r where.Resolver, a Act) (Result, error) {
	out := Result{State: "applied"}
	if r.GridHome == "" {
		return out, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	res, err := r.Resolve(where.Query{Cwd: a.Cwd, Project: a.Project})
	if err != nil {
		return out, err
	}
	dir := filepath.Join(r.GridHome, "projects", res.Project, "updates")
	unlock, err := localdata.Lock(r.GridHome, ".updates-"+res.Project+".lock")
	if err != nil {
		return out, err
	}
	defer unlock()
	list, err := Read(dir)
	if err != nil {
		return out, err
	}
	draft := -1
	for i, u := range list {
		if u.Status == "draft" {
			draft = i
			break
		}
	}
	now := Now()
	minute := now.Format(MinuteLayout)
	if a.Act == "draft" {
		if draft >= 0 {
			return out, changed("an update is already drafted: %s", list[draft].name)
		}
		title := a.Title
		if title == "" {
			title = "Update " + now.Format("2006-01-02")
		}
		if err := CheckTitle(title); err != nil {
			return out, err
		}
		lines, err := closedLines(r, a.Cwd, res.Project, now, list)
		if err != nil {
			return out, err
		}
		day := now.Format("2006-01-02")
		name := day + "-update.md"
		for n := 2; exists(filepath.Join(dir, name)); n++ {
			name = fmt.Sprintf("%s-update-%d.md", day, n)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return out, err
		}
		u := Update{Status: "draft", Title: title, Drafted: minute, Lines: lines, name: name}
		out.Name = name
		return finish(dir, out, atomicfile.Write(filepath.Join(dir, name), u.Bytes(), 0o644))
	}
	if draft < 0 {
		return out, changed("no update is drafted for project %s", res.Project)
	}
	u := list[draft]
	out.Name = u.name
	path := filepath.Join(dir, u.name)
	switch a.Act {
	case "discard":
		return finish(dir, out, os.Remove(path))
	case "edit":
		if len(a.Lines) == 0 {
			return out, errors.New("edit needs the lines")
		}
		u.Lines = a.Lines
	case "skip":
		u.Status = "skipped"
	case "send":
		u.Status, u.At = "sent", minute
	default:
		return out, fmt.Errorf("unknown act %q", a.Act)
	}
	return finish(dir, out, atomicfile.Write(path, u.Bytes(), 0o644))
}

func finish(dir string, out Result, err error) (Result, error) {
	if err != nil {
		return out, err
	}
	out.Updates, err = Read(dir)
	return out, err
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

// Bytes is the file text: front matter with the keys the app reads, then one
// "- <task> · <text>" line each. The title is quoted; the app's dialect strips
// the quotes without unescaping, so CheckTitle refuses what cannot round-trip.
func (u Update) Bytes() []byte {
	var b strings.Builder
	b.WriteString("---\nstatus: " + u.Status + "\n")
	b.WriteString(`title: "` + u.Title + "\"\n")
	if len(u.To) > 0 {
		b.WriteString("to:\n")
		for _, to := range u.To {
			b.WriteString("  - " + to + "\n")
		}
	}
	if u.Drafted != "" {
		b.WriteString("drafted: " + u.Drafted + "\n")
	}
	if u.At != "" {
		b.WriteString("sent: " + u.At + "\n")
	}
	b.WriteString("---\n")
	for _, l := range u.Lines {
		b.WriteString("- " + l.Task + " · " + l.Text + "\n")
	}
	return []byte(b.String())
}

// CheckTitle refuses a title the app would read differently or not at all:
// empty or a placeholder, over 300 characters, a control character, a " # "
// comment, or a [list] or {object} shape.
func CheckTitle(title string) error {
	t := strings.TrimSpace(title)
	bad := t == "" || t == "-" || t == "–" || t == "—" || !textFits(title, 300) || comment(title) ||
		(strings.HasPrefix(title, "[") && strings.HasSuffix(title, "]")) || (strings.HasPrefix(title, "{") && strings.HasSuffix(title, "}"))
	if bad {
		return fmt.Errorf("title %q cannot be written as an update title", title)
	}
	return nil
}

func comment(s string) bool {
	runes := []rune(s)
	for i := 1; i+1 < len(runes); i++ {
		if runes[i] == '#' && unicode.IsSpace(runes[i-1]) && unicode.IsSpace(runes[i+1]) {
			return true
		}
	}
	return false
}

func textFits(s string, limit int) bool {
	return utf8.RuneCountInString(s) <= limit && strings.IndexFunc(s, unicode.IsControl) < 0
}

// ParseLine reads "<task> · <text>" as the app's lineOf reads a body line
// after its "- ": a display id or ULID, " ·", then nonempty text.
func ParseLine(s string) (Line, bool) {
	id, rest, ok := strings.Cut(s, " ·")
	if !ok || !taskPattern.MatchString(id) {
		return Line{}, false
	}
	if first, _ := utf8.DecodeRuneInString(rest); rest != "" && !unicode.IsSpace(first) {
		return Line{}, false
	}
	text := strings.TrimSpace(rest)
	return Line{Task: id, Text: text}, text != "" && textFits(text, 2000)
}

// Read lists the folder's update files the app would show, newest first: day
// descending, then handle ascending (read.go staffUpdates). A file the app
// would refuse is left out, as the app leaves it out.
func Read(dir string) ([]Update, error) {
	out := []Update{}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") || !e.Type().IsRegular() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if u, ok := Parse(e.Name(), data); ok {
			out = append(out, u)
		}
	}
	slices.SortFunc(out, func(a, b Update) int {
		if order := strings.Compare(b.Day, a.Day); order != 0 {
			return order
		}
		return strings.Compare(a.handle, b.handle)
	})
	return out, nil
}

// Parse applies the app's parseUpdate rules to one file.
func Parse(name string, data []byte) (Update, bool) {
	u := Update{To: []string{}, Lines: []Line{}, name: name}
	stamp := namePattern.FindStringSubmatch(name)
	if stamp == nil || !isMinute(stamp[1]+"T00:00") || len(data) > 64<<10 || !utf8.Valid(data) {
		return u, false
	}
	u.Day, u.handle, u.K = stamp[1], stamp[2], stamp[1]+"-"+stamp[2]
	text := strings.ReplaceAll(strings.TrimPrefix(string(data), string(rune(0xFEFF))), "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	end := strings.Index(rest, "\n---")
	if !ok || end < 0 {
		return u, false
	}
	body := strings.TrimPrefix(rest[end+4:], "\n")
	fields, lists := map[string]string{}, map[string][]string{}
	open := ""
	for _, line := range strings.Split(rest[:end], "\n") {
		if item, ok := strings.CutPrefix(strings.TrimLeft(line, " \t"), "- "); open != "" && ok && line != strings.TrimLeft(line, " \t") {
			lists[open] = append(lists[open], scalar(item))
			continue
		}
		open = ""
		kv := keyPattern.FindStringSubmatch(line)
		if kv == nil {
			continue
		}
		v := scalar(kv[2])
		fields[kv[1]], lists[kv[1]] = v, nil
		if v == "" {
			open = kv[1]
		} else if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
			for _, part := range strings.Split(v[1:len(v)-1], ",") {
				if part = scalar(part); part != "" {
					lists[kv[1]] = append(lists[kv[1]], part)
				}
			}
		}
	}
	for key, v := range fields {
		switch key {
		case "status", "title", "to", "drafted", "sent", "read", "bounced":
		default:
			return u, false
		}
		if v == "" || v == "-" || v == "–" || v == "—" {
			if lists[key] == nil {
				delete(fields, key)
			}
		} else if key != "to" && (lists[key] != nil || strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}")) {
			return u, false
		}
	}
	u.Status, u.Title = fields["status"], fields["title"]
	if u.Status != "draft" && u.Status != "sent" && u.Status != "skipped" || u.Title == "" || !textFits(u.Title, 300) {
		return u, false
	}
	if l := lists["to"]; l != nil {
		u.To = l
	} else if v := fields["to"]; v != "" {
		u.To = []string{v}
	}
	minutes := map[string]string{}
	for _, key := range []string{"drafted", "sent", "read", "bounced"} {
		if v, ok := fields[key]; ok {
			if !isMinute(v) {
				return u, false
			}
			minutes[key] = v
		}
	}
	u.Drafted, u.At, u.Read, u.Bounced = minutes["drafted"], minutes["sent"], minutes["read"] != "", minutes["bounced"] != ""
	if u.Status == "draft" && u.Drafted == "" || u.Status == "sent" && u.At == "" {
		return u, false
	}
	if u.Status != "sent" && (u.At != "" || u.Read || u.Bounced) {
		return u, false
	}
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		item, ok := strings.CutPrefix(line, "- ")
		l, ok2 := ParseLine(item)
		if !ok || !ok2 || len(u.Lines) == MaxLines {
			return u, false
		}
		u.Lines = append(u.Lines, l)
	}
	return u, true
}

// scalar trims a value, drops a " # " comment and one pair of matching quotes.
func scalar(raw string) string {
	runes := []rune(raw)
	for i := 1; i+1 < len(runes); i++ {
		if runes[i] == '#' && unicode.IsSpace(runes[i-1]) && unicode.IsSpace(runes[i+1]) {
			raw = string(runes[:i-1])
			break
		}
	}
	v := strings.TrimSpace(raw)
	if len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[0] == v[len(v)-1] {
		return v[1 : len(v)-1]
	}
	return v
}

func isMinute(s string) bool {
	if !minutePattern.MatchString(s) {
		return false
	}
	_, err := time.Parse(MinuteLayout, s)
	return err == nil
}

// closedLines is one line per task of the project closed at or after the
// newest sent minute (every closed task when none was sent). A closed task's
// moment is the when of its last history receipt to state closed; a closed
// task with no such receipt counts only when no update was sent. With no task
// the draft holds NothingNew. Past MaxLines the last line counts the rest.
func closedLines(r where.Resolver, cwd, project string, now time.Time, list []Update) ([]Line, error) {
	tasks, err := task.Store{Resolver: r}.List(cwd, project, "closed")
	if err != nil {
		return nil, err
	}
	since := ""
	for _, u := range list {
		if u.Status == "sent" && u.At > since {
			since = u.At
		}
	}
	var from time.Time
	if since != "" {
		if from, err = time.ParseInLocation(MinuteLayout, since, now.Location()); err != nil {
			return nil, err
		}
	}
	lines := []Line{}
	for _, t := range tasks {
		if t.Unreadable || t.State != "closed" {
			continue
		}
		if since != "" {
			at, ok := closedAt(t.Path)
			if !ok || at.Before(from) {
				continue
			}
		}
		lines = append(lines, Line{Task: t.ID, Text: lineText(t.Title, t.ID)})
	}
	if len(lines) == 0 {
		return []Line{NothingNew}, nil
	}
	if len(lines) > MaxLines {
		rest := len(lines) - (MaxLines - 1)
		lines = append(lines[:MaxLines-1], Line{Task: lines[MaxLines-1].Task, Text: fmt.Sprintf("and %d more closed tasks", rest)})
	}
	return lines, nil
}

func lineText(title, id string) string {
	text := strings.Join(strings.Fields(strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return ' '
		}
		return c
	}, title)), " ")
	if runes := []rune(text); len(runes) > 2000 {
		text = string(runes[:2000])
	}
	if text == "" {
		text = "Closed " + id
	}
	return text
}

// closedAt reads the task's history.md receipts, "- {json}" lines.
func closedAt(dir string) (time.Time, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		return time.Time{}, false
	}
	var at time.Time
	found := false
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 0, 64<<10), 4<<20)
	for scan.Scan() {
		raw, ok := strings.CutPrefix(scan.Text(), "- {")
		if !ok {
			continue
		}
		var e task.Event
		if json.Unmarshal([]byte("{"+raw), &e) != nil || e.ToState != "closed" {
			continue
		}
		if when, err := time.Parse(contract.TimeLayout, e.When); err == nil {
			at, found = when, true
		}
	}
	return at, found
}
