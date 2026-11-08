package golden

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// All normalization rules mirror recordings/record.py:108-122. Order matters:
//   - Replace fixture, reference and recorder paths (longest first), then the
//     case directory with $GRID_HOME/... Resolved sibling paths stay relative.
//   - ISO dates through minutes, optional seconds/fractions/Z become <TIME>.
//   - Thirteen-digit snapshot clocks after ':' and before ':', '"' or whitespace
//     become <TIME>; the file size is retained.
//   - Uppercase Crockford ULIDs become <ID-n> in first-seen order, shared across
//     file diffs, environment, stdout and stderr. JSON is never reserialized.
//
// Raw snapshots are compared BEFORE normalization; a clock-only change remains
// visible in files.diff. No newline, whitespace or JSON-key sorting is performed.
var (
	isoTime      = regexp.MustCompile(`\p{Nd}{4}-\p{Nd}\p{Nd}-\p{Nd}\p{Nd}T\p{Nd}\p{Nd}:\p{Nd}\p{Nd}(?::\p{Nd}\p{Nd}(?:\.\p{Nd}+)?Z?)?`)
	snapshotTime = regexp.MustCompile(`:\p{Nd}{13}(?:\.\p{Nd}+)?`)
	ulid         = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
)

type normalizer struct {
	paths  []string
	tokens map[string]string
	box    string
	ids    map[string]string
}

func newNormalizer(box string, tokens map[string]string) *normalizer {
	n := &normalizer{box: box, tokens: tokens, ids: map[string]string{}}
	for actual := range tokens {
		n.paths = append(n.paths, actual)
	}
	sort.Slice(n.paths, func(i, j int) bool { return len(n.paths[i]) > len(n.paths[j]) })
	return n
}

func (n *normalizer) text(s string) string {
	for _, actual := range n.paths {
		s = strings.ReplaceAll(s, actual, n.tokens[actual])
	}
	s = strings.ReplaceAll(s, n.box, "$GRID_HOME/..")
	s = replaceWords(s, isoTime, false, func(string) string { return "<TIME>" })
	// Keep the following separator unconsumed, so consecutive clocks both match.
	matches := snapshotTime.FindAllStringIndex(s, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		next, _ := utf8.DecodeRuneInString(s[m[1]:])
		if next == ':' || next == '"' || unicode.IsSpace(next) || (next >= 0x1c && next <= 0x1f) {
			s = s[:m[0]+1] + "<TIME>" + s[m[1]:]
		}
	}
	return replaceWords(s, ulid, true, func(id string) string {
		if n.ids[id] == "" {
			n.ids[id] = "<ID-" + strconv.Itoa(len(n.ids)+1) + ">"
		}
		return n.ids[id]
	})
}

// Python's word boundaries are Unicode-aware; Go regexp's \b is ASCII-only.
func replaceWords(s string, pattern *regexp.Regexp, endBoundary bool, replace func(string) string) string {
	word := func(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsNumber(r) }
	var out strings.Builder
	start := 0
	for _, m := range pattern.FindAllStringIndex(s, -1) {
		previous, _ := utf8.DecodeLastRuneInString(s[:m[0]])
		next, _ := utf8.DecodeRuneInString(s[m[1]:])
		if word(previous) || (endBoundary && word(next)) {
			continue
		}
		out.WriteString(s[start:m[0]])
		out.WriteString(replace(s[m[0]:m[1]]))
		start = m[1]
	}
	out.WriteString(s[start:])
	return out.String()
}
