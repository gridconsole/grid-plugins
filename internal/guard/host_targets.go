package guard

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

type hostShellKind int

const (
	hostWord hostShellKind = iota
	hostSeparator
	hostReadRedirect
	hostWriteRedirect
	hostDupRedirect // >&: a file unless the operand is an fd number or -
	hostRefuse      // the tokenizer cannot tell what the shell runs: refuse the command
	hostSkipped     // a # comment was skipped here: the shells may read its text as commands
)

type hostShellToken struct {
	text string
	kind hostShellKind
	// groups are the commands inside $(..), backticks, <(..) and >(..) in the
	// word, and any of those nested in ${..}. Nothing runs them; they are scanned.
	groups []string
	ansi   bool // the word holds $'..' or $"..", which the guard does not decode
	// live: the word holds a $, a backtick or a { the shell expands (none inside '..', no { inside "..").
	// dq: the word is one "..", nothing before or after it. calm: an argument that is not live, or that is
	// "$NAME" for a name the command never sets, in a command main's textual check reads as a read; set
	// by hostSegmentsOf.
	live, dq, calm bool
}

// hostMaxGroupDepth bounds nested substitutions; deeper text fails closed.
const hostMaxGroupDepth = 16

// hostKeywordAt reports whether word stands at text[i] as a whole shell word that
// begins at or after start: a command separator, blank or bracket before it and
// anything but a word character behind it.
func hostKeywordAt(text string, i, start int, word string) bool {
	if !strings.HasPrefix(text[i:], word) {
		return false
	}
	if i > start && !strings.ContainsRune(" \t\r\n;&|(){}", rune(text[i-1])) {
		return false
	}
	end := i + len(word)
	return end == len(text) || !hostWordByte(text[end])
}

func hostWordByte(b byte) bool {
	return b == '_' || b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// hostWordEnds are the bytes after which a new shell word begins: a # there starts
// a comment, anywhere else a # is an ordinary character. A carriage return is a
// word byte to the shells.
const hostWordEnds = " \t\n;&|()<>"

// hostHashRefused are the bytes a # may not follow where it would start a comment:
// bash extglob @(#|x), a [[ =~ (#|x) ]] regex and a zsh glob group read it as a
// pattern byte there, so the command is refused instead.
const hostHashRefused = "()<>"

// hostJoinLines drops every backslash-newline pair, as the shells do before they
// read words. A backslash escapes the byte after it, so \\<newline> stays. A comment
// ends at its newline even after a backslash, so a pair inside a comment is not a
// join: the second result reports one and the caller refuses the command. With no
// substitution, backtick, heredoc or $'..' in the text the quotes are tracked and
// only a # outside quotes starts a comment, and a backslash in '..' is literal;
// otherwise every # at the start of a word counts and every pair is joined. A pair
// after a << is refused: a quoted heredoc body keeps it, an unquoted one joins it.
func hostJoinLines(text string) (string, bool) {
	if !strings.Contains(text, "\\\n") {
		return text, false
	}
	exact := !strings.Contains(text, "`")
	for _, marker := range []string{"$(", "${", "$'", "<(", ">(", "<<"} {
		exact = exact && !strings.Contains(text, marker)
	}
	var b strings.Builder
	var quote byte
	comment, refuse := false, false
	prev := byte(' ')
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '\\' && i+1 < len(text) && quote != '\'' {
			i++
			if text[i] == '\n' {
				refuse = refuse || comment || strings.Contains(b.String(), "<<")
				continue
			}
			b.WriteByte('\\')
			ch, prev = text[i], 'x'
		} else {
			switch {
			case comment:
				comment = ch != '\n'
			case exact && quote != 0:
				if ch == quote {
					quote = 0
				}
			case exact && (ch == '\'' || ch == '"'):
				quote = ch
			case ch == '#' && strings.IndexByte(hostWordEnds, prev) >= 0:
				comment = true
				refuse = refuse || strings.IndexByte(hostHashRefused, prev) >= 0
			}
			prev = ch
		}
		b.WriteByte(ch)
	}
	return b.String(), refuse
}

