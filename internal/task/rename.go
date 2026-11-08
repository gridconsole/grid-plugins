package task

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.yaml.in/yaml/v4"
)

// RenameAllowed is the default role policy for renaming tasks. A host or the
// workflow may replace it during initialization, before concurrent writes begin.
var RenameAllowed = defaultTaskEditAllowed

func defaultTaskEditAllowed(role string) bool {
	return role == "" || role == "owner" || role == "member"
}

var (
	// ErrEditUsage identifies invalid rename or decline input, before any write.
	ErrEditUsage = errors.New("invalid task edit")
	// ErrEditRejected identifies a rename or decline rejected by the current task.
	ErrEditRejected = errors.New("task edit rejected")
)

// RenameOptions carries a title and optional project constraint. Role is the
// host's role, independent of the author recorded in WriteOptions.Who.
type RenameOptions struct {
	WriteOptions
	Title, Project, Role string
}

// Rename changes only title and updated, preserving all other task.md bytes.
// It uses the same locked reread, journal and atomic commit as Change.
func (s Store) Rename(o RenameOptions) (Event, error) {
	title := strings.Join(strings.Fields(o.Title), " ")
	if hasInvalidEditText(o.Title) || strings.ContainsAny(o.Title, "\u2028\u2029") || !utf8.ValidString(o.Title) || title == "" || utf8.RuneCountInString(title) > 200 {
		return Event{}, fmt.Errorf("%w: title must have 1-200 characters on one line without control, bidi override/isolate, or noncharacter code points", ErrEditUsage)
	}
	if !RenameAllowed(strings.TrimSpace(o.Role)) {
		return Event{}, errors.New("scope: role cannot rename tasks")
	}
	o.Action, o.Note = "rename", "renamed · "+title
	return s.editTask(o.WriteOptions, o.Project, func(t Task, _ *yaml.Node, event *Event) (map[string]string, error) {
		if t.State == "closed" {
			return nil, fmt.Errorf("%w: %s is closed; reopen it before renaming", ErrEditRejected, t.ID)
		}
		if t.Title == title {
			return nil, fmt.Errorf("%w: %s already has that title", ErrEditRejected, t.ID)
		}
		return map[string]string{"title": title, "updated": event.When}, nil
	})
}

// Validate before whitespace folding so controls cannot disappear into spaces.
func hasInvalidEditText(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool {
		return unicode.IsControl(r) || unicode.Is(unicode.Noncharacter_Code_Point, r) ||
			(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
	})
}

// JSON strings are valid YAML scalars; keep HTML punctuation readable.
func encodeTaskScalar(value string) (string, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// editTask shares Change's transaction primitives; the callback sees the task
// reread under the lock and returns only the header fields it intends to change.
func (s Store) editTask(o WriteOptions, project string, edit func(Task, *yaml.Node, *Event) (map[string]string, error)) (Event, error) {
	var event Event
	if project != "" {
		resolved, err := s.resolve(o.Cwd, project)
		if err != nil {
			return event, err
		}
		project = resolved.Project
	}
	err := s.withWrite(o.Cwd, o.ID, func(t Task) error {
		if project != "" && t.Project != project {
			return fmt.Errorf("scope: task %s is in project %q, not %q", t.ID, t.Project, project)
		}
		data, err := readOptional(filepath.Join(t.Path, "task.md"))
		if err != nil {
			return err
		}
		var doc yaml.Node
		if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
			return err
		}
		if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return errors.New("task.md front matter is not a mapping")
		}
		mapping := doc.Content[0]
		event = s.event(t, o, t.State)
		fields, err := edit(t, mapping, &event)
		if err != nil {
			return err
		}
		updated, err := patchTaskFields(data, mapping, fields)
		if err != nil {
			return err
		}
		if _, err := Fold(t.Path, string(updated), t.Intent); err != nil {
			return err
		}
		return commitWrite(t.Path, event, map[string][]byte{"task.md": updated})
	})
	return event, err
}

// patchTaskFields uses YAML source positions instead of reserializing unrelated
// keys. Multi-line scalar values are replaced as a unit; body, comments, spacing
// and line endings elsewhere survive. Flow mappings and anchored edited values
// are refused rather than changing fields that alias them.
func patchTaskFields(data []byte, mapping *yaml.Node, fields map[string]string) ([]byte, error) {
	if mapping.Style&yaml.FlowStyle != 0 {
		return nil, errors.New("task.md edits need a block mapping")
	}
	lines := strings.SplitAfter(string(data), "\n")
	end := 1
	for end < len(lines) && strings.TrimSpace(lines[end]) != "---" {
		end++
	}
	if end == len(lines) {
		return nil, errors.New("task.md front matter is not closed")
	}
	newline := "\n"
	if strings.HasSuffix(lines[0], "\r\n") {
		newline = "\r\n"
	}
	type replacement struct {
		end  int
		text string
	}
	replacements := map[int]replacement{}
	found := map[string]bool{}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		changed, ok := fields[key.Value]
		if !ok {
			continue
		}
		if key.Column != 1 || value.Kind != yaml.ScalarNode || value.Anchor != "" || key.Line != value.Line {
			return nil, fmt.Errorf("task.md %s must be an unanchored scalar on its key's line", key.Value)
		}
		start, stop := key.Line-1, end
		if i+2 < len(mapping.Content) {
			stop = mapping.Content[i+2].Line - 1
		}
		// Comments and blank lines at the end of a scalar belong to the unchanged
		// surrounding document. Indented lines within a block scalar belong to it.
		for stop > start+1 {
			line := strings.TrimRight(lines[stop-1], "\r\n")
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "#") {
				break
			}
			stop--
		}
		encoded, err := encodeTaskScalar(changed)
		if err != nil {
			return nil, err
		}
		replacements[start] = replacement{stop, key.Value + ": " + encoded + newline}
		found[key.Value] = true
	}
	var out strings.Builder
	for i := 0; i < end; {
		if replacement, ok := replacements[i]; ok {
			out.WriteString(replacement.text)
			i = replacement.end
		} else {
			out.WriteString(lines[i])
			i++
		}
	}
	var missing []string
	for key := range fields {
		if !found[key] {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	for _, key := range missing {
		encoded, err := encodeTaskScalar(fields[key])
		if err != nil {
			return nil, err
		}
		out.WriteString(key + ": " + encoded + newline)
	}
	out.WriteString(strings.Join(lines[end:], ""))
	return []byte(out.String()), nil
}
