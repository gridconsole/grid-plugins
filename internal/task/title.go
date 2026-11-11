package task

import (
	"errors"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// titleOf derives a new task's title from the person's words; the words
// themselves are never changed. After leading spaces, tabs and newlines, a
// front matter block (the New task page always sends one) gives its nonblank
// string title, with whitespace runs folded to one space; a block without one
// gives the first nonblank line of its body. Words with no block give their
// first nonblank line, as before. A block with no title and a blank body is an
// error. The title is cut at 120 runes.
func titleOf(words string) (string, error) {
	text := words
	trimmed := strings.TrimLeft(words, " \t\n")
	if body, ok := emptyBlockBody(trimmed); ok {
		// `---` straight followed by `---` is an empty block; the shared parser
		// needs a line between the fences, so it does not see one.
		if strings.TrimSpace(body) == "" {
			return "", errors.New("task words hold only front matter and no title")
		}
		text = body
	} else if doc := taskfile.ParseFrontmatter(trimmed); doc != nil {
		raw, _ := doc.Data["title"].(string)
		if title := strings.Join(strings.Fields(raw), " "); title != "" {
			return cutTitle(title), nil
		}
		if strings.TrimSpace(doc.Body) == "" {
			return "", errors.New("task words hold only front matter and no title")
		}
		text = doc.Body
	}
	return cutTitle(strings.TrimSpace(strings.SplitN(strings.TrimSpace(text), "\n", 2)[0])), nil
}

// emptyBlockBody reports whether text opens with a `---` line followed at once
// by a closing `---` line, and gives what follows the closing line.
func emptyBlockBody(text string) (string, bool) {
	rest, ok := strings.CutPrefix(text, "---")
	if !ok {
		return "", false
	}
	rest, ok = cutLineEnd(rest)
	if !ok {
		return "", false
	}
	rest, ok = strings.CutPrefix(rest, "---")
	if !ok {
		return "", false
	}
	if rest == "" {
		return "", true
	}
	return cutLineEnd(rest)
}

// cutLineEnd removes a leading \n or \r\n.
func cutLineEnd(s string) (string, bool) {
	if rest, ok := strings.CutPrefix(s, "\r\n"); ok {
		return rest, true
	}
	return strings.CutPrefix(s, "\n")
}

func cutTitle(title string) string {
	if chars := []rune(title); len(chars) > 120 {
		return string(chars[:120])
	}
	return title
}
