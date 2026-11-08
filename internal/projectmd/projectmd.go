// Package projectmd edits front-matter keys while retaining the project body.
package projectmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"go.yaml.in/yaml/v4"
)

type Result struct {
	Project string `json:"project"`
	Key     string `json:"key"`
	Value   any    `json:"value"`
}

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// Key resolves the project's name or id through project.Load. Set parses value
// as YAML (an omitted value is the empty string), validates identity fields with
// project.Parse, and preserves other keys and the Markdown body. project.Write
// cannot preserve those fields, so the shared atomic writer commits the edit.
func Key(home, name, key string, value *string) (Result, error) {
	result := Result{Key: key}
	if home == "" {
		return result, fmt.Errorf("cannot find the Grid home; set GRID_HOME")
	}
	if !keyPattern.MatchString(key) {
		return result, fmt.Errorf("key must be a single front-matter key")
	}
	records, err := project.Load(home)
	if err != nil {
		return result, err
	}
	var found *project.Record
	for _, r := range records {
		if r.Name == name || r.ID == name {
			if found != nil {
				return result, fmt.Errorf("project is ambiguous")
			}
			copy := r
			found = &copy
		}
	}
	if found == nil {
		return result, fmt.Errorf("project %q not found", name)
	}
	result.Project = found.Name
	root, err := os.OpenRoot(home)
	if err != nil {
		return result, err
	}
	defer root.Close()
	path := filepath.Join("projects", found.Name, "project.md")
	if err = safefile.NoSymlinks(root, path); err != nil {
		return result, err
	}
	if value != nil {
		lockPath := filepath.Join("projects", found.Name, ".project-md.lock")
		if err = safefile.NoSymlinks(root, lockPath); err != nil {
			return result, err
		}
		lock, err := fslock.OpenNoFollow(filepath.Join(home, lockPath), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return result, err
		}
		defer lock.Close()
		if err = fslock.LockExclusive(lock); err != nil {
			return result, err
		}
	}
	b, err := safefile.Read(root, path)
	if err != nil {
		return result, err
	}
	front, body, err := mdfile.Split(b)
	if err != nil {
		return result, err
	}
	var doc yaml.Node
	if err = mdfile.Load(front, &doc, false); err != nil {
		return result, err
	}
	if len(doc.Content) == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return result, fmt.Errorf("project front matter must be a mapping")
	}
	mapping := doc.Content[0]
	index := -1
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			index = i + 1
			break
		}
	}
	if value != nil {
		var node yaml.Node
		raw := *value
		if raw == "" {
			raw = `""`
		}
		if err = mdfile.Load([]byte(raw), &node, false); err != nil {
			return result, err
		}
		var decoded any
		if err = yaml.Load([]byte(raw), &decoded); err != nil {
			return result, err
		}
		if _, err = json.Marshal(decoded); err != nil {
			return result, fmt.Errorf("value cannot be represented as JSON: %w", err)
		}
		if !jsonValue(decoded) {
			return result, fmt.Errorf("value needs string mapping keys")
		}
		if len(node.Content) != 1 {
			return result, fmt.Errorf("value must be one YAML value")
		}
		if index < 0 {
			mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, node.Content[0])
			index = len(mapping.Content) - 1
		} else {
			mapping.Content[index] = node.Content[0]
		}
		front, err = yaml.Dump(&doc, yaml.WithIndent(2), yaml.WithLineWidth(-1))
		if err != nil {
			return result, err
		}
		b = []byte("---\n" + string(front) + "---\n" + body)
		if _, err = project.Parse(found.Name, string(b)); err != nil {
			return result, err
		}
		if err = safefile.AtomicWrite(root, path, b); err != nil {
			return result, err
		}
	}
	if index < 0 {
		return result, fmt.Errorf("front-matter key %q is not set", key)
	}
	encoded, err := yaml.Dump(mapping.Content[index])
	if err != nil {
		return result, err
	}
	if err = yaml.Load(encoded, &result.Value); err != nil {
		return result, err
	}
	// JSON requires string map keys; fail before a caller tries to print one.
	if !jsonValue(result.Value) {
		return result, fmt.Errorf("value needs string mapping keys")
	}
	return result, nil
}

func jsonValue(v any) bool {
	switch x := v.(type) {
	case map[any]any:
		return false
	case map[string]any:
		for _, v := range x {
			if !jsonValue(v) {
				return false
			}
		}
	case []any:
		for _, v := range x {
			if !jsonValue(v) {
				return false
			}
		}
	}
	return true
}

// Plain prints strings directly and structured values as compact YAML.
func Plain(v any) string {
	if s, ok := v.(string); ok {
		return strings.ReplaceAll(strings.ReplaceAll(s, "\r", "\\r"), "\n", "\\n")
	}
	b, _ := yaml.Dump(v, yaml.WithLineWidth(-1))
	return strings.TrimSpace(string(b))
}
