// Package mdfile reads the strict YAML front matter shared by Grid documents.
package mdfile

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v4"
)

// Load decodes YAML that a writer of the brain controls, so it first
// refuses the constructs that make a document mean something other than what it
// shows. Strict also refuses fields the target does not declare.
func Load(front []byte, out any, strict bool) error {
	var doc yaml.Node
	if err := yaml.Load(front, &doc, yaml.WithUniqueKeys(), yaml.WithSingleDocument()); err != nil {
		return fmt.Errorf("front matter: %w", err)
	}
	if err := plainNodes(&doc); err != nil {
		return fmt.Errorf("front matter: %w", err)
	}
	opts := []yaml.Option{yaml.WithUniqueKeys(), yaml.WithSingleDocument()}
	if strict {
		opts = append(opts, yaml.WithKnownFields())
	}
	if err := yaml.Load(front, out, opts...); err != nil {
		return fmt.Errorf("front matter: %w", err)
	}
	return nil
}

func plainNodes(n *yaml.Node) error {
	if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Style&yaml.TaggedStyle != 0 {
		return fmt.Errorf("anchors, aliases and tags are not supported (line %d)", n.Line)
	}
	for _, c := range n.Content {
		if err := plainNodes(c); err != nil {
			return err
		}
	}
	return nil
}

// SplitOptional is Split for a file written by a person, where front matter is
// optional. A file that does not start with a --- line, or whose --- block is
// never closed (a Markdown rule, not front matter), has empty front matter and
// its whole text as the body. A leading byte order mark is not part of the test
// and stays in the body.
func SplitOptional(data []byte) ([]byte, string, error) {
	probe := data
	if len(probe) >= 3 && probe[0] == 0xEF && probe[1] == 0xBB && probe[2] == 0xBF {
		probe = probe[3:]
	}
	if front, body, err := Split(probe); err == nil {
		return front, body, nil
	}
	return nil, string(data), nil
}

// Split returns YAML between the opening and closing --- lines and the
// unmodified Markdown body, including its original line endings.
func Split(data []byte) ([]byte, string, error) {
	lines := strings.SplitAfter(string(data), "\n")
	content := func(line string) string { return strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r") }
	if content(lines[0]) != "---" {
		return nil, "", fmt.Errorf("Markdown must start with --- front matter")
	}
	for i, l := range lines[1:] {
		if content(l) == "---" {
			front := strings.ReplaceAll(strings.Join(lines[1:i+1], ""), "\r\n", "\n")
			return []byte(front), strings.Join(lines[i+2:], ""), nil
		}
	}
	return nil, "", fmt.Errorf("front matter has no closing ---")
}
