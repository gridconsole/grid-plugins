// Package taskfile reads the deliberately small frontmatter dialect shared with
// Grid's task contract. It is not a YAML parser.
package taskfile

import (
	"encoding/json"
	"regexp"
	"strings"
)

type Frontmatter struct {
	Data map[string]any `json:"data"`
	Body string         `json:"body"`
	Odd  bool           `json:"odd"`
}

var block = regexp.MustCompile(`^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$`)

// JavaScript's whitespace includes BOM and excludes U+0085; Go's \s and
// strings.TrimSpace differ on both, so keep the legacy dialect explicit.
const jsSpace = `[\t\n\v\f\r \x{00a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

var field = regexp.MustCompile(`^([A-Za-z_][\w-]*):` + jsSpace + `*(.*)$`)
var item = regexp.MustCompile(`^` + jsSpace + `+-` + jsSpace + `+(.*)$`)
var comment = regexp.MustCompile(jsSpace + `+#` + jsSpace + `.*$`)

// ParseFrontmatter follows lib.mjs parseFrontmatter, including string-valued
// scalars, duplicate keys, nested inline lists, and its intentionally unquoted
// comma splitting, with JSON decoding for double-quoted scalars written by task
// edits. A missing block returns nil.
func ParseFrontmatter(text string) *Frontmatter {
	m := block.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	f := &Frontmatter{Data: map[string]any{}, Body: m[2]}
	key := ""
	for _, line := range strings.Split(strings.ReplaceAll(m[1], "\r\n", "\n"), "\n") {
		if v := item.FindStringSubmatch(line); v != nil && key != "" {
			list, _ := f.Data[key].([]any)
			f.Data[key] = append(list, value(v[1], &f.Odd))
			continue
		}
		key = ""
		if kv := field.FindStringSubmatch(line); kv != nil {
			odd(kv[1], &f.Odd)
			f.Data[kv[1]] = value(kv[2], &f.Odd)
			if s, ok := f.Data[kv[1]].(string); ok && s == "" {
				key = kv[1]
			}
		}
	}
	return f
}

// ScalarFrontmatter is the older brief reader, which ignores list items.
func ScalarFrontmatter(text string) *Frontmatter {
	m := block.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	f := &Frontmatter{Data: map[string]any{}, Body: m[2]}
	for _, line := range strings.Split(strings.ReplaceAll(m[1], "\r\n", "\n"), "\n") {
		if kv := field.FindStringSubmatch(line); kv != nil {
			f.Data[kv[1]] = scalar(kv[2])
		}
	}
	return f
}

func scalar(raw string) string {
	if v, ok := quotedJSONScalar(raw); ok {
		return v
	}
	v := trimSpace(comment.ReplaceAllString(raw, ""))
	if len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	}
	return v
}

// Decode before stripping comments: a hash inside a JSON string is literal.
// Only an empty suffix or a legacy inline comment may follow the string.
func quotedJSONScalar(raw string) (string, bool) {
	raw = trimSpace(raw)
	if !strings.HasPrefix(raw, "\"") {
		return "", false
	}
	var value string
	decoder := json.NewDecoder(strings.NewReader(raw))
	if err := decoder.Decode(&value); err != nil {
		return "", false
	}
	rest := raw[decoder.InputOffset():]
	return value, trimSpace(comment.ReplaceAllString(rest, "")) == ""
}

func odd(key string, seen *bool) {
	if key == "__proto__" || key == "constructor" || key == "prototype" {
		*seen = true
	}
}
func value(raw string, seen *bool) any {
	if v, ok := quotedJSONScalar(raw); ok {
		return v
	}
	v := scalar(raw)
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		out := []any{}
		for _, p := range splitTop(v[1 : len(v)-1]) {
			x := value(p, seen)
			if s, ok := x.(string); !ok || s != "" {
				out = append(out, x)
			}
		}
		return out
	}
	if strings.HasPrefix(v, "{") && strings.HasSuffix(v, "}") {
		out := map[string]any{}
		for _, p := range splitTop(v[1 : len(v)-1]) {
			if kv := field.FindStringSubmatch(trimSpace(p)); kv != nil {
				odd(kv[1], seen)
				out[kv[1]] = value(kv[2], seen)
			}
		}
		return out
	}
	return v
}
func splitTop(s string) []string {
	out := []string{}
	depth, start := 0, 0
	for i, c := range s {
		switch c {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
		if c == ',' && depth == 0 {
			out = append(out, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	if trimSpace(s[start:]) != "" {
		out = append(out, trimSpace(s[start:]))
	}
	return out
}

// HistoryLine returns a line without a newline, escaping, or normalization.
// The optional trailing arguments are the via (first) and the why (second): the
// why is written last, as " · why <text>", and left out when it is empty.
func HistoryLine(time, event, detail, who string, via ...string) string {
	line := "- " + time + " · " + event
	if detail != "" {
		line += " · " + detail
	}
	line += " · by " + who
	if len(via) > 0 && via[0] != "" {
		line += " · via " + via[0]
	}
	if len(via) > 1 && via[1] != "" {
		line += " · why " + via[1]
	}
	return line
}

func trimSpace(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return r == '\t' || r == '\n' || r == '\v' || r == '\f' || r == '\r' || r == ' ' || r == 0xa0 || r == 0x1680 || (r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 || r == 0x202f || r == 0x205f || r == 0x3000 || r == 0xfeff
	})
}
