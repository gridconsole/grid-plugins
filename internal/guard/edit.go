package guard

import "strings"

type edit struct {
	old, new string
	all      bool
}

// after enumerates CRLF and literal/JavaScript replacement interpretations.
func after(tool string, input map[string]any, text string) []string {
	if tool == "Write" {
		if s, ok := input["content"].(string); ok {
			return []string{s}
		}
		return nil
	}
	raw := []any{input}
	if tool == "MultiEdit" {
		raw, _ = input["edits"].([]any)
	}
	if len(raw) == 0 {
		return nil
	}
	edits := []edit{}
	for _, r := range raw {
		e, ok := r.(map[string]any)
		if !ok {
			return nil
		}
		old, a := e["old_string"].(string)
		next, b := e["new_string"].(string)
		if !a || !b {
			return nil
		}
		all := false
		if v, present := e["replace_all"]; present {
			var good bool
			all, good = v.(bool)
			if !good {
				return nil
			}
		}
		edits = append(edits, edit{old, next, all})
	}
	out := []string{}
	for _, start := range []string{text, strings.ReplaceAll(text, "\r\n", "\n")} {
		for _, literal := range []bool{true, false} {
			cur := start
			ok := true
			for _, e := range edits {
				cur, ok = applyEdit(cur, e, literal)
				if !ok {
					break
				}
			}
			if ok {
				out = append(out, cur)
			}
		}
	}
	return out
}
func applyEdit(text string, e edit, literal bool) (string, bool) {
	if e.old == "" {
		return e.new, text == ""
	}
	if !strings.Contains(text, e.old) {
		return "", false
	}
	var out strings.Builder
	pos := 0
	for {
		i := strings.Index(text[pos:], e.old)
		if i < 0 {
			out.WriteString(text[pos:])
			break
		}
		i += pos
		out.WriteString(text[pos:i])
		if literal {
			out.WriteString(e.new)
		} else {
			out.WriteString(replacement(e.new, text, i, len(e.old)))
		}
		pos = i + len(e.old)
		if !e.all {
			out.WriteString(text[pos:])
			break
		}
	}
	return out.String(), true
}
func replacement(s, text string, at, n int) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '$' && i+1 < len(s) {
			switch s[i+1] {
			case '$':
				out.WriteByte('$')
				i++
				continue
			case '&':
				out.WriteString(text[at : at+n])
				i++
				continue
			case '`':
				out.WriteString(text[:at])
				i++
				continue
			case '\'':
				out.WriteString(text[at+n:])
				i++
				continue
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}
