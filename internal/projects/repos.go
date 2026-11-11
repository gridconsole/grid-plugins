package projects

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// RepoItem is one repository of projects/<project>/repos.json, the file the app's
// Settings tab reads. It keeps every key in the order the file has it, so a key
// other than id, where, remote and branch survives an edit unchanged.
type RepoItem struct{ fields []repoField }

type repoField struct {
	Key   string
	Value json.RawMessage
}

// Get is the string value of key; it is empty when the key is absent or not a string.
func (it RepoItem) Get(key string) string {
	for _, f := range it.fields {
		if f.Key == key {
			var s string
			if json.Unmarshal(f.Value, &s) == nil {
				return s
			}
		}
	}
	return ""
}

// set replaces key in place, or appends it.
func (it *RepoItem) set(key, value string) {
	raw := jsonString(value)
	for i := range it.fields {
		if it.fields[i].Key == key {
			it.fields[i].Value = raw
			return
		}
	}
	it.fields = append(it.fields, repoField{key, raw})
}

func jsonString(s string) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // a string always encodes
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}

// MarshalJSON writes the keys in file order.
func (it RepoItem) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range it.fields {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(jsonString(f.Key))
		b.WriteByte(':')
		if err := json.Compact(&b, f.Value); err != nil {
			return nil, err
		}
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// UnmarshalJSON reads an object and keeps its key order; a repeated key keeps the last value, as a map decode does.
func (it *RepoItem) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return errors.New("a repository is a JSON object")
	}
	var fields []repoField
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return err
		}
		if i := indexField(fields, key); i >= 0 {
			fields[i].Value = value
		} else {
			fields = append(fields, repoField{key, value})
		}
	}
	it.fields = fields
	return nil
}

func indexField(fields []repoField, key string) int {
	for i, f := range fields {
		if f.Key == key {
			return i
		}
	}
	return -1
}

// RepoChange holds the fields an edit was given; a nil field is left as it is.
type RepoChange struct{ Where, Remote, Branch *string }

// RepoAdd appends a repository to the project's repos.json. An empty id becomes
// the last part of where; an id already in the file is refused.
func (s Store) RepoAdd(name, id, path, remote, branch string) (RepoItem, error) {
	if strings.TrimSpace(path) == "" {
		return RepoItem{}, errors.New("where is empty")
	}
	if id == "" {
		id = filepath.Base(path)
		if id == "." || id == string(filepath.Separator) || id == ".." {
			return RepoItem{}, fmt.Errorf("cannot take an id from %q; give one", path)
		}
	}
	var item RepoItem
	err := s.repoChange(name, func(items []json.RawMessage) ([]json.RawMessage, error) {
		if i, _ := repoIndex(items, id); i >= 0 {
			return nil, fmt.Errorf("repository %q is already in project %q", id, name)
		}
		item.set("id", id)
		item.set("where", path)
		item.set("remote", remote)
		item.set("branch", branch)
		raw, err := item.MarshalJSON()
		return append(items, raw), err
	})
	return item, err
}

// RepoEdit changes only the fields given and keeps every other key of the item.
func (s Store) RepoEdit(name, id string, change RepoChange) (RepoItem, error) {
	var item RepoItem
	err := s.repoChange(name, func(items []json.RawMessage) ([]json.RawMessage, error) {
		i, found := repoIndex(items, id)
		if i < 0 {
			return nil, fmt.Errorf("project %q has no repository %q", name, id)
		}
		item = found
		for _, c := range []struct {
			key string
			val *string
		}{{"where", change.Where}, {"remote", change.Remote}, {"branch", change.Branch}} {
			if c.val != nil {
				item.set(c.key, *c.val)
			}
		}
		raw, err := item.MarshalJSON()
		out := append([]json.RawMessage(nil), items...)
		out[i] = raw
		return out, err
	})
	return item, err
}

// RepoRemove takes one repository out of the project's repos.json.
func (s Store) RepoRemove(name, id string) error {
	return s.repoChange(name, func(items []json.RawMessage) ([]json.RawMessage, error) {
		i, _ := repoIndex(items, id)
		if i < 0 {
			return nil, fmt.Errorf("project %q has no repository %q", name, id)
		}
		return append(append([]json.RawMessage(nil), items[:i]...), items[i+1:]...), nil
	})
}

// repoIndex finds the first object item with this id; an item that is not an object is never a match.
func repoIndex(items []json.RawMessage, id string) (int, RepoItem) {
	for i, raw := range items {
		var it RepoItem
		if it.UnmarshalJSON(raw) == nil && it.Get("id") == id {
			return i, it
		}
	}
	return -1, RepoItem{}
}

// repoChange runs fn on the project's list under the projects lock and writes the
// result atomically. When fn fails nothing is written.
func (s Store) repoChange(name string, fn func([]json.RawMessage) ([]json.RawMessage, error)) error {
	home := s.Resolver.GridHome
	if home == "" {
		return errors.New("cannot find the Grid home; set GRID_HOME")
	}
	records, err := s.Resolver.Projects()
	if err != nil {
		return err
	}
	project, matches := "", 0
	for _, r := range records {
		if (r.Name == name || r.ID == name) && s.Resolver.PersonalProjectAllowed(r.Name) {
			project, matches = r.Name, matches+1
		}
	}
	switch {
	case matches == 0:
		return &where.LookupError{Kind: where.ErrProjectUnregistered, Message: fmt.Sprintf("project %q is not registered", name)}
	case matches > 1:
		return fmt.Errorf("project %q is ambiguous", name)
	}
	unlock, err := localdata.Lock(home, ".projects.lock")
	if err != nil {
		return err
	}
	defer unlock()
	target, err := localdata.Safe(home, "projects", project, "repos.json")
	if err != nil {
		return err
	}
	items, err := readRepoItems(target)
	if err != nil {
		return fmt.Errorf("project %q: %w", name, err)
	}
	items, err = fn(items)
	if err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString(`{"items":[`)
	for i, raw := range items {
		if i > 0 {
			b.WriteByte(',')
		}
		if err := json.Compact(&b, raw); err != nil {
			return err
		}
	}
	b.WriteString("]}\n")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return atomicfile.Write(target, b.Bytes(), 0o644)
}

// readRepoItems reads the file the way the app does: one object, items an array, nothing after
// it. A missing file is an empty list; a file that is anything else is refused, never overwritten.
func readRepoItems(path string) ([]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	bad := errors.New(`repos.json is not one object {"items":[...]}`)
	var doc map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(data))
	if dec.Decode(&doc) != nil || dec.Decode(new(any)) != io.EOF {
		return nil, bad
	}
	raw := bytes.TrimSpace(doc["items"])
	var items []json.RawMessage
	if !bytes.HasPrefix(raw, []byte("[")) || json.Unmarshal(raw, &items) != nil {
		return nil, bad
	}
	return items, nil
}
