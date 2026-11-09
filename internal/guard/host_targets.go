package guard

import (
	"path/filepath"
	"regexp"
	"strings"
)

type hostShellKind int

const (
	hostWord hostShellKind = iota
	hostSeparator
	hostReadRedirect
	hostWriteRedirect
	hostDupRedirect // >&: a file unless the operand is an fd number or -
)

type hostShellToken struct {
	text string
	kind hostShellKind
	// groups are the commands inside $(..), backticks, <(..) and >(..) in the
	// word, and any of those nested in ${..}. Nothing runs them; they are scanned.
	groups []string
}

// hostMaxGroupDepth bounds nested substitutions; deeper text fails closed.
const hostMaxGroupDepth = 16

// hostShellGroup finds the end of a quote, substitution or brace group that
// starts at start. An unfinished group consumes the remaining text.
func hostShellGroup(text string, start int, close byte) (int, bool) {
	for i := start; i < len(text); i++ {
		switch {
		case text[i] == '\\':
			i++
		case text[i] == close:
			return i + 1, true
		case close == '\'':
		case text[i] == '\'' || text[i] == '"' || text[i] == '`':
			end, _ := hostShellGroup(text, i+1, text[i])
			i = end - 1
		case text[i] == '(':
			end, _ := hostShellGroup(text, i+1, ')')
			i = end - 1
		case text[i] == '{':
			end, _ := hostShellGroup(text, i+1, '}')
			i = end - 1
		}
	}
	return len(text), false
}

// hostGroupBody returns the text a substitution runs; backticks unescape \`, \$ and \\.
func hostGroupBody(text string, start, end int, closed bool, close byte) string {
	if closed {
		end--
	}
	body := text[start:end]
	if close != '`' {
		return body
	}
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) && strings.IndexByte("`$\\", body[i+1]) >= 0 {
			i++
		}
		b.WriteByte(body[i])
	}
	return b.String()
}

func hostDigits(text string) bool {
	return text != "" && strings.Trim(text, "0123456789") == ""
}

// hostShellTokens removes quoting but preserves glob and expansion markers.
// This is deliberately conservative even for quoted markers, not a shell parser.
// Redirect operators come out whole: >, >>, >|, >&, &>, &>>, <>, <, <<, <<<, <&,
// each with an optional leading fd number, which is dropped.
func hostShellTokens(command string) []hostShellToken {
	var out []hostShellToken
	var word strings.Builder
	var groups []string
	flush := func() {
		if word.Len() > 0 {
			out = append(out, hostShellToken{text: word.String(), groups: groups})
		}
		word.Reset()
		groups = nil
	}
	emit := func(kind hostShellKind, text string) {
		flush()
		out = append(out, hostShellToken{text: text, kind: kind})
	}
	at := func(i int) byte {
		if i < len(command) {
			return command[i]
		}
		return 0
	}
	var quote byte
	for i := 0; i < len(command); i++ {
		ch := command[i]
		subst := ch == '`' || (ch == '$' && (at(i+1) == '(' || at(i+1) == '{')) || ((ch == '<' || ch == '>') && quote == 0 && at(i+1) == '(')
		if subst {
			start, close := i+1, byte('`')
			if ch != '`' {
				start, close = i+2, ')'
				if ch == '$' && command[i+1] == '{' {
					close = '}'
				}
			}
			end, closed := hostShellGroup(command, start, close)
			body := hostGroupBody(command, start, end, closed, close)
			if close == '}' {
				for _, tok := range hostShellTokens(body) {
					groups = append(groups, tok.groups...)
				}
			} else {
				groups = append(groups, body)
			}
			word.WriteString(command[i:end])
			i = end - 1
			continue
		}
		if ch == '\\' && quote != '\'' && i+1 < len(command) {
			i++
			if command[i] != '\n' {
				word.WriteByte(command[i])
			}
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			} else {
				word.WriteByte(ch)
			}
			continue
		}
		switch ch {
		case '\'', '"':
			quote = ch
		case ' ', '\t', '\r':
			flush()
		case '\n', ';', '(', ')':
			emit(hostSeparator, string(ch))
		case '|':
			text := "|"
			if next := at(i + 1); next == '|' || next == '&' {
				text += string(next)
				i++
			}
			emit(hostSeparator, text)
		case '&':
			switch at(i + 1) {
			case '>':
				text := "&>"
				i++
				if at(i+1) == '>' {
					text += ">"
					i++
				}
				emit(hostWriteRedirect, text)
			case '&':
				i++
				emit(hostSeparator, "&&")
			default:
				emit(hostSeparator, "&")
			}
		case '<', '>':
			if len(groups) == 0 && hostDigits(word.String()) {
				word.Reset() // 2>file, 1<>file: the digits name an fd
			}
			kind, text := hostReadRedirect, string(ch)
			switch next := at(i + 1); {
			case ch == '<' && next == '<':
				text = "<<"
				i++
				if third := at(i + 1); third == '<' || third == '-' {
					text += string(third)
					i++
				}
			case ch == '<' && next == '>':
				kind, text = hostWriteRedirect, "<>"
				i++
			case ch == '<' && next == '&':
				text = "<&"
				i++
			case ch == '>':
				kind = hostWriteRedirect
				switch next {
				case '>', '|':
					text += string(next)
					i++
				case '&':
					kind, text = hostDupRedirect, ">&"
					i++
				}
			}
			emit(kind, text)
		default:
			word.WriteByte(ch)
		}
	}
	flush()
	return out
}

