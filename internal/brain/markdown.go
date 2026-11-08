package brain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Encode writes YAML-compatible front matter: quoted scalar strings and JSON
// flow collections. This bounded format avoids a dependency on a YAML engine.
func Encode(n Note) ([]byte, error) {
	if n.Links == nil {
		n.Links = []Link{}
	}
	var b strings.Builder
	b.WriteString("---\n")
	fields := []struct{ key, value string }{
		{"name", n.Name}, {"topic", n.Topic}, {"type", n.Type},
		{"description", n.Description}, {"summary", n.Summary}, {"updated", n.Updated},
		{"source", n.Source}, {"superseded_by", n.SupersededBy}, {"level", n.Level},
		{"status", n.Status}, {"reason", n.Reason},
	}
	for _, f := range fields {
		v, err := json.Marshal(f.value)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&b, "%s: %s\n", f.key, v)
	}
	links, err := json.Marshal(n.Links)
	if err != nil {
		return nil, err
	}
	origin, err := json.Marshal(n.Origin)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&b, "links: %s\norigin: %s\n---\n%s\n", links, origin, strings.TrimRight(n.Body, "\n"))
	return []byte(b.String()), nil
}

// Parse accepts the emitted format plus plain/single-quoted scalars, literal
// and folded block scalars, and indented links/origin mappings. Unsupported YAML
// constructs fail explicitly instead of silently changing a note.
func Parse(data []byte) (Note, error) {
	fields, body, err := frontmatter(data)
	if err != nil {
		return Note{}, err
	}
	n := Note{Body: body, Links: []Link{}}
	scalars := map[string]*string{
		"name": &n.Name, "topic": &n.Topic, "type": &n.Type,
		"description": &n.Description, "summary": &n.Summary, "updated": &n.Updated,
		"source": &n.Source, "superseded_by": &n.SupersededBy, "level": &n.Level,
		"status": &n.Status, "reason": &n.Reason,
	}
	for key, raw := range fields {
		if p, ok := scalars[key]; ok {
			*p, err = scalar(raw)
		} else {
			switch key {
			case "links":
				n.Links, err = parseLinks(raw)
			case "origin":
				if strings.HasPrefix(strings.TrimSpace(raw), "{") {
					err = decodeJSON(raw, &n.Origin)
				} else {
					var m map[string]string
					m, err = mapping(raw)
					if err == nil {
						for k := range m {
							if k != "kind" && k != "id" && k != "tier" {
								err = fmt.Errorf("unsupported origin field %q", k)
							}
						}
						n.Origin = Origin{Kind: m["kind"], ID: m["id"], Tier: m["tier"]}
					}
				}
			default:
				err = fmt.Errorf("unsupported note field %q", key)
			}
		}
		if err != nil {
			return Note{}, fmt.Errorf("front matter %s: %w", key, err)
		}
	}
	return n, nil
}

func decodeJSON(raw string, dst any) error {
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	if !json.Valid([]byte(raw)) {
		return fmt.Errorf("expected a JSON flow collection")
	}
	return d.Decode(dst)
}

func frontmatter(data []byte) (map[string]string, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || lines[0] != "---" {
		return nil, "", fmt.Errorf("Markdown must start with --- front matter")
	}
	fields := map[string]string{}
	last := ""
	for i, l := range lines[1:] {
		if l == "---" {
			return fields, strings.Join(lines[i+2:], "\n"), nil
		}
		if strings.HasPrefix(l, " ") && last != "" {
			fields[last] += "\n" + l
			continue
		}
		if strings.TrimSpace(l) == "" {
			if last != "" && (strings.HasPrefix(fields[last], "|") || strings.HasPrefix(fields[last], ">")) {
				fields[last] += "\n  "
			}
			continue
		}
		if strings.HasPrefix(l, "#") {
			continue
		}
		k, v, ok := strings.Cut(l, ":")
		if !ok || strings.TrimSpace(k) != k || !ValidName(k) {
			return nil, "", fmt.Errorf("invalid front matter line %d", i+2)
		}
		if _, ok := fields[k]; ok {
			return nil, "", fmt.Errorf("duplicate front matter field %q", k)
		}
		fields[k] = strings.TrimSpace(v)
		last = k
	}
	return nil, "", fmt.Errorf("front matter has no closing ---")
}

func scalar(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	first, rest, multiline := strings.Cut(raw, "\n")
	if first == "|" || first == "|-" || first == ">" || first == ">-" {
		lines := strings.Split(rest, "\n")
		for i := range lines {
			lines[i] = strings.TrimSpace(lines[i])
		}
		sep := "\n"
		if strings.HasPrefix(first, ">") {
			sep = " "
		}
		return strings.Join(lines, sep), nil
	}
	if multiline {
		return "", fmt.Errorf("use | or > for a multiline scalar")
	}
	if strings.HasPrefix(raw, `"`) {
		return strconv.Unquote(raw)
	}
	if strings.HasPrefix(raw, "'") && strings.HasSuffix(raw, "'") && len(raw) >= 2 {
		return strings.ReplaceAll(raw[1:len(raw)-1], "''", "'"), nil
	}
	if strings.ContainsAny(raw[:1], "'[{&*!>") {
		return "", fmt.Errorf("unsupported scalar; use a quoted string")
	}
	v, _, _ := strings.Cut(raw, " #")
	return strings.TrimSpace(v), nil
}

func mapping(raw string) (map[string]string, error) {
	m := map[string]string{}
	for _, l := range strings.Split(strings.TrimSpace(raw), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(l), ":")
		if !ok {
			return nil, fmt.Errorf("expected a mapping")
		}
		if _, ok := m[k]; ok {
			return nil, fmt.Errorf("duplicate mapping field %q", k)
		}
		var err error
		m[k], err = scalar(strings.TrimSpace(v))
		if err != nil {
			return nil, err
		}
	}
	return m, nil
}

func parseLinks(raw string) ([]Link, error) {
	links := []Link{}
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		return linksFromJSON(raw)
	}
	if strings.TrimSpace(raw) == "" {
		return links, nil
	}
	var items []string
	for _, l := range strings.Split(strings.TrimSpace(raw), "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "- ") {
			items = append(items, l[2:])
		} else if len(items) > 0 {
			items[len(items)-1] += "\n" + l
		} else {
			return nil, fmt.Errorf("links must be a list")
		}
	}
	for _, item := range items {
		m, err := mapping(item)
		if err != nil {
			return nil, err
		}
		for k := range m {
			if k != "to" && k != "rel" && k != "summary" && k != "why" {
				return nil, fmt.Errorf("unsupported link field %q", k)
			}
		}
		links = append(links, Link{To: m["to"], Rel: m["rel"], Summary: m["summary"], Why: m["why"]})
	}
	return links, nil
}

func linksFromJSON(raw string) ([]Link, error) {
	links := []Link{}
	err := decodeJSON(raw, &links)
	if links == nil {
		links = []Link{}
	}
	return links, err
}