// hostShellGroup finds the end of a quote, substitution or brace group that
// starts at start, as bash reads it. An unfinished group consumes the remaining text.
func hostShellGroup(text string, start int, close byte) (int, bool) {
	return hostShellGroupAs(text, start, close, false, false)
}

// hostShellGroupAs finds a group's end as bash reads it, or as zsh does. In a ( )
// group the ) of a case pattern does not end it: once a word named case is seen no
// ) closes the group, so it takes the rest of the text and that text is scanned as
// commands. Nothing ever lowers the count: esac in an argument, a comment or a
// heredoc would otherwise close the group early. A mistake only keeps a group open.
// Quotes follow the shells: in '..' nothing but ' is special, not even a backslash;
// in ".." only \, ", a backtick, $( and ${ are; $'..' escapes with a backslash; a ( )
// group skips a comment, which begins at a # that starts a word and ends at the
// newline, and a heredoc body, up to its delimiter line. The readings differ in two
// places: bash ends a backtick group at the first unescaped backtick while zsh reads
// quotes in it, and in a ${..} inside ".." (quoted) zsh reads ' as a plain byte.
func hostShellGroupAs(text string, start int, close byte, zsh, quoted bool) (int, bool) {
	cases := 0
	atWord := true // the next byte starts a word
	var docs []hostHeredoc
	for i := start; i < len(text); i++ {
		ch, wordStart := text[i], atWord
		atWord = strings.IndexByte(hostWordEnds, ch) >= 0
		if close == '\'' {
			if ch == '\'' {
				return i + 1, true
			}
			continue
		}
		if close == '`' && !zsh {
			if ch == '\\' {
				i++
			} else if ch == '`' {
				return i + 1, true
			}
			continue
		}
		if ch == '$' && i+1 < len(text) && text[i+1] == '$' { // $$, the PID, is one unit
			i++
			continue
		}
		if close == '}' && quoted && zsh && ch == '\'' {
			continue
		}
		if close == '"' && ch != '\\' && ch != '"' && ch != '`' && !(ch == '$' && i+1 < len(text) && (text[i+1] == '(' || text[i+1] == '{')) {
			continue
		}
		if close == '"' && ch == '$' {
			i++
			ch = text[i]
		}
		switch {
		case ch == '\\':
			i++
		case close == ')' && hostKeywordAt(text, i, start, "case"):
			cases++
			i += len("case") - 1
		case ch == close && cases == 0:
			return i + 1, true
		case close == ')' && ch == '#' && wordStart && (i == 0 || strings.IndexByte(hostHashRefused, text[i-1]) < 0):
			for i+1 < len(text) && text[i+1] != '\n' {
				i++
			}
		case (close == ')' || close == '}') && ch == '<' && strings.HasPrefix(text[i:], "<<") && !strings.HasPrefix(text[i:], "<<<"):
			if doc, end, ok := hostHeredocAt(text, i); ok {
				docs = append(docs, doc)
				i, atWord = end-1, false
			} else {
				i++
			}
		case ch == '\n' && len(docs) > 0:
			next, _, ok := hostHeredocBodies(text, i+1, docs, zsh)
			if !ok {
				return len(text), false
			}
			docs = nil
			i, atWord = next-1, true
		case ch == '$' && i+1 < len(text) && text[i+1] == '\'':
			// $'..' is the one single-quoted form where a backslash escapes.
			i += 2
			for i < len(text) && text[i] != '\'' {
				if text[i] == '\\' {
					i++
				}
				i++
			}
		case ch == '\'' || ch == '"' || ch == '`':
			end, _ := hostShellGroupAs(text, i+1, ch, zsh, false)
			i, atWord = end-1, false
		case ch == '(':
			end, _ := hostShellGroupAs(text, i+1, ')', zsh, false)
			// A subshell ends a word; $( ) and <( ) are words, so a # after them is not a comment.
			i, atWord = end-1, i == start || strings.IndexByte("$<>", text[i-1]) < 0
		case ch == '{':
			end, _ := hostShellGroupAs(text, i+1, '}', zsh, close == '"' || close == '}' && quoted)
			i, atWord = end-1, false
		}
	}
	return len(text), false
}