// hostSegment is one simple command: its words, redirect operands and the
// separator that ended it.
type hostSegment struct {
	assigns, args, writes, reads []hostShellToken
	separator                    string
}

var hostFdOperand = regexp.MustCompile(`^(?:[0-9]+-?|-)$`)

func hostSegments(command string) []hostSegment {
	tokens := hostShellTokens(command)
	var out []hostSegment
	var seg hostSegment
	for i := 0; i < len(tokens); i++ {
		switch tok := tokens[i]; tok.kind {
		case hostSeparator:
			seg.separator = tok.text
			out = append(out, seg)
			seg = hostSegment{}
		case hostWord:
			if len(seg.args) == 0 && strings.Contains(tok.text, "=") {
				seg.assigns = append(seg.assigns, tok)
			} else {
				seg.args = append(seg.args, tok)
			}
		default:
			if i+1 >= len(tokens) || tokens[i+1].kind != hostWord {
				continue
			}
			i++
			operand := tokens[i]
			switch {
			case tok.kind == hostReadRedirect:
				seg.reads = append(seg.reads, operand)
			case tok.kind == hostDupRedirect && hostFdOperand.MatchString(operand.text):
			default:
				seg.writes = append(seg.writes, operand)
			}
		}
	}
	return append(out, seg)
}

func (g hostSegment) tokens() []hostShellToken {
	all := append([]hostShellToken{}, g.assigns...)
	all = append(all, g.args...)
	all = append(all, g.writes...)
	return append(all, g.reads...)
}

// readCommand reports a command that only reads, whatever its redirects do.
func (g hostSegment) readCommand() bool {
	if len(g.args) == 0 {
		return true
	}
	name := strings.ToLower(filepath.Base(g.args[0].text))
	if name == "find" {
		for _, arg := range g.args[1:] {
			if findWrites[arg.text] {
				return false
			}
		}
	}
	return readOnly[name]
}

func (g hostSegment) readsOnly() bool { return len(g.writes) == 0 && g.readCommand() }

// hostProtectedNames are the entries a write must not reach: matched as a
// basename pattern when the target is a glob or holds an expansion.
var hostProtectedNames = []string{"workflows.md", "workflow.md", "task.md", "settings.json", ".settings.lock", "approval.json", "approval-ship.json", ".task-write.json", ".task.lock", "questions", "plan-revisions"}

func hostProtectedPattern(pattern string) bool {
	for _, name := range hostProtectedNames {
		if match, _ := filepath.Match(pattern, name); match {
			return true
		}
	}
	return false
}

// hostBadBracket reports a bracket expression that Go's filepath.Match would
// read differently from a shell: a character class or an unfinished bracket.
func hostBadBracket(pattern string) bool {
	if !strings.Contains(pattern, "[") {
		return false
	}
	if strings.Contains(pattern, "[:") || strings.Contains(pattern, "[=") || strings.Contains(pattern, "[.") {
		return true
	}
	_, err := filepath.Match(strings.ReplaceAll(pattern, "[!", "[^"), "")
	return err != nil
}

// hostStarBrackets replaces each bracket expression with *, a superset of what
// it matches. Character classes such as [:alpha:] stay inside their bracket.
func hostStarBrackets(word string) string {
	var b strings.Builder
	for i := 0; i < len(word); i++ {
		if word[i] != '[' {
			b.WriteByte(word[i])
			continue
		}
		j := i + 1
		if j < len(word) && (word[j] == '!' || word[j] == '^') {
			j++
		}
		if j < len(word) && word[j] == ']' {
			j++
		}
		for j < len(word) && word[j] != ']' {
			if word[j] == '[' && j+1 < len(word) && strings.IndexByte(":=.", word[j+1]) >= 0 {
				if end := strings.Index(word[j+2:], string(word[j+1])+"]"); end >= 0 {
					j += end + 4
					continue
				}
			}
			j++
		}
		b.WriteByte('*')
		i = j
	}
	return b.String()
}

