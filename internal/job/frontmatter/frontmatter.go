// Package frontmatter reads the small YAML/JSON subset used by Job files.
// It supports maps, sequences, flow collections and quoted scalars. Block scalars, YAML
// tags, aliases and implicit dates are not supported.
package frontmatter

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Document struct {
	Data     map[string]any
	Body     string
	BodyLine int
	Lines    map[string]int
}

func Parse(text string) (Document, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	d := Document{Lines: map[string]int{}}
	if len(lines) == 0 || lines[0] != "---" {
		return d, fmt.Errorf("line 1: missing frontmatter block")
	}
	end := 1
	for end < len(lines) && lines[end] != "---" {
		end++
	}
	if end == len(lines) {
		return d, fmt.Errorf("line 1: unclosed frontmatter block")
	}
	v, err := Decode(strings.Join(lines[1:end], "\n"))
	if err != nil {
		return d, err
	}
	var ok bool
	d.Data, ok = v.(map[string]any)
	if !ok {
		return d, fmt.Errorf("line 2: frontmatter must be a map")
	}
	for i, s := range lines[1:end] {
		if len(s) > 0 && s[0] != ' ' {
			if k, _, ok := pair(s); ok {
				d.Lines[k] = i + 2
			}
		}
	}
	d.Body = strings.Join(lines[end+1:], "\n")
	d.BodyLine = end + 2
	return d, nil
}

func Into(v any, target any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return json.Unmarshal(b, target)
}

type row struct {
	indent, line int
	text         string
}
type parser struct {
	rows []row
	pos  int
}

func Decode(s string) (any, error) {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		return v, nil
	}
	p := parser{}
	for i, l := range strings.Split(s, "\n") {
		if strings.Contains(l, "\t") {
			return nil, fmt.Errorf("line %d: tabs are not supported", i+2)
		}
		n := len(l) - len(strings.TrimLeft(l, " "))
		l = stripComment(strings.TrimSpace(l))
		if l != "" {
			p.rows = append(p.rows, row{n, i + 2, l})
		}
	}
	if len(p.rows) == 0 {
		return map[string]any{}, nil
	}
	v, e := p.block(p.rows[0].indent)
	if e == nil && p.pos < len(p.rows) {
		e = fmt.Errorf("line %d: unexpected indentation", p.rows[p.pos].line)
	}
	return v, e
}