// hostHeredoc is a pending heredoc: its delimiter, whether the delimiter word was
// quoted (the body is then plain text) and whether <<- strips leading tabs.
type hostHeredoc struct {
	delim         string
	quoted, strip bool
}

// hostHeredocAt reads the << or <<- operator at text[i] and its delimiter word, and
// returns where the word ends. ok is false for a word the shells could read
// differently: empty, unfinished, or holding an expansion.
func hostHeredocAt(text string, i int) (hostHeredoc, int, bool) {
	var doc hostHeredoc
	j := i + 2
	if j < len(text) && text[j] == '-' {
		doc.strip = true
		j++
	}
	for j < len(text) && (text[j] == ' ' || text[j] == '\t') {
		j++
	}
	var b strings.Builder
	for ; j < len(text) && strings.IndexByte(" \t\n;&|()<>", text[j]) < 0; j++ {
		switch ch := text[j]; ch {
		case '$', '`':
			return doc, j, false
		case '\\':
			doc.quoted = true
			if j++; j == len(text) || text[j] == '\n' {
				return doc, j, false
			}
			b.WriteByte(text[j])
		case '\'', '"':
			doc.quoted = true
			k := strings.IndexByte(text[j+1:], ch)
			if k < 0 || ch == '"' && strings.ContainsAny(text[j+1:j+1+k], "\\$`") {
				return doc, len(text), false
			}
			b.WriteString(text[j+1 : j+1+k])
			j += k + 1
		default:
			b.WriteByte(ch)
		}
	}
	doc.delim = b.String()
	return doc, j, doc.delim != ""
}

// hostHeredocBodies skips the bodies of docs, which begin at text[start], and returns
// where the commands go on and the substitutions the unquoted bodies run. ok is false
// when a delimiter line never comes or a substitution in a body is unfinished.
func hostHeredocBodies(text string, start int, docs []hostHeredoc, zsh bool) (int, []string, bool) {
	var groups []string
	for _, doc := range docs {
		for body := start; ; {
			if start >= len(text) {
				return len(text), groups, false
			}
			end, next := len(text), len(text)
			if at := strings.IndexByte(text[start:], '\n'); at >= 0 {
				end, next = start+at, start+at+1
			}
			line := text[start:end]
			if doc.strip {
				line = strings.TrimLeft(line, "\t")
			}
			if line != doc.delim {
				start = next
				continue
			}
			if !doc.quoted {
				found, ok := hostBodyGroups(text[body:start], zsh)
				if !ok {
					return len(text), groups, false
				}
				groups = append(groups, found...)
			}
			start = next
			break
		}
	}
	return start, groups, true
}

// hostBodyGroups returns the substitutions an unquoted heredoc body runs; ok is false
// when one is unfinished. In the body only \, $( ), ${ } and backticks are special.
func hostBodyGroups(body string, zsh bool) ([]string, bool) {
	var groups []string
	for i := 0; i < len(body); i++ {
		ch := body[i]
		switch {
		case ch == '\\' || ch == '$' && i+1 < len(body) && body[i+1] == '$':
			i++
		case ch == '`' || ch == '$' && i+1 < len(body) && (body[i+1] == '(' || body[i+1] == '{'):
			start, close := i+1, byte('`')
			if ch == '$' {
				start, close = i+2, ')'
				if body[i+1] == '{' {
					close = '}'
				}
			}
			end, closed := hostShellGroupAs(body, start, close, zsh, close == '}')
			if !closed {
				return groups, false
			}
			inner := hostGroupBody(body, start, end, closed, close)
			if close != '}' {
				groups = append(groups, inner)
			} else if found, ok := hostBraceGroups(inner, zsh); ok {
				groups = append(groups, found...)
			} else {
				return groups, false
			}
			i = end - 1
		}
	}
	return groups, true
}

