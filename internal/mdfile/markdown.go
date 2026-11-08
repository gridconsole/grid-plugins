// Package mdfile reads the strict YAML front matter shared by Grid documents.
package mdfile

import (
	"fmt"
	"go.yaml.in/yaml/v4"
	"strings"
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

// Split returns the text between the opening and closing --- lines
// and the Markdown after them.
func Split(data []byte) ([]byte, string, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	if lines[0] != "---" {
		return nil, "", fmt.Errorf("Markdown must start with --- front matter")
	}
	for i, l := range lines[1:] {
		if l == "---" {
			return []byte(strings.Join(lines[1:i+1], "\n")), strings.Join(lines[i+2:], "\n"), nil
		}
	}
	return nil, "", fmt.Errorf("front matter has no closing ---")
}
