package brain

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"
)

// A note a person also edits in Obsidian is rewritten line by line, not dumped
// again (docs/research/brain-obsidian.md 4.4): the front matter keys Grid did not
// change, their order, comments, blank lines and the line endings stay as the
// person left them, and so does the body unless the change is in it.

// rawFile is a note file cut at its front matter without changing a byte.
type rawFile struct {
	bom   string   // a leading byte order mark
	open  string   // the opening --- line, with its line ending
	front []string // the front matter lines, each with its line ending
	close string   // the closing --- line
	body  string   // everything after the closing line
	eol   string   // the line ending the file uses: "\n" or "\r\n"
}

// splitRaw cuts data at its front matter. ok is false for a file with none, or
// whose --- block never closes (a Markdown rule, not front matter).
func splitRaw(data []byte) (rawFile, bool) {
	s := string(data)
	var f rawFile
	if strings.HasPrefix(s, "\xef\xbb\xbf") {
		f.bom, s = s[:3], s[3:]
	}
	lines := strings.SplitAfter(s, "\n")
	text := func(l string) string { return strings.TrimSuffix(strings.TrimSuffix(l, "\n"), "\r") }
	if text(lines[0]) != "---" {
		return rawFile{}, false
	}
	for i := 1; i < len(lines); i++ {
		if text(lines[i]) == "---" {
			f.open, f.front, f.close, f.body = lines[0], lines[1:i], lines[i], strings.Join(lines[i+1:], "")
			f.eol = "\n"
			if strings.HasSuffix(f.open, "\r\n") {
				f.eol = "\r\n"
			}
			return f, true
		}
	}
	return rawFile{}, false
}

func (f rawFile) bytes() []byte {
	return []byte(f.bom + f.open + strings.Join(f.front, "") + f.close + f.body)
}

// frontSpan is one top-level key and the lines that belong to it.
type frontSpan struct {
	key        string
	start, end int // lines [start, end)
}

var keyLine = regexp.MustCompile(`^(?:"([^"\n]*)"|'([^'\n]*)'|([^\s#'"\-\[\]{}&*!|>%@` + "`" + `][^:#\n]*?))[ \t]*:(?:[ \t]|\r?\n|$)`)

func gapLine(l string) bool {
	t := strings.TrimRight(l, "\r\n")
	return strings.TrimSpace(t) == "" || strings.HasPrefix(t, "#")
}

// frontSpans finds the top-level keys of front matter lines. A block runs to the
// next key; blank and column-0 comment lines after its last line belong to no key,
// so replacing or dropping a key leaves them where they are.
func frontSpans(front []string) []frontSpan {
	var spans []frontSpan
	for i, l := range front {
		if m := keyLine.FindStringSubmatch(l); m != nil {
			spans = append(spans, frontSpan{key: strings.TrimSpace(m[1] + m[2] + m[3]), start: i})
		}
	}
	for k := range spans {
		end := len(front)
		if k+1 < len(spans) {
			end = spans[k+1].start
		}
		for end > spans[k].start+1 && gapLine(front[end-1]) {
			end--
		}
		spans[k].end = end
	}
	return spans
}