// hostBraceGroups returns the substitutions nested in a ${..} body; ok is false when
// the tokenizer refuses the body.
func hostBraceGroups(body string, zsh bool) ([]string, bool) {
	var groups []string
	for _, tok := range hostShellTokensAs(body, zsh) {
		if tok.kind == hostRefuse {
			return groups, false
		}
		groups = append(groups, tok.groups...)
	}
	return groups, true
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

// hostHasComment reports whether a # starts a word anywhere in text, even in quotes.
func hostHasComment(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] == '#' && (i == 0 || strings.IndexByte(hostWordEnds, text[i-1]) >= 0) {
			return true
		}
	}
	return false
}

func hostDigits(text string) bool {
	return text != "" && strings.Trim(text, "0123456789") == ""
}

// hostShellTokens removes quoting but preserves glob and expansion markers.
// This is deliberately conservative even for quoted markers, not a shell parser.
// Redirect operators come out whole: >, >>, >|, >!, >>|, >>!, >&, >&|, >&!, >>&,
// &>, &>>, &>|, &>!, <>, <, <<, <<<, <&, each with an optional leading fd number,
// which is dropped. Only >& followed by a word is a dup (a file unless an fd).
// Backslash-newline pairs go first, before any word is matched. A # that starts a
// word, outside quotes, begins a comment that ends at the newline; nothing in it is
// special, not a quote nor a substitution. A heredoc body is skipped up to its delimiter line; an
// unquoted body's substitutions are scanned. A hostRefuse token ends the list when
// the tokenizer cannot tell what the shell runs: a carriage return outside quotes,
// $$ before a quote, ( or {, a heredoc without its delimiter line, or in the zsh
// reading a # that starts a word in a backtick group (that reading does not skip comments).
func hostShellTokens(command string) []hostShellToken {
	return hostShellTokensAs(command, false)
}

