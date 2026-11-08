// Package frontmatter splits a Markdown file into its YAML front matter and body.
// The YAML itself is read by go.yaml.in/yaml/v4.
package frontmatter

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Document is one parsed file. Lines holds the file line of each top-level key.
type Document struct {
	Data     map[string]any
	Body     string
	BodyLine int
	Lines    map[string]int
}

// Parse reads the front matter between the first two "---" lines. The values are
// plain Go maps, lists, strings, numbers and booleans; a date stays the text that
// was written, so a note's "updated" field is a string.
func Parse(text string) (Document, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	d := Document{Data: map[string]any{}, Lines: map[string]int{}}
	if lines[0] != "---" {
		return d, errors.New("line 1: missing frontmatter block")
	}
	end := 1
	for end < len(lines) && lines[end] != "---" {
		end++
	}
	if end == len(lines) {
		return d, errors.New("line 1: unclosed frontmatter block")
	}
	d.Body = strings.Join(lines[end+1:], "\n")
	d.BodyLine = end + 2
	src := strings.Join(lines[1:end], "\n")
	if strings.TrimSpace(src) == "" {
		return d, nil
	}
	var doc yaml.Node
	if err := yaml.Load([]byte(src), &doc); err != nil {
		return d, fmt.Errorf("frontmatter: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return d, errors.New("line 2: frontmatter must be a map")
	}
	top := doc.Content[0]
	v, err := value(top)
	if err != nil {
		return d, err
	}
	d.Data = v.(map[string]any)
	for i := 0; i+1 < len(top.Content); i += 2 {
		d.Lines[top.Content[i].Value] = top.Content[i].Line + 1 // the opening --- is line 1
	}
	return d, nil
}

// Into copies a decoded value into a struct that carries json tags.
func Into(v any, target any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func value(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.AliasNode:
		return value(n.Alias)
	case yaml.MappingNode:
		m := make(map[string]any, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("line %d: mapping keys must be plain text", k.Line+1)
			}
			if _, dup := m[k.Value]; dup {
				return nil, fmt.Errorf("line %d: duplicate key %s", k.Line+1, k.Value)
			}
			v, err := value(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m[k.Value] = v
		}
		return m, nil
	case yaml.SequenceNode:
		s := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := value(c)
			if err != nil {
				return nil, err
			}
			s = append(s, v)
		}
		return s, nil
	}
	if n.ShortTag() == "!!timestamp" {
		return n.Value, nil
	}
	var v any
	if err := n.Decode(&v); err != nil {
		return nil, fmt.Errorf("line %d: %w", n.Line+1, err)
	}
	return v, nil
}
