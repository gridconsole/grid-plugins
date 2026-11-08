package task

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// validTaskText checks the original input before whitespace folding. Freeform
// words, outcomes and receipt notes may contain tab and newline; names and
// rename/decline input must fit on one line without either.
func validTaskText(text string, layout bool) bool {
	return utf8.ValidString(text) && !strings.ContainsFunc(text, func(r rune) bool {
		if layout && (r == '\t' || r == '\n') {
			return false
		}
		return unicode.IsControl(r) || r == '\u2028' || r == '\u2029' ||
			(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069') ||
			(r >= '\ufdd0' && r <= '\ufdef') || r&0xffff >= 0xfffe
	})
}

func checkTaskText(field, text string, layout bool) error {
	if !validTaskText(text, layout) {
		return fmt.Errorf("%w: %s contains invalid UTF-8 or a forbidden control, separator, bidi or noncharacter code point", ErrEditUsage, field)
	}
	return nil
}

// validateText checks receipt text before callers resolve a task or take locks.
func (o WriteOptions) validateText() error {
	for _, field := range []struct{ name, value string }{
		{"author", o.Who.ID}, {"author role", o.Who.Role},
		{"action", o.Action}, {"state", o.State}, {"lands hash", o.LandsHash},
	} {
		if err := checkTaskText(field.name, field.value, false); err != nil {
			return err
		}
	}
	return checkTaskText("note", o.Note, true)
}
