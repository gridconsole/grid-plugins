package guard

import (
	"path/filepath"
	"strings"
)

type hostShellToken struct {
	text string
	op   bool
}

// hostShellGroup keeps substitutions in their containing word, including their
// whitespace and operators. Nothing is evaluated; an unfinished group consumes
// the remaining text and still leaves its expansion marker in the target.
func hostShellGroup(text string, start int, close byte) int {
	for i := start; i < len(text); i++ {
		switch {
		case text[i] == '\\':
			i++
		case text[i] == close:
			return i + 1
		case close == '\'':
		case text[i] == '\'' || text[i] == '"' || text[i] == '`':
			i = hostShellGroup(text, i+1, text[i]) - 1
		case text[i] == '(':
			i = hostShellGroup(text, i+1, ')') - 1
		case text[i] == '{':
			i = hostShellGroup(text, i+1, '}') - 1
		}
	}
	return len(text)
}

// hostShellTokens removes quoting but preserves glob and expansion markers.
// This is deliberately conservative even for quoted markers, not a shell parser.
func hostShellTokens(command string) []hostShellToken {
	var out []hostShellToken
	var word strings.Builder
	flush := func() {
		if word.Len() > 0 {
			out = append(out, hostShellToken{text: word.String()})
			word.Reset()
		}
	}
	var quote byte
	for i := 0; i < len(command); i++ {
		ch := command[i]
		if ch == '`' || (ch == '$' && i+1 < len(command) && strings.ContainsRune("({", rune(command[i+1]))) {
			start, close := i+1, byte('`')
			if ch == '$' {
				start = i + 2
				close = ')'
				if command[i+1] == '{' {
					close = '}'
				}
			}
			end := hostShellGroup(command, start, close)
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
		case ';', '&', '|', '(', ')', '\n', '<', '>':
			flush()
			op := string(ch)
			if i+1 < len(command) && command[i+1] == ch && strings.ContainsRune("&|<>", rune(ch)) {
				i++
				op += string(ch)
			}
			out = append(out, hostShellToken{text: op, op: true})
		default:
			word.WriteByte(ch)
		}
	}
	flush()
	return out
}

// protectedWriteReason checks redirect targets and possible file operands of
// commands not known to be read-only. A cd updates only subsequent commands;
// reads and echo/printf data are never treated as write targets.
func (c context) protectedWriteReason(command string) string {
	roots := c.gridRoots()
	underGrid := func(path string) bool {
		path = strings.ToLower(realNative(path, 0))
		for _, root := range roots {
			if inside(root, path) {
				return true
			}
		}
		return false
	}
	protected := func(path string) string {
		path = strings.ToLower(realNative(path, 0))
		for _, root := range roots {
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
	base, uncertain := c.in.Cwd, false
	check := func(word string) string {
		if key, value, ok := strings.Cut(word, "="); ok && key != "" {
			word = value
		}
		if word == "" {
			return ""
		}
		word = c.expand(word)
		if uncertain && !filepath.IsAbs(word) {
			return gateReason
		}
		if at := strings.IndexAny(word, "$`"); at >= 0 {
			// Only the literal prefix is trustworthy. Do not clean away an
			// expansion/.. component before deciding containment.
			if (!filepath.IsAbs(word) && underGrid(base)) || underGrid(resolve(base, word[:at])) {
				return gateReason
			}
			return ""
		}
		path := resolve(base, word)
		if !strings.ContainsAny(path, "*?[") {
			return protected(path)
		}
		matches, _ := filepath.Glob(path)
		for _, match := range matches {
			if reason := protected(match); reason != "" {
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
			for _, name := range []string{"workflows.md", "workflow.md", "task.md", "settings.json", ".settings.lock", "approval.json", "approval-ship.json", ".task-write.json", ".task.lock", "questions", "plan-revisions"} {
				if match, _ := filepath.Match(strings.ToLower(filepath.Base(path)), name); match {
					if reason := protected(filepath.Join(dir, name)); reason != "" {
						return reason
					}
				}
			}
		}
		return ""
	}
	tokens := hostShellTokens(command)
	for len(tokens) > 0 {
		end := 0
		for end < len(tokens) && !(tokens[end].op && strings.ContainsAny(tokens[end].text, ";&|()\n")) {
			end++
		}
		segment := tokens[:end]
		separator := ""
		if end < len(tokens) {
			separator = tokens[end].text
			end++
		}
		tokens = tokens[end:]
		var args []string
		for i := 0; i < len(segment); i++ {
			if segment[i].op {
				if i+1 < len(segment) {
					if strings.HasPrefix(segment[i].text, ">") {
						if reason := check(segment[i+1].text); reason != "" {
							return reason
						}
					}
					i++ // Input redirects are reads, not command operands.
				}
				continue
			}
			args = append(args, segment[i].text)
		}
		for len(args) > 0 && strings.Contains(args[0], "=") {
			args = args[1:]
		}
		if len(args) == 0 {
			continue
		}
		name := strings.ToLower(filepath.Base(args[0]))
		read := readOnly[name]
		if name == "find" {
			for _, arg := range args[1:] {
				if findWrites[arg] {
					read = false
				}
			}
		}
		if !read {
			for _, arg := range args[1:] {
				if strings.HasPrefix(arg, "-") && !strings.Contains(arg, "=") {
					continue
				}
				if reason := check(arg); reason != "" {
					return reason
				}
			}
		}
		if name == "cd" && len(args) >= 2 && (separator == "&&" || separator == ";" || separator == "\n") {
			dir := args[1]
			if dir == "--" && len(args) >= 3 {
				dir = args[2]
			}
			dir = c.expand(dir)
			if at := strings.IndexAny(dir, "$`"); at >= 0 {
				uncertain = uncertain || at == 0 || underGrid(base) || underGrid(resolve(base, dir[:at]))
			} else {
				base = realNative(resolve(base, dir), 0)
				uncertain = uncertain && !filepath.IsAbs(dir)
			}
		}
	}
	return ""
}