// hostShellTokensAs reads the command as bash does, or as zsh does (see hostShellGroupAs).
func hostShellTokensAs(command string, zsh bool) []hostShellToken {
	command, refuse := hostJoinLines(command)
	var out []hostShellToken
	var word strings.Builder
	var groups []string
	var docs []hostHeredoc
	started, comment, ansi := false, false, false // a word has begun, even an empty one; inside a comment; $'..' or $".." in the word
	live, dqStart, dqEnd := false, false, -1      // an expansion in the word; the word began with "; where that " closed
	flush := func() {
		started = false
		if word.Len() > 0 || len(groups) > 0 { // groups alone: substitutions in a heredoc body
			out = append(out, hostShellToken{text: word.String(), groups: groups, ansi: ansi, live: live, dq: dqStart && dqEnd == word.Len()})
		}
		word.Reset()
		groups, ansi, live, dqStart, dqEnd = nil, false, false, false, -1
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
		refuse = refuse || ch == '\r' && quote == 0
		if ch == '$' && at(i+1) == '$' && quote != '\'' && quote != '$' && !comment {
			// $$ is one unit; the shells disagree on a quote, ( or { right after it.
			next := at(i + 2)
			refuse = refuse || next == '(' || next == '{' || next == '\'' && quote == 0
			word.WriteString("$$")
			started, live = true, true
			i++
			continue
		}
		// In '..' and $'..' only the closing ' is special, and in a comment nothing is: no $(,
		// ${ or backtick opens a group there.
		subst := !comment && quote != '\'' && quote != '$' && (ch == '`' || (ch == '$' && (at(i+1) == '(' || at(i+1) == '{'))) || ((ch == '<' || ch == '>') && quote == 0 && !comment && at(i+1) == '(')
		if subst {
			start, close := i+1, byte('`')
			if ch != '`' {
				start, close = i+2, ')'
				if ch == '$' && command[i+1] == '{' {
					close = '}'
				}
			}
			end, closed := hostShellGroupAs(command, start, close, zsh, close == '}' && quote == '"')
			body := hostGroupBody(command, start, end, closed, close)
			if close == '}' {
				found, ok := hostBraceGroups(body, zsh)
				groups = append(groups, found...)
				refuse = refuse || !ok
			} else {
				groups = append(groups, body)
				refuse = refuse || zsh && close == '`' && hostHasComment(body)
			}
			word.WriteString(command[i:end])
			started = true
			live = live || ch != '<' && ch != '>'
			i = end - 1
			continue
		}
		if comment {
			if ch != '\n' {
				continue
			}
			comment = false
		}
		if ch == '\\' && quote != '\'' && i+1 < len(command) {
			i++
			if command[i] != '\n' {
				word.WriteByte(command[i])
			}
			started = true
			continue
		}
		if quote != 0 {
			if quote == '$' && ch == '\'' || quote != '$' && ch == quote { // quote '$' is $'..'
				if quote == '"' && dqStart && dqEnd < 0 {
					dqEnd = word.Len()
				}
				quote = 0
			} else {
				word.WriteByte(ch)
				live = live || ch == '$' && quote == '"'
			}
			continue
		}
		switch ch {
		case '\'', '"':
			dqStart = dqStart || ch == '"' && !started
			quote, started = ch, true
		case '$':
			word.WriteByte(ch)
			started, live = true, true
			ansi = ansi || at(i+1) == '\'' || at(i+1) == '"'
			if at(i+1) == '\'' { // $'..': a backslash escapes in it
				quote = '$'
				i++
			}
		case '#':
			if started {
				word.WriteByte(ch)
			} else {
				comment = true
				out = append(out, hostShellToken{kind: hostSkipped})
				refuse = refuse || i > 0 && strings.IndexByte(hostHashRefused, command[i-1]) >= 0
			}
		case ' ', '\t':
			flush()
		case '\n', ';', '(', ')':
			emit(hostSeparator, string(ch))
			if ch == '\n' && len(docs) > 0 {
				next, found, ok := hostHeredocBodies(command, i+1, docs, zsh)
				groups, docs, refuse = found, nil, refuse || !ok
				i = next - 1
			}
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
				if clobber := at(i + 1); clobber == '|' || clobber == '!' {
					text += string(clobber)
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
				if third := at(i + 2); third != '<' {
					doc, _, ok := hostHeredocAt(command, i)
					docs = append(docs, doc)
					refuse = refuse || !ok
				}
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
				if next == '>' {
					text += ">"
					i++
				}
				switch after := at(i + 1); after {
				case '|', '!': // clobber forms of bash and zsh
					text += string(after)
					i++
				case '&':
					text += "&"
					i++
					if clobber := at(i + 1); clobber == '|' || clobber == '!' {
						text += string(clobber)
						i++
					} else if text == ">&" {
						kind = hostDupRedirect
					}
				}
			}
			emit(kind, text)
		default:
			word.WriteByte(ch)
			started = true
			live = live || ch == '{'
		}
	}
	flush()
	if refuse {
		out = append(out, hostShellToken{kind: hostRefuse})
	}
	return out
}

// hostSegment is one simple command: its words, redirect operands and the
// separator that ended it.
type hostSegment struct {
	assigns, args, writes, reads []hostShellToken
	comments                     []hostShellToken // substitutions in a heredoc body: scanned, never words
	separator                    string
	refuse                       bool // the tokenizer could not tell what the shell runs
	skipped                      bool // a # comment was skipped in the command
	restart                      bool // the zsh reading of the command starts here
}

var hostFdOperand = regexp.MustCompile(`^(?:[0-9]+-?|-)$`)

// hostSegments returns the commands as bash reads the text and, where zsh reads it
// differently, the zsh reading after them, so a write either shell runs is scanned.
func hostSegments(command string, whole hostWhole) []hostSegment {
	out := hostSegmentsOf(hostShellTokensAs(command, false), whole)
	if zsh := hostSegmentsOf(hostShellTokensAs(command, true), whole); !slices.EqualFunc(out, zsh, hostSameSegment) {
		zsh[0].restart = true
		out = append(out, zsh...)
	}
	return out
}

func hostSameSegment(a, b hostSegment) bool {
	same := func(x, y hostShellToken) bool {
		return x.text == y.text && x.kind == y.kind && x.ansi == y.ansi && x.live == y.live && x.dq == y.dq && slices.Equal(x.groups, y.groups)
	}
	return a.separator == b.separator && a.refuse == b.refuse && a.skipped == b.skipped && slices.EqualFunc(a.tokens(), b.tokens(), same) &&
		slices.EqualFunc(a.writes, b.writes, same) && slices.EqualFunc(a.args, b.args, same)
}

