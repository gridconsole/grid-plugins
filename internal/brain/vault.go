package brain

// The vault reader: any .md file in the brain folder is a note, its id is its
// path, and every [[link]] in the body is an edge. See
// docs/research/brain-obsidian.md sections 2.1, 3.2-3.4 and 4.1-4.2. Nothing here
// writes: the write paths keep their old layout until the writer lanes move them.

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// Edge is one link out of a note. To is the target's full id (brain:<level>/<id>)
// when the link resolves to a file, else the target as written. Anchor is the
// heading or block after "#".
type Edge struct {
	To       string `json:"to"`
	Rel      string `json:"rel"`
	Why      string `json:"why"`
	Anchor   string `json:"anchor"`
	Resolved bool   `json:"resolved"`
}

// ListRow is one file in the listing `grid brain list --json` prints (section
// 3.4). Every field is always present; lists are [] when empty.
type ListRow struct {
	ID          string   `json:"id"`
	Path        string   `json:"path"`
	Kind        string   `json:"kind"`
	Origin      string   `json:"origin"`
	Author      string   `json:"author"`
	Level       string   `json:"level"`
	Project     string   `json:"project"`
	Task        string   `json:"task"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Topic       string   `json:"topic"`
	Tags        []string `json:"tags"`
	Aliases     []string `json:"aliases"`
	Concepts    []string `json:"concepts"`
	Edges       []Edge   `json:"edges"`
	Rev         string   `json:"rev"`
	Updated     string   `json:"updated"`
	Conflict    []string `json:"conflict"`
}

// Listing is the envelope of `grid brain list --json`: the brain root once, then
// one row per file in path order.
type Listing struct {
	Root  string    `json:"root"`
	Notes []ListRow `json:"notes"`
}

// rawLink is a link as read from text, before it is resolved.
type rawLink struct {
	Level  string // "home", "project" or "" for the note's own level
	Target string // without .md, without #anchor
	Anchor string
	Rel    string
	Why    string
}

// VaultNote is what ParseVault reads from any Markdown file: front matter is
// optional, unknown keys are kept, and the Obsidian shapes of tags and aliases
// are accepted.
type VaultNote struct {
	Front       map[string]any
	Body        string
	BadFront    bool // the front matter block is not YAML; the body is still read
	Description string
	Summary     string
	Tags        []string
	Aliases     []string
	Concepts    []string
	Links       []rawLink // front-matter links first, then body links in order
}

// ParseVault is the lenient reader for a note a person wrote. It never refuses a
// file: no front matter is empty front matter, an unknown key is kept in Front,
// and front matter that is not YAML sets BadFront and leaves the rest empty.
func ParseVault(data []byte) (VaultNote, error) {
	front, body, err := mdfile.SplitOptional(data)
	if err != nil {
		return VaultNote{}, err
	}
	v := VaultNote{Front: map[string]any{}, Body: strings.ReplaceAll(body, "\r\n", "\n")}
	if strings.TrimSpace(string(front)) != "" {
		var m map[string]any
		if err := mdfile.Load(front, &m, false); err != nil || m == nil {
			v.BadFront = true
		} else {
			v.Front = m
		}
	}
	v.Description = frontString(v.Front["description"])
	v.Summary = frontString(v.Front["summary"])
	for _, t := range frontList(v.Front["tags"], true) {
		v.Tags = appendUnique(v.Tags, strings.TrimLeft(t, "#"))
	}
	v.Aliases = frontList(v.Front["aliases"], false)
	v.Concepts = frontList(v.Front["concepts"], false)
	v.Links = append(frontLinks(v.Front), bodyLinks(v.Body)...)
	for _, t := range inlineTags(v.Body) {
		v.Tags = appendUnique(v.Tags, t)
	}
	return v, nil
}

func frontString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(x)
	case time.Time:
		return x.Format("2006-01-02")
	case []any, map[string]any:
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// frontList reads a property Obsidian shows as a list: a YAML list, or one text
// value that holds a comma-separated list (and, for tags, a space-separated one).
func frontList(v any, spaces bool) []string {
	var out []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			out = appendUnique(out, s)
		}
	}
	switch x := v.(type) {
	case string:
		for _, part := range strings.Split(x, ",") {
			if spaces {
				for _, w := range strings.Fields(part) {
					add(w)
				}
			} else {
				add(part)
			}
		}
	case []any:
		for _, item := range x {
			if _, nested := item.(map[string]any); !nested && item != nil {
				add(frontString(item))
			}
		}
	}
	return out
}

func appendUnique(list []string, s string) []string {
	for _, have := range list {
		if have == s {
			return list
		}
	}
	return append(list, s)
}

// frontLinks reads the links of a note in front matter: the old list of
// {to, rel, why, summary} maps (3.3), and every quoted "[[x]]" value (3.2).
func frontLinks(front map[string]any) []rawLink {
	var out []rawLink
	if list, ok := front["links"].([]any); ok {
		for _, item := range list {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			to := frontString(m["to"])
			if to == "" {
				continue
			}
			rel, why := frontString(m["rel"]), frontString(m["why"])
			if rel == "" {
				rel = "related"
			}
			level, target, anchor := splitTarget(to)
			out = append(out, rawLink{Level: level, Target: target, Anchor: anchor, Rel: rel, Why: why})
		}
	}
	keys := make([]string, 0, len(front))
	for k := range front {
		if k != "links" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case string:
			for _, m := range findLinks(x) {
				out = append(out, rawLink{Level: m.level, Target: m.target, Anchor: m.anchor, Rel: "related"})
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	for _, k := range keys {
		walk(front[k])
	}
	return out
}

// splitTarget turns link text into level, target and anchor. A brain:<level>/
// prefix (old front-matter links) names the level; ".md" and "#anchor" are cut.
func splitTarget(raw string) (level, target, anchor string) {
	target = strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(target, "brain:"); ok {
		if l, r, found := strings.Cut(rest, "/"); found && (l == "home" || l == "project") {
			level, target = l, r
		}
	}
	target, anchor, _ = strings.Cut(target, "#")
	target = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(target), ".md"))
	return level, target, strings.TrimSpace(anchor)
}

type linkMatch struct {
	start, end int
	level      string
	target     string
	anchor     string
}

var wikiLink = regexp.MustCompile(`!?\[\[([^\[\]\n]+?)\]\]`)
var mdLink = regexp.MustCompile(`!?\[[^\]\n]*\]\(\s*(<[^>\n]+>|[^()\s]+)(?:\s+"[^"\n]*")?\s*\)`)

// findLinks returns the [[x]], [[x|y]], [[x#h]], ![[x]] and [t](x.md) links of
// one line of text, in order. The display text is ignored; the heading or block
// is the anchor. A link with no target ([[#heading]]) is not an edge.
func findLinks(line string) []linkMatch {
	var out []linkMatch
	for _, m := range wikiLink.FindAllStringSubmatchIndex(line, -1) {
		inner := strings.ReplaceAll(line[m[2]:m[3]], `\|`, `|`)
		inner, _, _ = strings.Cut(inner, "|")
		level, target, anchor := splitTarget(inner)
		if target != "" {
			out = append(out, linkMatch{start: m[0], end: m[1], level: level, target: target, anchor: anchor})
		}
	}
	for _, m := range mdLink.FindAllStringSubmatchIndex(line, -1) {
		raw := strings.Trim(line[m[2]:m[3]], "<>")
		if strings.Contains(raw, "://") || strings.HasPrefix(strings.ToLower(raw), "mailto:") {
			continue
		}
		if dec, err := url.PathUnescape(raw); err == nil {
			raw = dec
		}
		path, anchor, _ := strings.Cut(raw, "#")
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			continue
		}
		path = strings.TrimSuffix(path, path[len(path)-3:])
		if path = strings.TrimSpace(path); path != "" {
			out = append(out, linkMatch{start: m[0], end: m[1], target: path, anchor: strings.TrimSpace(anchor)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// maskCode blanks inline code spans, so a link or tag inside one is not read.
func maskCode(line string) string {
	r := []rune(line)
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

var headingLine = regexp.MustCompile(`^\s{0,3}(#{1,6})\s+(.*?)\s*#*\s*$`)
var linksTail = regexp.MustCompile(`^\s*[-\x{2013}\x{2014}]\s*([A-Za-z][A-Za-z-]*)\s*:\s*(.*?)\s*$`)
var listItem = regexp.MustCompile(`^\s*[-*+]\s+`)

// textLines runs fn on each line of body that is not inside a fenced code block,
// with inline code blanked.
func textLines(body string, fn func(line string)) {
	fence := ""
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimLeft(line, " ")
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
		fn(maskCode(line))
	}
}

// bodyLinks reads every link in the body (3.2). A "## Links" line of the form
// "- [[target]] - rel: why" gives its first link that rel and why; every other
// link is "related" with no why. Links in code are skipped.
func bodyLinks(body string) []rawLink {
	var out []rawLink
	inLinks, linksLevel := false, 0
	textLines(body, func(line string) {
		if h := headingLine.FindStringSubmatch(line); h != nil {
			level := len(h[1])
			if strings.EqualFold(h[2], "links") {
				inLinks, linksLevel = true, level
			} else if inLinks && level <= linksLevel {
				inLinks = false
			}
			return
		}
		matches := findLinks(line)
		for i, m := range matches {
			l := rawLink{Level: m.level, Target: m.target, Anchor: m.anchor, Rel: "related"}
			if i == 0 && inLinks && listItem.MatchString(line) {
				if t := linksTail.FindStringSubmatch(line[m.end:]); t != nil && knownRelation(t[1]) {
					l.Rel, l.Why = t[1], t[2]
				}
			}
			out = append(out, l)
		}
	})
	return out
}

func knownRelation(rel string) bool {
	for _, r := range relations {
		if r == rel {
			return true
		}
	}
	return false
}

var inlineTag = regexp.MustCompile(`(?:^|[\s(,;])#([\p{L}\p{N}_/-]+)`)

// inlineTags are the #tags in the body text, outside code and links. A tag of
// digits only is not a tag in Obsidian.
func inlineTags(body string) []string {
	var out []string
	textLines(body, func(line string) {
		for _, m := range findLinks(line) {
			line = line[:m.start] + strings.Repeat(" ", m.end-m.start) + line[m.end:]
		}
		for _, m := range inlineTag.FindAllStringSubmatch(line, -1) {
			if strings.Trim(m[1], "0123456789") != "" {
				out = appendUnique(out, m[1])
			}
		}
	})
	return out
}

var emphasis = strings.NewReplacer("**", "", "__", "", "*", "", "~~", "", "`", "")
var mdLinkText = regexp.MustCompile(`!?\[([^\]\n]*)\]\([^)\n]*\)`)
var htmlTag = regexp.MustCompile(`<[^>\n]+>`)

// plainText removes the Markdown of one line: links keep their display text (or
// their target), emphasis and code marks go.
func plainText(line string) string {
	line = wikiLink.ReplaceAllStringFunc(line, func(s string) string {
		if strings.HasPrefix(s, "!") {
			return ""
		}
		inner := strings.ReplaceAll(s[2:len(s)-2], `\|`, `|`)
		target, text, hasText := strings.Cut(inner, "|")
		if hasText {
			return text
		}
		target, _, _ = strings.Cut(target, "#")
		return target
	})
	line = mdLinkText.ReplaceAllString(line, "$1")
	line = htmlTag.ReplaceAllString(line, "")
	return strings.TrimSpace(emphasis.Replace(line))
}

var orderedItem = regexp.MustCompile(`^\d+[.)]\s+`)

// firstParagraph is the default description (4.2): the first paragraph of the
// body after an optional "# H1", with Markdown removed, cut at 200 characters on
// a word.
func firstParagraph(body string) string {
	var para []string
	started, seenH1 := false, false
	fence := ""
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if fence != "" {
			if strings.HasPrefix(line, fence) {
				fence = ""
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~"):
			if started {
				return clipWords(strings.Join(para, " "), 200)
			}
			fence = line[:3]
			continue
		case line == "":
			if started {
				return clipWords(strings.Join(para, " "), 200)
			}
			continue
		case strings.HasPrefix(line, "#") && headingLine.MatchString(line):
			if started {
				return clipWords(strings.Join(para, " "), 200)
			}
			seenH1 = true
			continue
		case line == "---" || line == "***" || line == "___":
			if started {
				return clipWords(strings.Join(para, " "), 200)
			}
			continue
		}
		_ = seenH1
		line = strings.TrimSpace(strings.TrimLeft(line, ">"))
		line = orderedItem.ReplaceAllString(listItem.ReplaceAllString(line, ""), "")
		if text := plainText(line); text != "" {
			para = append(para, text)
			started = true
		}
	}
	return clipWords(strings.Join(para, " "), 200)
}

func clipWords(s string, limit int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= limit {
		return string(r)
	}
	cut := limit
	for i := limit; i > 0; i-- {
		if r[i] == ' ' {
			cut = i
			break
		}
	}
	return strings.TrimSpace(string(r[:cut]))
}

// --- ids ---

var composed = map[rune]map[rune]rune{}

func init() {
	pairs := map[rune]string{
		0x0300: "AÀ EÈ IÌ OÒ UÙ aà eè iì oò uù",
		0x0301: "AÁ EÉ IÍ OÓ UÚ YÝ aá eé ií oó uú yý CĆ cć NŃ nń SŚ sś ZŹ zź LĹ lĺ RŔ rŕ",
		0x0302: "AÂ EÊ IÎ OÔ UÛ aâ eê iî oô uû CĈ cĉ GĜ gĝ HĤ hĥ JĴ jĵ SŜ sŝ WŴ wŵ YŶ yŷ",
		0x0303: "AÃ NÑ OÕ aã nñ oõ IĨ iĩ UŨ uũ",
		0x0304: "AĀ aā EĒ eē IĪ iī OŌ oō UŪ uū",
		0x0306: "AĂ aă GĞ gğ",
		0x0307: "CĊ cċ EĖ eė GĠ gġ ZŻ zż",
		0x0308: "AÄ EË IÏ OÖ UÜ aä eë iï oö uü yÿ YŸ",
		0x030A: "AÅ aå UŮ uů",
		0x030B: "OŐ oő UŰ uű",
		0x030C: "CČ cč DĎ dď EĚ eě NŇ nň RŘ rř SŠ sš TŤ tť ZŽ zž",
		0x0327: "CÇ cç SŞ sş TŢ tţ GĢ gģ KĶ kķ LĻ lļ NŅ nņ RŖ rŗ",
		0x0328: "AĄ aą EĘ eę",
	}
	for mark, list := range pairs {
		m := map[rune]rune{}
		for _, p := range strings.Fields(list) {
			r := []rune(p)
			m[r[0]] = r[1]
		}
		composed[mark] = m
	}
}

// NFC puts a base letter followed by a combining mark into its one-character
// form for the Latin letters of Western and Central European names. macOS
// stores file names decomposed, so "Café" on disk and "Café" in a link must be
// one id. This is not the full Unicode normal form (the module has no
// golang.org/x/text); other scripts pass through as they are.
func NFC(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if table, ok := composed[r]; ok && len(out) > 0 {
			if c, ok := table[out[len(out)-1]]; ok {
				out[len(out)-1] = c
				continue
			}
		}
		out = append(out, r)
	}
	return string(out)
}

// --- the walk ---

type vaultFile struct {
	level string
	path  string // slash path below the level root, as on disk
	rel   string // slash path below Root
	id    string // note id below the level root
	kind  string
	topic string
	data  []byte
	mod   time.Time
	note  VaultNote
}

// brainOpenDir opens the folder of a level of the brain folder: the folder itself
// at home, projects/<name> at project level.
func (s Store) vaultLevel(level string) (root *os.Root, prefix string, err error) {
	top, err := os.OpenRoot(s.BrainDir)
	if err != nil {
		return nil, "", err
	}
	if level == "home" {
		return top, "", nil
	}
	if level != "project" || !ValidName(s.Project) {
		top.Close()
		return nil, "", fmt.Errorf("project level needs a valid project name")
	}
	prefix = "projects/" + s.Project
	defer top.Close()
	if err := safefile.NoSymlinks(top, prefix); err != nil {
		return nil, "", err
	}
	root, err = top.OpenRoot(prefix)
	return root, prefix, err
}

// vaultKind names what a file is by where it sits (3.4 kinds).
func vaultKind(parts []string) string {
	if parts[len(parts)-1] == "TOPIC.md" {
		return "topic"
	}
	if parts[0] != "grid" || len(parts) < 2 {
		return "note"
	}
	switch parts[1] {
	case "agents":
		return "agent"
	case "skills":
		return "skill"
	case "workflows":
		return "workflow"
	case "routines":
		return "routine"
	case "people":
		return "person"
	case "machines":
		return "machine"
	case "accounts":
		return "account"
	}
	return "config"
}

// vaultFiles walks one level of the brain folder at any depth. It skips every
// dot-folder (.obsidian, .trash, .grid), dot-files, Proposals/ and, at home
// level, projects/, and every file that is not .md. It refuses symbolic links.
func (s Store) vaultFiles(level string) ([]vaultFile, error) {
	root, prefix, err := s.vaultLevel(level)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var files []vaultFile
	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() && (path == "Proposals" || (level == "home" && path == "projects")) {
			return fs.SkipDir
		}
		if d.Type()&os.ModeSymlink != 0 {
			// WalkDir does not follow a link, so a link to a folder shows as a file.
			linked := strings.EqualFold(filepath.Ext(name), ".md")
			if info, err := os.Stat(filepath.Join(root.Name(), filepath.FromSlash(path))); err == nil && info.IsDir() {
				linked = true
			}
			if linked {
				return fmt.Errorf("brain paths cannot be symbolic links")
			}
			return nil
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
		b, err := safefile.Read(root, path)
		if err != nil {
			return err
		}
		note, err := ParseVault(b)
		if err != nil {
			return err
		}
		parts := strings.Split(path, "/")
		f := vaultFile{level: level, path: path, rel: path, id: NFC(strings.TrimSuffix(path, path[len(path)-3:])),
			kind: vaultKind(parts), topic: "-", data: b, note: note}
		if prefix != "" {
			f.rel = prefix + "/" + path
		}
		if len(parts) > 1 {
			f.topic = NFC(parts[0])
		}
		if info, err := d.Info(); err == nil {
			f.mod = info.ModTime()
		}
		files = append(files, f)
		return nil
	})
	return files, err
}

// legacyFiles is the old layout seen as vault files: the valid accepted notes of
// knowledge/<topic>/<name>.md, with the id the old reader gives them (the name).
func (s Store) legacyFiles(level string) ([]vaultFile, error) {
	root, err := s.scope(level, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	notes, _, err := scan(root, level)
	if err != nil {
		return nil, err
	}
	dropped, err := Dropped(root)
	if err != nil {
		return nil, err
	}
	prefix, _ := s.prefix(level)
	var files []vaultFile
	for _, n := range notes {
		if dropped[n.Topic+"/"+n.Name] || n.SupersededBy != "" {
			continue
		}
		path := "knowledge/" + n.Topic + "/" + n.Name + ".md"
		b, err := safefile.Read(root, path)
		if err != nil {
			return nil, err
		}
		note, err := ParseVault(b)
		if err != nil {
			return nil, err
		}
		f := vaultFile{level: level, path: path, rel: filepath.ToSlash(filepath.Join(prefix, path)), id: n.Name,
			kind: "note", topic: n.Topic, data: b, note: note}
		if info, err := root.Stat(path); err == nil {
			f.mod = info.ModTime()
		}
		files = append(files, f)
	}
	return files, nil
}

func (s Store) filesOf(level string) ([]vaultFile, error) {
	if !s.Vault() {
		return s.legacyFiles(level)
	}
	files, err := s.vaultFiles(level)
	if err != nil {
		return nil, err
	}
	// Accept still publishes under GridHome/knowledge, so those notes stay visible.
	legacy, err := s.legacyFiles(level)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.id] = true
	}
	for _, f := range legacy {
		if !seen[f.id] {
			files = append(files, f)
		}
	}
	return files, nil
}

// --- the lookup (2.1) ---

type levelIndex struct {
	ids   []string
	exact map[string]bool
	lower map[string][]string
	base  map[string][]string
}

type lookup map[string]*levelIndex

func newLookup(files map[string][]vaultFile) lookup {
	l := lookup{}
	for level, list := range files {
		ix := &levelIndex{exact: map[string]bool{}, lower: map[string][]string{}, base: map[string][]string{}}
		for _, f := range list {
			ix.ids = append(ix.ids, f.id)
			ix.exact[f.id] = true
			ix.lower[strings.ToLower(f.id)] = append(ix.lower[strings.ToLower(f.id)], f.id)
			b := f.id[strings.LastIndex(f.id, "/")+1:]
			ix.base[b] = append(ix.base[b], f.id)
		}
		l[level] = ix
	}
	return l
}

// resolve finds the file a link target names in a level: an exact match on the
// NFC id; else one that ignores case; else, for a bare name, the one note with
// that file name; else the one note whose path ends in the target (Obsidian's
// shortest path). Two matches at one step is ambiguous and resolves to nothing.
func (l lookup) resolve(level, target string) (string, bool) {
	ix := l[level]
	target = NFC(strings.Trim(strings.TrimSpace(target), "/"))
	if ix == nil || target == "" {
		return "", false
	}
	if ix.exact[target] {
		return target, true
	}
	if m := ix.lower[strings.ToLower(target)]; len(m) > 0 {
		return only(m)
	}
	if !strings.Contains(target, "/") {
		if m := ix.base[target]; len(m) > 0 {
			return only(m)
		}
		var found []string
		for b, ids := range ix.base {
			if strings.EqualFold(b, target) {
				found = append(found, ids...)
			}
		}
		return only(found)
	}
	var found []string
	suffix := strings.ToLower("/" + target)
	for _, id := range ix.ids {
		if strings.HasSuffix(strings.ToLower(id), suffix) {
			found = append(found, id)
		}
	}
	return only(found)
}

func only(ids []string) (string, bool) {
	if len(ids) == 1 {
		return ids[0], true
	}
	return "", false
}

// --- rows ---

var attachmentExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true,
	".pdf": true, ".mp3": true, ".mp4": true, ".wav": true, ".ogg": true, ".webm": true, ".mov": true, ".m4a": true,
	".canvas": true, ".base": true, ".json": true, ".csv": true, ".txt": true, ".zip": true}

func (s Store) row(f vaultFile, lk lookup) ListRow {
	n := f.note
	title := f.id[strings.LastIndex(f.id, "/")+1:]
	r := ListRow{ID: "brain:" + f.level + "/" + f.id, Path: f.rel, Kind: f.kind, Origin: "person", Author: s.Author,
		Level: f.level, Title: title, Topic: f.topic, Rev: NoteRev(f.data), Conflict: []string{},
		Tags: []string{}, Aliases: []string{}, Concepts: []string{}, Edges: []Edge{}}
	if f.level == "project" {
		r.Project = s.Project
	}
	r.Description = n.Description
	if r.Description == "" {
		r.Description = firstParagraph(n.Body)
	}
	r.Summary = n.Summary
	if r.Summary == "" {
		r.Summary = r.Description
	}
	r.Tags = append(r.Tags, n.Tags...)
	r.Aliases = append(r.Aliases, n.Aliases...)
	r.Concepts = append(r.Concepts, n.Concepts...)
	if t := frontString(n.Front["updated"]); t != "" {
		r.Updated = t
	} else if !f.mod.IsZero() {
		r.Updated = contract.FormatTime(f.mod.UTC())
	}
	seen := map[Edge]bool{}
	for _, l := range n.Links {
		level := l.Level
		if level == "" {
			level = f.level
		}
		e := Edge{To: l.Target, Rel: l.Rel, Why: l.Why, Anchor: l.Anchor}
		if id, ok := lk.resolve(level, l.Target); ok {
			e.To, e.Resolved = "brain:"+level+"/"+id, true
		} else if !strings.Contains(l.Target, ":") && !attachmentExt[strings.ToLower(filepath.Ext(l.Target))] {
			r.Concepts = appendUnique(r.Concepts, l.Target)
		}
		if !seen[e] {
			seen[e] = true
			r.Edges = append(r.Edges, e)
		}
	}
	return r
}

// readFiles reads every level the store reads, the files of each.
func (s Store) readFiles() (map[string][]vaultFile, error) {
	s.adoptForRead()
	all := map[string][]vaultFile{}
	for _, level := range s.levels() {
		files, err := s.filesOf(level)
		if err != nil {
			return nil, err
		}
		all[level] = files
	}
	return all, nil
}

// List is the listing of the brain: one row per file in path order. With the
// brain folder key set it is every .md file of the vault; with the key absent it
// is the notes of the old knowledge/ layout.
func (s Store) List() (Listing, error) {
	all, err := s.readFiles()
	if err != nil {
		return Listing{}, err
	}
	lk := newLookup(all)
	out := Listing{Root: s.Root(), Notes: []ListRow{}}
	for _, files := range all {
		for _, f := range files {
			out.Notes = append(out.Notes, s.row(f, lk))
		}
	}
	if s.Vault() {
		out.Notes = s.decorate(out.Notes)
	}
	sort.SliceStable(out.Notes, func(i, j int) bool { return out.Notes[i].Path < out.Notes[j].Path })
	return out, nil
}

// vaultEntries are the notes of the vault as entries, project level first, for
// Show and Search. Only plain notes: a TOPIC.md or a file of grid/ is config and
// does not enter the knowledge index.
func (s Store) vaultEntries() ([]Entry, error) {
	all, err := s.readFiles()
	if err != nil {
		return nil, err
	}
	lk := newLookup(all)
	ix := s.originIndex()
	var entries []Entry
	for _, level := range s.levels() {
		for _, f := range all[level] {
			if f.kind != "note" {
				continue
			}
			r := s.row(f, lk)
			ix.apply(&r, level, f.id)
			n := Note{Name: f.id, Topic: f.topic, Description: r.Description, Summary: r.Summary, Updated: r.Updated,
				Level: level, Status: "accepted", Links: []Link{}, Body: f.note.Body}
			for _, e := range r.Edges {
				n.Links = append(n.Links, Link{To: e.To, Rel: e.Rel, Why: e.Why})
			}
			rc := r
			entries = append(entries, Entry{ID: r.ID, Path: r.Path, Note: n, row: &rc})
		}
	}
	return entries, nil
}

// vaultShow finds a note by id: brain:<level>/<id>, or an id alone, which is read
// project first. The lookup order is the one of 2.1.
func (s Store) vaultShow(id string) (Entry, error) {
	level, name := "", NFC(id)
	if rest, ok := strings.CutPrefix(id, "brain:"); ok {
		var found bool
		level, name, found = strings.Cut(rest, "/")
		if !found || (level != "home" && level != "project") {
			return Entry{}, fmt.Errorf("id must be brain:<home|project>/<id>")
		}
		name = NFC(name)
	}
	if !ValidID(strings.TrimSuffix(name, ".md")) {
		return Entry{}, fmt.Errorf("note id must be a path id or brain:<home|project>/<id>")
	}
	name = strings.TrimSuffix(name, ".md")
	entries, err := s.vaultEntries()
	if err != nil {
		return Entry{}, err
	}
	all := map[string][]vaultFile{}
	for _, e := range entries {
		all[e.Note.Level] = append(all[e.Note.Level], vaultFile{id: e.Note.Name})
	}
	lk := newLookup(all)
	for _, l := range s.levels() {
		if level != "" && level != l {
			continue
		}
		if got, ok := lk.resolve(l, name); ok {
			for _, e := range entries {
				if e.Note.Level == l && e.Note.Name == got {
					return e, nil
				}
			}
		}
	}
	// An id a person renamed away still resolves, to the note it became (5.2).
	for _, l := range s.levels() {
		if level != "" && level != l {
			continue
		}
		if to, moved := s.originIndex().renamedTo(l, name); moved {
			for _, e := range entries {
				if e.Note.Level == l && e.Note.Name == to {
					return e, nil
				}
			}
		}
	}
	return Entry{}, fmt.Errorf("accepted note %q not found", id)
}

// --- topics of a vault ---

// vaultTopics are the built-in topics that exist at a level and the custom ones:
// a top-level folder that holds a TOPIC.md. A folder without one holds notes that
// are listed and searched but has no row in SessionStart.
func (s Store) vaultTopics(files []vaultFile, level string) []Topic {
	var topics []Topic
	known := map[string]bool{}
	for _, t := range BuiltinTopics() {
		known[t.Name] = true
		if allows(t, level) {
			topics = append(topics, t)
		}
	}
	var custom []Topic
	seen := map[string]bool{}
	for _, f := range files {
		if f.kind != "topic" || f.id != f.topic+"/TOPIC" || f.topic == "grid" || known[f.topic] || seen[f.topic] {
			continue
		}
		seen[f.topic] = true
		desc := f.note.Description
		if desc == "" {
			desc = firstParagraph(f.note.Body)
		}
		if desc == "" {
			desc = "Custom knowledge"
		}
		custom = append(custom, Topic{Name: f.topic, Kind: "knowledge", Levels: []string{level}, Description: desc, Custom: true})
	}
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	return append(topics, custom...)
}

// vaultRows are the SessionStart rows of a level: the entries of a knowledge
// topic are the notes in its folder, those of a config topic the files of
// grid/<topic>/.
func (s Store) vaultRows(level string) ([]TopicRow, error) {
	files, err := s.vaultFiles(level)
	if err != nil {
		return nil, err
	}
	var rows []TopicRow
	for _, t := range s.vaultTopics(files, level) {
		r := TopicRow{Topic: t.Name, Description: t.Description}
		for _, f := range files {
			counted := false
			if t.Kind == "config" {
				counted = f.topic == "grid" && strings.HasPrefix(f.id, "grid/"+t.Name+"/") && f.kind != "topic"
			} else {
				counted = f.topic == t.Name && f.kind == "note"
			}
			if !counted {
				continue
			}
			r.Entries++
			updated := frontString(f.note.Front["updated"])
			if updated == "" && !f.mod.IsZero() {
				updated = contract.FormatTime(f.mod.UTC())
			}
			if updated > r.LastChange {
				r.LastChange = updated
			}
		}
		rows = append(rows, r)
	}
	return rows, nil
}

// --- the config key ---

// WithConfig fills BrainDir and Author from the shared Grid config: the
// workspace whose data folder is GridHome names the brain folder (its "brain"
// key) and the person's name. A key that is absent, or that names the data
// folder itself, leaves the old layout in place. getenv may be nil.
func (s Store) WithConfig(userHome string, getenv func(string) string) Store {
	if s.GridHome == "" || userHome == "" {
		return s
	}
	g := func(key string) string {
		if key == "GRID_HOME" {
			return s.GridHome
		}
		if getenv != nil {
			return getenv(key)
		}
		return ""
	}
	h := gridhome.Resolve(g, userHome)
	if s.BrainDir == "" && h.Brain != "" && filepath.Clean(h.Brain) != filepath.Clean(s.GridHome) {
		s.BrainDir = h.Brain
	}
	if s.Author == "" {
		s.Author = h.Member
	}
	if s.MachineDir == "" {
		s.MachineDir, s.MachineID = h.Machine, gridhome.Machine(h.Machine)
	}
	return s
}

// SkippedPath reports whether a path below a level root is one the walk never
// reads: a dot-folder or dot-file, Proposals/, or a file that is not .md.
func SkippedPath(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		if strings.HasPrefix(p, ".") {
			return true
		}
	}
	return parts[0] == "Proposals" || !strings.EqualFold(filepath.Ext(rel), ".md") || !utf8.ValidString(rel)
}