func (p *parser) block(indent int) (any, error) {
	if strings.HasPrefix(p.rows[p.pos].text, "- ") || p.rows[p.pos].text == "-" {
		out := []any{}
		for p.pos < len(p.rows) && p.rows[p.pos].indent == indent {
			r := p.rows[p.pos]
			if r.text != "-" && !strings.HasPrefix(r.text, "- ") {
				return nil, fmt.Errorf("line %d: expected list item", r.line)
			}
			rest := strings.TrimSpace(strings.TrimPrefix(r.text, "-"))
			p.pos++
			var v any
			var e error
			if k, x, ok := pair(rest); ok && !strings.HasPrefix(rest, "{") {
				m := map[string]any{}
				v, e = p.value(x, indent+2)
				m[k] = v
				if e == nil && p.pos < len(p.rows) && p.rows[p.pos].indent > indent {
					extra, err := p.block(p.rows[p.pos].indent)
					e = err
					if mm, ok := extra.(map[string]any); ok {
						for k, v := range mm {
							if _, has := m[k]; has {
								return nil, fmt.Errorf("line %d: duplicate key %s", r.line, k)
							}
							m[k] = v
						}
					} else if e == nil {
						e = fmt.Errorf("line %d: expected map", r.line)
					}
				}
				v = m
			} else {
				v, e = p.value(rest, indent)
			}
			if e != nil {
				return nil, fmt.Errorf("line %d: %w", r.line, e)
			}
			out = append(out, v)
		}
		return out, nil
	}
	out := map[string]any{}
	for p.pos < len(p.rows) && p.rows[p.pos].indent == indent {
		r := p.rows[p.pos]
		k, x, ok := pair(r.text)
		if !ok {
			return nil, fmt.Errorf("line %d: expected key: value", r.line)
		}
		if _, has := out[k]; has {
			return nil, fmt.Errorf("line %d: duplicate key %s", r.line, k)
		}
		p.pos++
		v, e := p.value(x, indent)
		if e != nil {
			return nil, fmt.Errorf("line %d: %w", r.line, e)
		}
		out[k] = v
	}
	return out, nil
}
func (p *parser) value(s string, indent int) (any, error) {
	if s == "" {
		if p.pos < len(p.rows) && p.rows[p.pos].indent > indent {
			return p.block(p.rows[p.pos].indent)
		}
		return nil, nil
	}
	// Literal blocks deliberately fail rather than silently changing source text.
	if s == "|" || s == ">" || strings.HasPrefix(s, "&") || strings.HasPrefix(s, "*") || strings.HasPrefix(s, "!") {
		return nil, fmt.Errorf("unsupported YAML scalar; use a quoted string")
	}
	return scalar(s)
}
func scalar(s string) (any, error) {
	s = strings.TrimSpace(s)
	var decoded any
	if json.Unmarshal([]byte(s), &decoded) == nil {
		return decoded, nil
	}
	if s == "" {
		return "", nil
	}
	if s[0] == '[' || s[0] == '{' {
		close := byte(']')
		if s[0] == '{' {
			close = '}'
		}
		if s[len(s)-1] != close {
			return nil, fmt.Errorf("unclosed flow collection")
		}
		parts, e := split(s[1:len(s)-1], ',')
		if e != nil {
			return nil, e
		}
		if s[0] == '[' {
			a := []any{}
			for _, x := range parts {
				v, e := scalar(x)
				if e != nil {
					return nil, e
				}
				a = append(a, v)
			}
			return a, nil
		}
		m := map[string]any{}
		for _, x := range parts {
			k, x, ok := pair(x)
			if !ok {
				return nil, fmt.Errorf("expected flow key: value")
			}
			if _, ok := m[k]; ok {
				return nil, fmt.Errorf("duplicate key %s", k)
			}
			v, e := scalar(x)
			if e != nil {
				return nil, e
			}
			m[k] = v
		}
		return m, nil
	}
	if s[0] == '"' {
		var v string
		if err := json.Unmarshal([]byte(s), &v); err != nil {
			return nil, err
		}
		return v, nil
	}
	if s[0] == '\'' {
		if len(s) < 2 || s[len(s)-1] != '\'' {
			return nil, fmt.Errorf("unclosed quote")
		}
		return strings.ReplaceAll(s[1:len(s)-1], "''", "'"), nil
	}
	switch s {
	case "null", "~":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if n, e := strconv.Atoi(s); e == nil {
		return n, nil
	}
	return s, nil
}
func pair(s string) (string, string, bool) {
	quote := byte(0)
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '[' || c == '{' {
			depth++
		}
		if c == ']' || c == '}' {
			depth--
		}
		if c == ':' && depth == 0 && (i+1 == len(s) || s[i+1] == ' ') {
			k := strings.Trim(strings.TrimSpace(s[:i]), "\"'")
			return k, strings.TrimSpace(s[i+1:]), k != ""
		}
	}
	return "", "", false
}
func split(s string, delimiter byte) ([]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	out := []string{}
	start, depth := 0, 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		switch c {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		}
		if c == delimiter && depth == 0 {
			out = append(out, strings.TrimSpace(s[start:i]))
			start = i + 1
		}
	}
	if depth != 0 || quote != 0 {
		return nil, fmt.Errorf("unclosed collection or quote")
	}
	out = append(out, strings.TrimSpace(s[start:]))
	return out, nil
}
func stripComment(s string) string {
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == '\\' && quote == '"' {
				i++
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
		}
		if c == '#' && (i == 0 || s[i-1] == ' ') {
			return strings.TrimSpace(s[:i])
		}
	}
	return s
}