// hostWhole is the entire command the tool gave, of which a substitution body is only a part.
// relaxed: main's own textual check, onlyReads on the flattened text, reads all of it as reads.
// Only then may the guard trust more than the strict reading: a comment, a numeric
// subscript, an expansion in '..' or a "$NAME" for a name the command never sets. So a line
// that the lane allows and main denied stays denied.
type hostWhole struct {
	text    string
	relaxed bool
}

func hostWholeOf(text string) hostWhole {
	return hostWhole{text, !hostHoldsANSI(text, 0) && hostMainReads(text)}
}

// hostHoldsANSI reports a $'..' or $".." anywhere in command, substitutions included. The shells decode it
// into text flatten never sees, so main's onlyReads is no evidence for a command that holds one. A
// $ before the closing ' of '-x$' is not one.
func hostHoldsANSI(command string, depth int) bool {
	if depth > hostMaxGroupDepth {
		return true
	}
	for _, seg := range hostSegments(command, hostWhole{text: command}) {
		for _, tok := range seg.tokens() {
			if tok.ansi {
				return true
			}
			for _, body := range tok.groups {
				if hostHoldsANSI(body, depth+1) {
					return true
				}
			}
		}
	}
	return false
}

// hostMainReads is main's check: onlyReads holds for both readings of the flattened command.
func hostMainReads(command string) bool {
	for _, flat := range flattened(command) {
		if !onlyReads(command, flat) {
			return false
		}
	}
	return true
}