// spanValue loads the value of the one key a span holds.
func spanValue(lines []string) (*yaml.Node, error) {
	text := strings.ReplaceAll(strings.Join(lines, ""), "\r\n", "\n")
	var doc yaml.Node
	if err := yaml.Load([]byte(text), &doc); err != nil {
		return nil, err
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode || len(doc.Content[0].Content) != 2 {
		return nil, fmt.Errorf("not one key")
	}
	return doc.Content[0].Content[1], nil
}

// sameValue compares what two values mean, not how they are written: quotes,
// block or flow style and comments do not count.
func sameValue(a, b *yaml.Node) bool {
	if a.Kind != b.Kind || len(a.Content) != len(b.Content) {
		return false
	}
	if a.Kind == yaml.ScalarNode {
		return a.Value == b.Value && a.Tag == b.Tag
	}
	for i := range a.Content {
		if !sameValue(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}

func withEOL(lines []string, eol string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = strings.ReplaceAll(l, "\n", eol)
	}
	return out
}

// patchFront returns f's front matter lines with the keys of want put in. A key
// whose value means the same is not touched, a changed key is replaced in its
// place, a new key goes last. A key f has and want lacks is removed when drop says
// Grid owns it, and kept otherwise: it is the person's.
func patchFront(f rawFile, want string, drop func(key string) bool) []string {
	wl := strings.SplitAfter(want, "\n")
	if wl[len(wl)-1] == "" {
		wl = wl[:len(wl)-1]
	}
	newSpans, oldSpans := frontSpans(wl), frontSpans(f.front)
	newBy := map[string]frontSpan{}
	for _, sp := range newSpans {
		newBy[sp.key] = sp
	}
	oldAt := map[string]int{}
	for j, sp := range oldSpans {
		oldAt[sp.key] = j
	}
	// A key the file lacks goes where the new bytes put it: right after the key
	// that comes before it there (at the top when it comes first).
	inserts := map[int][]string{}
	anchor := -1
	for _, sp := range newSpans {
		if j, ok := oldAt[sp.key]; ok {
			anchor = j
			continue
		}
		inserts[anchor] = append(inserts[anchor], withEOL(wl[sp.start:sp.end], f.eol)...)
	}
	var out []string
	i := 0
	for j, sp := range oldSpans {
		out = append(out, f.front[i:sp.start]...)
		i = sp.end
		if j == 0 {
			out = append(out, inserts[-1]...)
		}
		keep := f.front[sp.start:sp.end]
		nsp, inNew := newBy[sp.key]
		switch {
		case inNew:
			a, errA := spanValue(keep)
			b, errB := spanValue(wl[nsp.start:nsp.end])
			if errA != nil || errB != nil || !sameValue(a, b) {
				keep = withEOL(wl[nsp.start:nsp.end], f.eol)
			}
		case drop(sp.key):
			keep = nil
		}
		out = append(out, keep...)
		out = append(out, inserts[j]...)
	}
	out = append(out, f.front[i:]...)
	if len(oldSpans) == 0 {
		out = append(out, inserts[-1]...)
	}
	return out
}

func lfText(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

// patchConfig is the patch for a config entry: want is the whole new entry as the
// editor encoded it and is the truth for which keys exist, so a key it lacks is
// removed. What a person kept in the file that the editor did not change stays:
// comments, blank lines, key order, spelling and line endings.
func patchConfig(old, want []byte) []byte {
	of, ok := splitRaw(old)
	wf, ok2 := splitRaw(want)
	if !ok || !ok2 {
		return want
	}
	of.front = patchFront(of, strings.Join(wf.front, ""), func(string) bool { return true })
	if strings.TrimRight(lfText(of.body), "\n") != strings.TrimRight(lfText(wf.body), "\n") {
		of.body = strings.ReplaceAll(lfText(wf.body), "\n", of.eol)
	}
	return of.bytes()
}

// filedFront is the front matter of a note that has been accepted: flat scalars
// only, so Obsidian's Properties editor shows every key and never a nested map.
// The author is the flat source; the links live in the body (## Links).
type filedFront struct {
	Name         string `yaml:"name"`
	Topic        string `yaml:"topic"`
	Type         string `yaml:"type"`
	Description  string `yaml:"description"`
	Summary      string `yaml:"summary"`
	Updated      string `yaml:"updated"`
	Source       string `yaml:"source"`
	SupersededBy string `yaml:"superseded_by"`
	Level        string `yaml:"level"`
	Status       string `yaml:"status"`
	Reason       string `yaml:"reason"`
}

// filedKeys are the keys Grid owns in a filed note. A key outside the list, and
// the retired origin key, belong to the person or to the history and stay.
var filedKeys = strings.Fields("name topic type description summary updated source superseded_by links level status reason")

func filedKey(k string) bool { return slices.Contains(filedKeys, k) }

func dumpFiled(n Note) (string, error) {
	f := filedFront{n.Name, n.Topic, n.Type, n.Description, n.Summary, n.Updated, n.Source, n.SupersededBy, n.Level, n.Status, n.Reason}
	b, err := yaml.Dump(f, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	return string(b), err
}

// filedOf is the note n becomes when it is filed: the same note without the origin
// of its author, who is named by the flat source. A Job's source names the task
// and the Job, completed from the origin when the proposal's source does not.
func filedOf(n Note) Note {
	if n.Origin.Kind == "job" && n.Origin.ID != "" && !strings.Contains(n.Source, n.Origin.ID) {
		n.Source += " (job " + n.Origin.ID + ")"
	}
	n.Origin = Origin{}
	return n
}

// encodeFiled writes an accepted note: flat front matter, the body, then the
// links as a ## Links section a person can read and edit.
func encodeFiled(n Note) ([]byte, error) {
	front, err := dumpFiled(n)
	if err != nil {
		return nil, err
	}
	return []byte("---\n" + front + "---\n" + bodyWithLinks(n.Body, n.Links, n.Level)), nil
}

// EncodeOver is Encode for a note that is already on disk as old: every key and
// line a person added is kept, and only what n changed is written. A note without
// front matter is a person's own and comes back as it was. Proposals are Grid's
// own files and are written whole.
func EncodeOver(old []byte, n Note) ([]byte, error) {
	if old == nil || n.Status != "accepted" {
		return Encode(n)
	}
	f, ok := splitRaw(old)
	if !ok {
		return old, nil
	}
	front, err := dumpFiled(n)
	if err != nil {
		return nil, err
	}
	// An empty superseded_by or reason is written into a new note, but a note
	// that lacks the key is not given an empty one by a rewrite.
	have := map[string]bool{}
	for _, sp := range frontSpans(f.front) {
		have[sp.key] = true
	}
	for _, k := range []string{"superseded_by", "reason"} {
		if !have[k] {
			front = strings.Replace(front, "\n"+k+": ''\n", "\n", 1)
		}
	}
	f.front = patchFront(f, front, filedKey)
	f.body = patchBody(f.body, n, f.eol)
	return f.bytes(), nil
}

// patchBody changes the body of a filed note only as far as n differs from it: a
// new body replaces the old one, and links n has that the file does not are added
// to its ## Links section, one line each.
func patchBody(raw string, n Note, eol string) string {
	plain, have := splitLinks(lfText(raw), n.Level)
	changed := strings.TrimRight(plain, "\n") != strings.TrimRight(lfText(n.Body), "\n")
	var add []Link
	for _, l := range n.Links {
		if !slices.ContainsFunc(have, func(h Link) bool { return h.To == l.To && h.Rel == l.Rel }) {
			add = append(add, l)
		}
	}
	switch {
	case changed:
		return strings.ReplaceAll(bodyWithLinks(n.Body, n.Links, n.Level), "\n", eol)
	case len(add) == 0:
		return raw
	}
	lines := strings.SplitAfter(raw, "\n")
	if h, e, ok := lastLinksSection(lines); ok {
		at := e
		for at > h+1 && strings.TrimSpace(lines[at-1]) == "" {
			at--
		}
		var ins []string
		if at == h+1 { // a heading with nothing under it
			ins = append(ins, eol)
		}
		for _, l := range add {
			ins = append(ins, linkLine(l, n.Level)+eol)
		}
		if !strings.HasSuffix(lines[at-1], "\n") {
			lines[at-1] += eol
		}
		return strings.Join(slices.Insert(lines, at, ins...), "")
	}
	out := raw
	if out != "" && !strings.HasSuffix(out, "\n") {
		out += eol
	}
	if strings.TrimSpace(out) != "" {
		out += eol
	}
	return out + "## Links" + eol + eol + linkLines(add, n.Level, eol)
}

// bodyWithLinks is the body of a filed note as Grid writes it: the text, then a
// ## Links section when there are links. It has LF line endings.
func bodyWithLinks(body string, links []Link, level string) string {
	text := strings.TrimRight(body, "\n")
	out := text + "\n"
	if len(links) > 0 {
		if text != "" {
			out += "\n"
		}
		out += "## Links\n\n" + linkLines(links, level, "\n")
	}
	return out
}

// linkLine is "- [[target]] - rel: why", the line Grid writes and reads back. The
// target is the link's id as it is (brain:<level>/<id>, job:<task>/<job> or
// person:<id>), which Grid resolves at either level; a bare [[id]] a person types
// is read, and renamed, all the same.
//
// The link's summary is the display text, [[target|summary]], which is what
// Obsidian shows for the link. A summary that cannot be one line of display text
// is left out: the destination's own summary is what a reader shows.
func linkLine(l Link, level string) string {
	line := "- [[" + l.To + "]]"
	if s := strings.TrimSpace(l.Summary); s != "" && !strings.ContainsAny(s, "\r\n[]|") {
		line = "- [[" + l.To + "|" + s + "]]"
	}
	if why := strings.Join(strings.Fields(l.Why), " "); l.Rel != "related" || why != "" {
		line += " - " + l.Rel + ": " + why
	}
	return line
}

func linkLines(links []Link, level, eol string) string {
	var b strings.Builder
	for _, l := range links {
		b.WriteString(linkLine(l, level) + eol)
	}
	return b.String()
}

func headingOf(line string) (level int, text string, ok bool) {
	h := headingLine.FindStringSubmatch(strings.TrimRight(line, "\r\n"))
	if h == nil {
		return 0, "", false
	}
	return len(h[1]), h[2], true
}

// lastLinksSection finds the last "Links" heading outside code and the lines it
// owns: [h, e) holds the heading and runs to the next heading of the same or a
// higher rank, or to the end.
func lastLinksSection(lines []string) (h, e int, ok bool) {
	fence, rank := "", 0
	e = -1
	for i, l := range lines {
		t := strings.TrimLeft(l, " ")
		if fence != "" {
			if strings.HasPrefix(t, fence) {
				fence = ""
			}
			continue
		}
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fence = t[:3]
			continue
		}
		lv, text, isHead := headingOf(l)
		if !isHead {
			continue
		}
		if e < 0 && ok && lv <= rank {
			e = i
		}
		if strings.EqualFold(text, "links") {
			h, rank, ok, e = i, lv, true, -1
		}
	}
	if ok && e < 0 {
		e = len(lines)
	}
	return h, e, ok
}

// linkTo maps a link read from a ## Links line back to the id a Link carries.
func linkTo(m linkMatch, level string) string {
	to := m.target
	if !strings.HasPrefix(to, "job:") && !strings.HasPrefix(to, "person:") {
		lv := m.level
		if lv == "" {
			lv = level
		}
		to = "brain:" + lv + "/" + to
	}
	if m.anchor != "" {
		to += "#" + m.anchor
	}
	return to
}

// splitLinks takes the ## Links section out of a body, when it is the section Grid
// writes: every line in it a list item that holds a link and, after it, a known
// relation and why. Anything else in a section of that name is the person's own
// text and stays in the body. body has LF line endings.
func splitLinks(body, level string) (string, []Link) {
	if level != "home" && level != "project" {
		return body, nil
	}
	lines := strings.SplitAfter(body, "\n")
	h, e, ok := lastLinksSection(lines)
	if !ok {
		return body, nil
	}
	var links []Link
	for _, raw := range lines[h+1 : e] {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		line := maskBytes(strings.TrimRight(raw, "\n"))
		ms := findLinks(line)
		if len(ms) == 0 || !listItem.MatchString(line) {
			return body, nil
		}
		l := Link{To: linkTo(ms[0], level), Rel: "related"}
		if w := wikiLink.FindStringSubmatch(line); w != nil {
			if _, alias, ok := strings.Cut(strings.ReplaceAll(w[1], `\|`, `|`), "|"); ok {
				l.Summary = strings.TrimSpace(alias)
			}
		}
		if t := linksTail.FindStringSubmatch(line[ms[0].end:]); t != nil && knownRelation(t[1]) {
			l.Rel, l.Why = t[1], t[2]
		} else if strings.TrimSpace(line[ms[0].end:]) != "" {
			return body, nil
		}
		if !linkTarget(l.To) {
			return body, nil
		}
		links = append(links, l)
	}
	before, after := strings.Join(lines[:h], ""), strings.Join(lines[e:], "")
	if after == "" {
		before = strings.TrimRight(before, "\n") + "\n"
	}
	return before + after, links
}

// maskBytes blanks inline code spans like maskCode but keeps every byte offset,
// so a match found in the masked line can be replaced in the real one.
func maskBytes(line string) string {
	r := []byte(line)
	for i := 0; i < len(r); i++ {
		if r[i] != '`' {
			continue
		}
		n := 0
		for i+n < len(r) && r[i+n] == '`' {
			n++
		}
		end := -1
		for j := i + n; j < len(r); j++ {
			if r[j] != '`' {
				continue
			}
			k := 0
			for j+k < len(r) && r[j+k] == '`' {
				k++
			}
			if k == n {
				end = j + k
				break
			}
			j += k - 1
		}
		if end < 0 {
			i += n - 1
			continue
		}
		for x := i; x < end; x++ {
			r[x] = ' '
		}
		i = end - 1
	}
	return string(r)
}

// retarget moves a link path that names the note topic/oldName to newName, the
// other parts of the path as written. A path is the bare name, topic/name,
// knowledge/topic/name or brain:<level>/name; a ".md" ending is kept.
func retarget(path, level, topic, oldName, newName string) (string, bool) {
	stem, ext := path, ""
	if strings.EqualFold(path[max(0, len(path)-3):], ".md") {
		stem, ext = path[:len(path)-3], path[len(path)-3:]
	}
	prefix, last := "", stem
	if i := strings.LastIndex(stem, "/"); i >= 0 {
		prefix, last = stem[:i+1], stem[i+1:]
	}
	if last != oldName {
		return path, false
	}
	switch prefix {
	case "", topic + "/", "knowledge/" + topic + "/", "brain:" + level + "/":
		return prefix + newName + ext, true
	}
	return path, false
}

type linkEdit struct {
	start, end int
	text       string
}

// rewriteLine moves every link on one line from topic/oldName to newName and
// reports whether it changed anything. The display text after a | and the anchor
// after a # are kept; links in inline code are not links.
func rewriteLine(line, level, topic, oldName, newName string) (string, bool) {
	masked := maskBytes(line)
	var edits []linkEdit
	for _, m := range wikiLink.FindAllStringSubmatchIndex(masked, -1) {
		if masked[m[0]:m[1]] != line[m[0]:m[1]] {
			continue
		}
		inner := line[m[2]:m[3]]
		cut := len(inner)
		if i := strings.Index(inner, "|"); i >= 0 {
			cut = i
			if i > 0 && inner[i-1] == '\\' {
				cut = i - 1
			}
		}
		head, tail := inner[:cut], inner[cut:]
		pathPart, anchor := head, ""
		if i := strings.Index(head, "#"); i >= 0 {
			pathPart, anchor = head[:i], head[i:]
		}
		trimmed := strings.TrimSpace(pathPart)
		if next, ok := retarget(trimmed, level, topic, oldName, newName); ok && trimmed != "" {
			lead := pathPart[:strings.Index(pathPart, trimmed)]
			edits = append(edits, linkEdit{m[2], m[3], lead + next + pathPart[len(lead)+len(trimmed):] + anchor + tail})
		}
	}
	for _, m := range mdLink.FindAllStringSubmatchIndex(masked, -1) {
		raw := line[m[2]:m[3]]
		angled := strings.HasPrefix(raw, "<")
		inner := strings.Trim(raw, "<>")
		if strings.Contains(inner, "://") || strings.HasPrefix(strings.ToLower(inner), "mailto:") {
			continue
		}
		dec := inner
		if d, err := url.PathUnescape(inner); err == nil {
			dec = d
		}
		path, anchor, hasAnchor := strings.Cut(dec, "#")
		if !strings.EqualFold(path[max(0, len(path)-3):], ".md") {
			continue
		}
		next, ok := retarget(path, level, topic, oldName, newName)
		if !ok {
			continue
		}
		if dec != inner {
			next = strings.ReplaceAll(url.PathEscape(next), "%2F", "/")
		}
		if hasAnchor {
			next += "#" + anchor
		}
		if angled {
			next = "<" + next + ">"
		}
		edits = append(edits, linkEdit{m[2], m[3], next})
	}
	if len(edits) == 0 {
		return line, false
	}
	slices.SortFunc(edits, func(a, b linkEdit) int { return b.start - a.start })
	for _, e := range edits {
		line = line[:e.start] + e.text + line[e.end:]
	}
	return line, true
}

// RewriteLinks moves the links in text, a file's front matter or body with its
// own line endings, from the note topic/oldName to newName, and reports whether
// any moved. Links in fenced code and in inline code stay as they are.
func rewriteLinks(text, level, topic, oldName, newName string, fenced bool) (string, bool) {
	lines := strings.SplitAfter(text, "\n")
	changed, fence := false, ""
	for i, l := range lines {
		if fenced {
			t := strings.TrimLeft(l, " ")
			if fence != "" {
				if strings.HasPrefix(t, fence) {
					fence = ""
				}
				continue
			}
			if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
				fence = t[:3]
				continue
			}
		}
		if next, ok := rewriteLine(l, level, topic, oldName, newName); ok {
			lines[i], changed = next, true
		}
	}
	return strings.Join(lines, ""), changed
}

// RewriteNoteLinks moves the links of a whole note file, with or without front
// matter, from topic/oldName to newName. Nothing else in the file changes.
func rewriteNoteLinks(data []byte, level, topic, oldName, newName string) ([]byte, bool) {
	f, ok := splitRaw(data)
	if !ok {
		out, changed := rewriteLinks(string(data), level, topic, oldName, newName, true)
		return []byte(out), changed
	}
	front, c1 := rewriteLinks(strings.Join(f.front, ""), level, topic, oldName, newName, false)
	body, c2 := rewriteLinks(f.body, level, topic, oldName, newName, true)
	if !c1 && !c2 {
		return data, false
	}
	f.front, f.body = strings.SplitAfter(front, "\n"), body
	if n := len(f.front); n > 0 && f.front[n-1] == "" {
		f.front = f.front[:n-1]
	}
	return f.bytes(), true
}
