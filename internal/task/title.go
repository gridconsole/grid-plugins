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
	if doc := taskfile.ParseFrontmatter(strings.TrimLeft(words, " \t\n")); doc != nil {
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

func cutTitle(title string) string {
	if chars := []rune(title); len(chars) > 120 {
		return string(chars[:120])
	}
	return title
}