// hostStarExpansions replaces each expansion with * so the rest of the word can
// be read as a pattern.
func hostStarExpansions(word string) string {
	var b strings.Builder
	for i := 0; i < len(word); i++ {
		ch := word[i]
		switch {
		case ch == '`':
			end, _ := hostShellGroup(word, i+1, '`')
			i = end - 1
			b.WriteByte('*')
		case ch == '$' && i+1 < len(word) && (word[i+1] == '(' || word[i+1] == '{'):
			close := byte(')')
			if word[i+1] == '{' {
				close = '}'
			}
			end, _ := hostShellGroup(word, i+2, close)
			i = end - 1
			b.WriteByte('*')
		case ch == '$':
			j := i + 1
			for j < len(word) && (word[j] == '_' || word[j] >= '0' && word[j] <= '9' || word[j] >= 'a' && word[j] <= 'z' || word[j] >= 'A' && word[j] <= 'Z') {
				j++
			}
			if j == i+1 && j < len(word) {
				j++ // $@, $?, $$
			}
			i = j - 1
			b.WriteByte('*')
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// writeScan checks shell commands for writes to protected Grid files. It is a
// conservative static guard: nothing is executed or expanded beyond a few known
// variables.
type writeScan struct {
	c     context
	roots []string // loaded on first use: resolving them reads the file system
}

func (s *writeScan) gridRoots() []string {
	if s.roots == nil {
		s.roots = s.c.gridRoots()
	}
	return s.roots
}

var hostKnownVar = regexp.MustCompile(`\$(?:\{(HOME|GRID_HOME|GRID_PROJECTS|PWD)\}|(HOME|GRID_HOME|GRID_PROJECTS|PWD)\b)`)

// protectedWriteReason checks redirect targets and possible file operands of
// commands not known to be read-only. A cd updates only subsequent commands;
// reads and echo/printf data are never treated as write targets. The inside of
// every substitution is a command of its own.
func (c context) protectedWriteReason(command string) string {
	return (&writeScan{c: c}).run(command, c.in.Cwd, false, 0)
}

func (s *writeScan) under(path string) bool {
	path = strings.ToLower(realNative(path, 0))
	for _, root := range s.gridRoots() {
		if inside(root, path) {
			return true
		}
	}
	return false
}

// mayReach also counts a path that stops inside the name of a Grid root, since
// a glob can finish it.
func (s *writeScan) mayReach(path string) bool {
	path = strings.ToLower(realNative(path, 0))
	for _, root := range s.gridRoots() {
		if inside(root, path) || strings.HasPrefix(root, path) {
			return true
		}
	}
	return false
}

func (s *writeScan) protected(path string) string {
	path = strings.ToLower(realNative(path, 0))
	for _, root := range s.gridRoots() {
		if settingsAt(root, path) {
			return switchReason
		}
		if projectWorkflowAt(root, path) || gateAt(root, path) {
			if filepath.Base(path) == "workflow.md" {
				return workflowReason
			}
			return gateReason
		}
	}
	return ""
}

// fill puts the values the guard holds into a word: ~, ~+ and HOME, GRID_HOME,
// GRID_PROJECTS and PWD. Any other variable stays as written.
func (s *writeScan) fill(word, base string, uncertain bool) string {
	word = s.c.expand(word)
	if !uncertain && (word == "~+" || strings.HasPrefix(word, "~+/")) {
		word = base + word[2:]
	}
	return hostKnownVar.ReplaceAllStringFunc(word, func(match string) string {
		name := strings.Trim(match, "${}")
		switch {
		case name == "PWD" && uncertain:
		case name == "PWD":
			return base
		case s.c.env[name] != "":
			return s.c.env[name]
		}
		return match
	})
}

func (s *writeScan) run(command, base string, uncertain bool, depth int) string {
	if depth > hostMaxGroupDepth {
		return gateReason
	}
	for _, seg := range hostSegments(command) {
		for _, tok := range seg.tokens() {
			for _, body := range tok.groups {
				if reason := s.run(body, base, uncertain, depth+1); reason != "" {
					return reason
				}
			}
		}
		for _, target := range seg.writes {
			if reason := s.check(target.text, base, uncertain, true); reason != "" {
				return reason
			}
		}
		if len(seg.args) == 0 {
			continue
		}
		if !seg.readCommand() {
			for _, arg := range seg.args[1:] {
				if strings.HasPrefix(arg.text, "-") && !strings.Contains(arg.text, "=") {
					continue
				}
				if reason := s.check(arg.text, base, uncertain, false); reason != "" {
					return reason
				}
			}
		}
		if strings.ToLower(filepath.Base(seg.args[0].text)) == "cd" && (seg.separator == "&&" || seg.separator == ";" || seg.separator == "\n") {
			base, uncertain = s.cd(seg.args[1:], base, uncertain)
		}
	}
	return ""
}

// cd follows a directory change. Options are skipped; anything but one literal
// directory after them makes the base uncertain.
func (s *writeScan) cd(args []hostShellToken, base string, uncertain bool) (string, bool) {
	for len(args) > 0 && len(args[0].text) > 1 && strings.HasPrefix(args[0].text, "-") {
		option := args[0].text
		args = args[1:]
		if option == "--" {
			break
		}
	}
	if len(args) != 1 {
		return base, true
	}
	dir := s.fill(args[0].text, base, uncertain)
	if dir == "-" {
		return base, true
	}
	if at := strings.IndexAny(dir, "$`"); at >= 0 {
		return base, uncertain || at == 0 || s.under(base) || s.under(resolve(base, dir[:at]))
	}
	return realNative(resolve(base, dir), 0), uncertain && !filepath.IsAbs(dir)
}

// check judges one word. A target (a redirect operand) is certainly written; any
// other operand of a non-read command might be. A key=value operand is checked
// whole and by its value, as in dd of=file and --output=file.
func (s *writeScan) check(word, base string, uncertain, target bool) string {
	if !target {
		if key, value, ok := strings.Cut(word, "="); ok && key != "" {
			if reason := s.one(word, base, false, false); reason != "" {
				return reason
			}
			word = value
		}
	}
	return s.one(word, base, uncertain, target)
}

func (s *writeScan) one(word, base string, uncertain, target bool) string {
	word = s.fill(word, base, uncertain)
	if word == "" {
		return ""
	}
	if uncertain && !filepath.IsAbs(word) {
		return gateReason
	}
	if at := strings.IndexAny(word, "$`"); at >= 0 {
		// Only the literal prefix is trustworthy. Do not clean away an
		// expansion/.. component before deciding containment.
		grid := (!filepath.IsAbs(word) && s.under(base)) || s.under(resolve(base, word[:at]))
		if target {
			if grid {
				return gateReason
			}
			return ""
		}
		// A plain operand is only a reason to look when an expansion could
		// stand for a protected name: $MSG or $ROOT/script.mjs cannot.
		if !grid {
			return ""
		}
		pattern := hostStarExpansions(word)
		name := strings.ToLower(filepath.Base(pattern))
		if strings.Trim(name, "*") == "" {
			return ""
		}
		if strings.ContainsAny(filepath.Dir(word), "$`") && hostProtectedPattern(strings.ReplaceAll(name, "[!", "[^")) {
			return gateReason
		}
		word = pattern
	}
	if hostBadBracket(word) {
		// A redirect target is certainly written: refuse under a Grid root. An
		// operand may be sed or python text, so read each bracket as * instead.
		if !target {
			word = hostStarBrackets(word)
		} else if s.mayReach(resolve(base, word[:strings.Index(word, "[")])) {
			return gateReason
		}
	}
	word = strings.ReplaceAll(word, "[!", "[^")
	path := resolve(base, word)
	if !strings.ContainsAny(path, "*?[") {
		return s.protected(path)
	}
	matches, _ := filepath.Glob(path)
	for _, match := range matches {
		if reason := s.protected(match); reason != "" {
			return reason
		}
	}
	// A policy need not exist yet. Match its reserved basename in each
	// possible directory as well as checking existing glob results.
	dirs := []string{filepath.Dir(path)}
	if strings.ContainsAny(dirs[0], "*?[") {
		matches, _ := filepath.Glob(dirs[0])
		dirs = append(dirs, matches...)
	}
	for _, dir := range dirs {
		for _, name := range hostProtectedNames {
			if match, _ := filepath.Match(strings.ToLower(filepath.Base(path)), name); match {
				if reason := s.protected(filepath.Join(dir, name)); reason != "" {
					return reason
				}
			}
		}
	}
	return ""
}