// VaultID is the level and note id of an absolute path inside the brain folder,
// as the Read hook records it: the path below the level root without .md. ok is
// false for a path outside the folder or one the walk skips. A path below
// projects/<p>/ is at project level when p is the store's project; below another
// project's folder it is not this store's note.
func (s Store) VaultID(abs string) (level, id string, ok bool) {
	if !s.Vault() {
		return "", "", false
	}
	// The folder or the file may be reached through a link: try both as given and
	// both resolved.
	rel := ""
	for _, root := range []string{s.BrainDir, gridhome.Real(s.BrainDir)} {
		for _, p := range []string{abs, gridhome.Real(abs)} {
			if r, err := filepath.Rel(root, p); err == nil && r != "." && r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator)) {
				rel = r
			}
		}
	}
	if rel == "" {
		return "", "", false
	}
	rel = filepath.ToSlash(rel)
	level = "home"
	if rest, found := strings.CutPrefix(rel, "projects/"); found {
		p, below, cut := strings.Cut(rest, "/")
		if !cut || p != s.Project || s.Project == "" {
			return "", "", false
		}
		level, rel = "project", below
	}
	if SkippedPath(rel) {
		return "", "", false
	}
	id = NFC(strings.TrimSuffix(rel, rel[len(rel)-3:]))
	return level, id, ValidID(id)
}
