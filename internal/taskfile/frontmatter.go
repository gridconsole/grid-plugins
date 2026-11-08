// Package taskfile reads the deliberately small frontmatter dialect shared with
// Grid's task contract. It is not a YAML parser.
package taskfile

import (
	"regexp"
	"strings"
)

type Frontmatter struct {
	Data map[string]any `json:"data"`
	Body string         `json:"body"`
	Odd  bool           `json:"odd"`
}

var block = regexp.MustCompile(`^---\r?\n([\s\S]*?)\r?\n---\r?\n?([\s\S]*)$`)
var field = regexp.MustCompile(`^([A-Za-z_][\w-]*):\s*(.*)$`)
var item = regexp.MustCompile(`^\s+-\s+(.*)$`)
var comment = regexp.MustCompile(`\s+#\s.*$`)

// ParseFrontmatter matches lib.mjs parseFrontmatter, including string-valued
// scalars, duplicate keys, nested inline lists, and its intentionally unquoted
// comma splitting. A missing block returns nil.
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
	v := strings.TrimSpace(comment.ReplaceAllString(raw, ""))
	if len(v) >= 2 && (v[0] == '\'' || v[0] == '"') && v[len(v)-1] == v[0] {
		v = v[1 : len(v)-1]
	}
	return v
}
func odd(key string, seen *bool) {
	if key == "__proto__" || key == "constructor" || key == "prototype" {
		*seen = true
	}
}
func value(raw string, seen *bool) any {
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
			if kv := field.FindStringSubmatch(strings.TrimSpace(p)); kv != nil {
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
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if strings.TrimSpace(s[start:]) != "" {
		out = append(out, strings.TrimSpace(s[start:]))
	}
	return out
}

// HistoryLine returns a line without a newline, escaping, or normalization.
func HistoryLine(time, event, detail, who string, via ...string) string {
	line := "- " + time + " · " + event
	if detail != "" {
		line += " · " + detail
	}
	line += " · by " + who
	if len(via) > 0 && via[0] != "" {
		line += " · via " + via[0]
	}
	return line
}