func hostSegmentsOf(tokens []hostShellToken, whole hostWhole) []hostSegment {
	var out []hostSegment
	var seg hostSegment
	for i := 0; i < len(tokens); i++ {
		switch tok := tokens[i]; tok.kind {
		case hostSeparator:
			seg.separator = tok.text
			out = append(out, seg)
			seg = hostSegment{}
		case hostRefuse:
			seg.refuse = true
		case hostSkipped:
			seg.skipped = true
		case hostWord:
			if tok.text == "" {
				seg.comments = append(seg.comments, tok)
			} else if len(seg.args) == 0 && strings.Contains(tok.text, "=") {
				seg.assigns = append(seg.assigns, tok)
			} else {
				tok.calm = whole.relaxed && (!tok.live || tok.dq && hostOwnVar(tok.text) && hostPlainVar(tok.text, whole.text))
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
	all = append(all, g.reads...)
	return append(all, g.comments...)
}

// readCommand reports a command that only reads, whatever its redirects do.
func (g hostSegment) readCommand() bool {
	if len(g.args) == 0 {
		return true
	}
	name := strings.ToLower(filepath.Base(g.args[0].text))
	for _, arg := range g.args[1:] {
		if readerRuns(name, arg.text) || hostOpaqueOption(name, arg) {
			return false
		}
	}
	return readOnly[name]
}

// hostOpaqueOption reports a find or rg argument that may expand to an option the
// guard cannot read: $'..' and $"..", or a word that starts with - or an expansion
// and holds one ($x, $(..), a backtick, {a,b}). It counts as a runner, never a read,
// unless it is calm: no expansion runs in it ('-x$'), or it is a lone "$PWD" the command
// never sets. Any other "$NAME" stays a runner: the environment or a startup file may
// hold an option there (PAT=--pre=tee) that the command does not set.
func hostOpaqueOption(name string, arg hostShellToken) bool {
	if name != "find" && name != "rg" {
		return false
	}
	const expansion = "$`{"
	return arg.ansi || arg.text != "" && !arg.calm && strings.ContainsAny(arg.text, expansion) &&
		(arg.text[0] == '-' || strings.IndexByte(expansion, arg.text[0]) >= 0)
}

var hostPlainName = regexp.MustCompile(`^\$(?:\{([A-Za-z_][A-Za-z0-9_]*)\}|([A-Za-z_][A-Za-z0-9_]*))$`)
var hostPrintf = regexp.MustCompile(`(?:^|[^A-Za-z0-9_])printf(?:[^A-Za-z0-9_]|$)`)

// hostOwnVar reports the word $PWD or ${PWD}: a variable the shell sets and keeps itself, which the
// environment cannot hand it a different value for. Any other name may arrive from the environment
// or a startup file with an option in it, and the line does not set it, so it is never calm.
func hostOwnVar(word string) bool {
	m := hostPlainName.FindStringSubmatch(word)
	return m != nil && m[1]+m[2] == "PWD"
}

// hostLower folds ASCII upper case and nothing else, so an index in the result is an index in s.
func hostLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// hostPlainVar reports the word $NAME or ${NAME} for a variable that whole only ever
// reads: each use of the name is $NAME or ${NAME}, and no printf runs, since printf -v
// sets a variable by its name. $_ is the last argument of the command before it. Names
// compare without case: zsh ties cdpath to CDPATH, fignore to FIGNORE, psvar to PSVAR. A
// ${! or ${( in whole (bash ${!n:=w}, zsh ${(P)n::=w}) assigns to the variable whose name
// another variable holds, so no name is known to be only read.
func hostPlainVar(word, whole string) bool {
	m := hostPlainName.FindStringSubmatch(word)
	if m == nil || hostPrintf.MatchString(whole) || strings.Contains(whole, "${!") || strings.Contains(whole, "${(") {
		return false
	}
	name := m[1] + m[2]
	if name == "_" {
		return false
	}
	folded, lname := hostLower(whole), hostLower(name)
	for from := 0; ; {
		i := strings.Index(folded[from:], lname)
		if i < 0 {
			return true
		}
		i, from = from+i, from+i+len(name)
		if i > 0 && hostWordByte(whole[i-1]) || from < len(whole) && hostWordByte(whole[from]) {
			continue // a longer name
		}
		if !(i > 0 && whole[i-1] == '$') && !(i > 1 && whole[i-2:i] == "${" && from < len(whole) && whole[from] == '}') {
			return false
		}
	}
}

// hostNumericSubscript matches the name of an assignment to an array element by number: x[0]=,
// x[0]+=. A number names no command, so BASH_CMDS[0]=, functions[0]= and aliases[0]= shadow nothing;
// PATH[0]= and zsh path[1]= change where a command is found, and IFS[0]= how words split.
var hostNumericSubscript = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*)\[[0-9]+\]\+?$`)

// subscripted reports an assignment word whose name has a subscript: a[..]=, and
// BASH_CMDS[..]= or zsh functions[..]= and commands[..]=, which shadow a command. The
// shells read the text inside the brackets as an expression, not as words. A numeric
// subscript is let through when relaxed.
func (g hostSegment) subscripted(relaxed bool) bool {
	for _, assign := range g.assigns {
		name, _, _ := strings.Cut(assign.text, "=")
		if !strings.Contains(name, "[") {
			continue
		}
		if m := hostNumericSubscript.FindStringSubmatch(name); !relaxed || m == nil || strings.EqualFold(m[1], "path") || strings.EqualFold(m[1], "ifs") {
			return true
		}
	}
	return false
}

// hostInSubscript reports a [ left open on the line of text[i]: a << there is a
// shift in an array subscript to bash and zsh, so the lines after it run.
func hostInSubscript(text string, i int) bool {
	line := text[strings.LastIndexByte(text[:i], '\n')+1 : i]
	return strings.Count(line, "[") > strings.Count(line, "]")
}

// hostIntegerVars hold a number in bash 5 or zsh 5: assigning a word to one evaluates it as arithmetic,
// and a word that names a variable holding a[$(cmd)] runs cmd (a='x[$(cmd)]'; RANDOM=a). Only a number may be assigned.
var hostIntegerVars = map[string]bool{
	"BASHPID": true, "EUID": true, "HISTCMD": true, "OPTIND": true, "PPID": true, "RANDOM": true, "SRANDOM": true, "UID": true,
	"ARGC": true, "COLUMNS": true, "EGID": true, "FUNCNEST": true, "GID": true, "HISTSIZE": true, "KEYTIMEOUT": true,
	"LINENO": true, "LINES": true, "LISTMAX": true, "MAILCHECK": true, "SAVEHIST": true, "SECONDS": true, "SHLVL": true,
	"TRY_BLOCK_ERROR": true, "TRY_BLOCK_INTERRUPT": true, "TTYIDLE": true, "ZSH_SUBSHELL": true, "status": true,
}

var hostNumber = regexp.MustCompile(`^[0-9]+$`)

// integerAssigned reports an assignment of anything but a number to an integer variable, x= or x[..]= or x+=.
func (g hostSegment) integerAssigned() bool {
	for _, assign := range g.assigns {
		name, value, _ := strings.Cut(assign.text, "=")
		name, _, _ = strings.Cut(strings.TrimSuffix(name, "+"), "[")
		if hostIntegerVars[name] && !hostNumber.MatchString(value) {
			return true
		}
	}
	return false
}

func (g hostSegment) readsOnly() bool { return len(g.writes) == 0 && g.readCommand() }

// hostProvenReads reports a command the tokenizer shows to only read, in both shell
// readings and in every substitution. A heredoc skip is trusted only for a quoted
// delimiter, and never with $[ or ((, or after a [ left open on its line, where the
// shells read << as a shift and run the lines the tokenizer took for a body. A
// comment skipped, or an assignment to a subscript, is never proven (the shells can read
// the text after # as commands, and a[..]=, BASH_CMDS[..]= shadow a command) unless
// main's own textual check, onlyReads on the flattened text, reads the whole command as
// reads too: flatten sees every word, comments and subscripts included.
func hostProvenReads(command string) bool {
	if hostReadsIn(command, hostWhole{text: command}, 0) {
		return true
	}
	loose := hostWholeOf(command)
	return loose.relaxed && hostReadsIn(command, loose, 0)
}

// hostReadsIn proves command, a part of whole.
func hostReadsIn(command string, whole hostWhole, depth int) bool {
	if depth > hostMaxGroupDepth || strings.Contains(command, "$[") || strings.Contains(command, "((") {
		return false
	}
	for i := 0; i+1 < len(command); i++ {
		switch {
		case command[i] != '<' || command[i+1] != '<':
		case strings.HasPrefix(command[i:], "<<<"):
			i += 2
		default:
			if doc, _, ok := hostHeredocAt(command, i); !ok || !doc.quoted || hostInSubscript(command, i) {
				return false
			}
			i++
		}
	}
	for _, seg := range hostSegments(command, whole) {
		if seg.refuse || seg.skipped && !whole.relaxed || seg.subscripted(whole.relaxed) || seg.integerAssigned() || !seg.readsOnly() {
			return false
		}
		for _, tok := range seg.tokens() {
			for _, body := range tok.groups {
				if !hostReadsIn(body, whole, depth+1) {
					return false
				}
			}
		}
	}
	return true
}

// hostProtectedNames are the entries a write must not reach: matched as a
// basename pattern when the target is a glob or holds an expansion.
var hostProtectedNames = []string{"workflows.md", "workflow.md", "task.md", "history.md", "settings.json", ".settings.lock", "approval.json", "approval-ship.json", ".task-write.json", ".task.lock", "questions", "plan-revisions"}

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
	whole hostWhole // the command as the tool gave it; run scans parts of it
	roots []string  // loaded on first use: resolving them reads the file system
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
	return (&writeScan{c: c, whole: hostWholeOf(command)}).run(command, c.in.Cwd, false, 0)
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
		if historyAt(root, path) {
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
	start, startUncertain := base, uncertain
	for _, seg := range hostSegments(command, s.whole) {
		if seg.refuse {
			return gateReason
		}
		if seg.restart {
			base, uncertain = start, startUncertain
		}
		for _, tok := range seg.tokens() {
			for _, body := range tok.groups {
				if reason := s.run(body, base, uncertain, depth+1); reason != "" {
					return reason
				}
			}
		}
		for _, target := range seg.writes {
			if target.ansi {
				return gateReason
			}
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
				if arg.ansi { // $'..' and $".." may hold any path: the guard does not decode them
					return gateReason
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
