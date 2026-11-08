package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Topic struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Levels      []string `json:"levels"`
	Description string   `json:"description"`
	Custom      bool     `json:"custom"`
}

// BuiltinTopics returns the eight topics in the first slice. Executable schemas
// remain owned by the topic commands; the brain keeper cannot write them.
func BuiltinTopics() []Topic {
	return []Topic{
		{Name: "decisions", Kind: "knowledge", Levels: []string{"home", "project"}, Description: "Decisions and their reasons"},
		{Name: "conventions", Kind: "knowledge", Levels: []string{"project"}, Description: "Project practices and lessons"},
		{Name: "environments", Kind: "config", Levels: []string{"project"}, Description: "Development and test settings"},
		{Name: "machines", Kind: "config", Levels: []string{"home"}, Description: "Computers and capacity"},
		{Name: "deploy", Kind: "config", Levels: []string{"project"}, Description: "Delivery targets and steps"},
		{Name: "people", Kind: "config", Levels: []string{"home", "project"}, Description: "Members and approvers"},
		{Name: "agents", Kind: "config", Levels: []string{"home", "project"}, Description: "Agent settings"},
		{Name: "routines", Kind: "config", Levels: []string{"home", "project"}, Description: "Scheduled work"},
	}
}

func allows(t Topic, level string) bool {
	for _, l := range t.Levels {
		if l == level {
			return true
		}
	}
	return false
}

func topicAt(root *os.Root, name, level string) (Topic, error) {
	if !ValidName(name) {
		return Topic{}, fmt.Errorf("topic must be a plain name")
	}
	for _, t := range BuiltinTopics() {
		if t.Name == name {
			if !allows(t, level) {
				return Topic{}, fmt.Errorf("topic %s is not available at %s level", name, level)
			}
			return t, nil
		}
	}
	// Reserved deferred topics cannot be smuggled in as custom knowledge topics.
	if name == "skills" || name == "workflows" || name == "design" || name == "research" || name == "incidents" {
		return Topic{}, fmt.Errorf("topic %s is not in the first slice; use conventions or decisions for knowledge", name)
	}
	b, err := read(root, filepath.Join("knowledge", name, "TOPIC.md"))
	if err != nil {
		return Topic{}, fmt.Errorf("topic %s needs a TOPIC.md definition: %w", name, err)
	}
	f, body, err := frontmatter(b)
	if err != nil {
		return Topic{}, err
	}
	get := func(key string) string { v, _ := scalar(f[key]); return v }
	if get("name") != name || get("kind") != "knowledge" {
		return Topic{}, fmt.Errorf("custom topic %s must declare its name and kind: knowledge", name)
	}
	declaredLevel := get("level")
	if declaredLevel != level && declaredLevel != "home, project" && declaredLevel != "[home, project]" && f["level"] != "[home, project]" {
		return Topic{}, fmt.Errorf("custom topic %s does not declare level %s", name, level)
	}
	description := get("description")
	if description == "" {
		for _, l := range strings.Split(body, "\n") {
			if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "#") {
				description = strings.TrimSpace(l)
				break
			}
		}
	}
	if description == "" {
		description = "Custom knowledge"
	}
	return Topic{Name: name, Kind: "knowledge", Levels: []string{level}, Description: description, Custom: true}, nil
}

func scopeTopics(root *os.Root, level string) ([]Topic, error) {
	var topics []Topic
	known := map[string]bool{}
	for _, t := range BuiltinTopics() {
		known[t.Name] = true
		if allows(t, level) {
			topics = append(topics, t)
		}
	}
	if err := noSymlinks(root, "knowledge"); err != nil {
		return nil, err
	}
	dirs, err := fs.ReadDir(root.FS(), "knowledge")
	if errors.Is(err, fs.ErrNotExist) {
		return topics, nil
	}
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if dir.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("brain paths cannot be symbolic links")
		}
		if !dir.IsDir() || known[dir.Name()] {
			continue
		}
		t, err := topicAt(root, dir.Name(), level)
		if err != nil {
			// Keep undiscovered or malformed topic folders visible as invalid.
			t = Topic{Name: dir.Name(), Kind: "invalid", Levels: []string{level}, Description: "Invalid topic definition", Custom: true}
		}
		topics = append(topics, t)
	}
	return topics, nil
}

// Topics includes custom knowledge definitions, project first. Built-in topic
// metadata always comes from code, so a local TOPIC.md cannot loosen its class.
func (s Store) Topics() ([]Topic, error) {
	result := BuiltinTopics()
	seen := map[string]int{}
	for i, t := range result {
		seen[t.Name] = i
	}
	for _, level := range s.levels() {
		root, err := s.scope(level, false)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		topics, err := scopeTopics(root, level)
		root.Close()
		if err != nil {
			return nil, err
		}
		for _, t := range topics {
			if i, ok := seen[t.Name]; ok {
				if t.Custom {
					result[i].Levels = append(result[i].Levels, level)
				}
				continue
			}
			seen[t.Name] = len(result)
			result = append(result, t)
		}
	}
	return result, nil
}

// scan returns only valid accepted knowledge. Invalid files are counted but
// never treated as facts; proposals, logs, definitions and indices are excluded.
func scan(root *os.Root, level string) ([]Note, map[string]int, error) {
	notes := []Note{}
	invalid := map[string]int{}
	if err := noSymlinks(root, "knowledge"); err != nil {
		return nil, nil, err
	}
	err := fs.WalkDir(root.FS(), "knowledge", func(path string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) && path == "knowledge" {
			return nil
		}
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("brain paths cannot be symbolic links")
		}
		parts := strings.Split(path, "/")
		if d.IsDir() || len(parts) < 3 || filepath.Ext(path) != ".md" || d.Name() == "TOPIC.md" {
			return nil
		}
		topic := parts[1]
		b, err := read(root, path)
		if err != nil {
			return err
		}
		n, err := Parse(b)
		if err == nil && n.Topic == "" {
			n.Topic = topic
		} // folder is the topic for handwritten notes
		if err != nil || n.Validate() != nil || n.Topic != topic || n.Level != level || n.Name+".md" != d.Name() || len(parts) != 3 || gate(root, n) != "" {
			invalid[topic]++
			return nil
		}
		if n.Status != "accepted" || n.SupersededBy != "" {
			return nil
		}
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	// Ambiguous identities cannot be resolved safely by show or a link.
	counts := map[string]int{}
	for _, n := range notes {
		counts[n.Name]++
	}
	kept := notes[:0]
	for _, n := range notes {
		if counts[n.Name] > 1 {
			invalid[n.Topic]++
			continue
		}
		kept = append(kept, n)
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].Name < kept[j].Name })
	return kept, invalid, nil
}
