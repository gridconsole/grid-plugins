package task

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// SummaryAllowed is the default role policy for writing task summaries. A host or
// the workflow may replace it during initialization, before concurrent writes
// begin.
var SummaryAllowed = defaultTaskEditAllowed

// summaryFile is the task folder file that holds the three reader summaries. The
// app reads it for the task page (contract/formats.md, "Task summary").
const summaryFile = "summary.md"

// summaryReaders are the reader levels in the order summary.md holds them.
var summaryReaders = []string{"owner", "developer", "customer"}

// SummaryOptions carries the reader level and the words for one summary write.
// Role is the host's role, independent of the author recorded in WriteOptions.Who.
type SummaryOptions struct {
	WriteOptions
	Reader, Text, Role string
}

// Summary replaces the body of one reader's section of summary.md and keeps every
// other byte of the file. It goes through the same lock, journal and receipt as
// Note, so it can neither race a state move nor be overwritten by journal replay.
// It changes no task state, and a closed task can still be summarised.
func (s Store) Summary(o SummaryOptions) (Event, error) {
	reader := strings.ToLower(strings.TrimSpace(o.Reader))
	known := false
	for _, name := range summaryReaders {
		known = known || name == reader
	}
	if !known {
		return Event{}, fmt.Errorf("%w: --for must be owner, developer or customer", ErrEditUsage)
	}
	text := strings.TrimSpace(o.Text)
	if text == "" {
		return Event{}, fmt.Errorf("%w: a summary needs words", ErrEditUsage)
	}
	if err := checkTaskText("text", text, true); err != nil {
		return Event{}, err
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			return Event{}, fmt.Errorf("%w: a line of the summary starts with \"## \", which would open a new section", ErrEditUsage)
		}
	}
	o.Action, o.Note = "summary", "summary · "+reader
	if err := o.WriteOptions.validateText(); err != nil {
		return Event{}, err
	}
	if !SummaryAllowed(strings.TrimSpace(o.Role)) {
		return Event{}, errors.New("scope: role cannot write task summaries")
	}
	var event Event
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if err := writableFile(t.Path, summaryFile); err != nil {
			return err
		}
		old, err := readOptional(filepath.Join(t.Path, summaryFile))
		if err != nil {
			return err
		}
		event = s.event(t, o.WriteOptions, t.State)
		return commitWrite(t.Path, event, map[string][]byte{summaryFile: setSummary(old, reader, text)})
	})
	return event, err
}

// freshSummary is the file as a task that was never summarised gets it: the three
// headings in order, one blank line between sections, every body empty.
func freshSummary() []byte {
	var out strings.Builder
	for i, name := range summaryReaders {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString("## " + strings.ToUpper(name[:1]) + name[1:] + "\n")
	}
	return []byte(out.String())
}

// summaryHeading returns the reader level a heading line opens, as the app reads
// it: a line starting with "## ", the rest lowercased and trimmed. It returns ""
// for any other line.
func summaryHeading(line string) (string, bool) {
	h, ok := strings.CutPrefix(strings.TrimRight(line, "\r\n"), "## ")
	return strings.ToLower(strings.TrimSpace(h)), ok
}

// setSummary returns data with the body of reader's section replaced by text. The
// body is everything between the heading line and the next heading line (or the
// end of the file); every byte outside it is kept, line ends included. When the
// app would see the same heading twice it keeps the last, so that one is replaced.
// A file without the heading gets the section at its end.
func setSummary(data []byte, reader, text string) []byte {
	if len(data) == 0 {
		data = freshSummary()
	}
	lines := strings.SplitAfter(string(data), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	eol := "\n"
	if strings.Contains(string(data), "\r\n") {
		eol = "\r\n"
	}
	at := -1
	for i, line := range lines {
		if name, ok := summaryHeading(line); ok && name == reader {
			at = i
		}
	}
	next := len(lines)
	for i := at + 1; at >= 0 && i < len(lines); i++ {
		if _, ok := summaryHeading(lines[i]); ok {
			next = i
			break
		}
	}
	title := "## " + strings.ToUpper(reader[:1]) + reader[1:]
	if at < 0 {
		// Append: end the last line, leave one blank line, then the new section.
		out := string(data)
		if !strings.HasSuffix(out, "\n") {
			out += eol
		}
		if !strings.HasSuffix(out, eol+eol) {
			out += eol
		}
		return []byte(out + title + eol + eol + strings.ReplaceAll(text, "\n", eol) + eol)
	}
	heading := lines[at]
	if own := heading[len(strings.TrimRight(heading, "\r\n")):]; own != "" {
		eol = own
	}
	var body string
	if next < len(lines) {
		// Body, then one blank line before the next heading.
		body = eol + strings.ReplaceAll(text, "\n", eol) + eol + eol
	} else {
		body = eol + strings.ReplaceAll(text, "\n", eol) + eol
	}
	if !strings.HasSuffix(heading, "\n") {
		heading += eol
	}
	return []byte(strings.Join(lines[:at], "") + heading + body + strings.Join(lines[next:], ""))
}
