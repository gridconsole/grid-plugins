// Package projectenv saves, edits, removes and lists a project's environments
// as brain config entries (topic "environments", project level). The bytes of
// an entry are a contract: the Grid app reads exactly the shape Render writes.
// Secrets are never written as values: an entry names a secret and says
// whether it is set, nothing more.
package projectenv

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

const (
	level = "project"
	topic = "environments"
)

// Entry is one environment. Unset scalars and empty lists are not written.
type Entry struct {
	ID      string
	Name    string
	Kind    string // local, remote, cloud or deploy (a deploy target); empty when unset
	Where   string
	Runs    string
	Test    string
	How     string
	From    string
	Who     []string
	Secrets []Secret
	Steps   []Step
	// Approval is nil when the environment does not say.
	Approval *bool
}

// Secret names a secret an environment needs; Set says whether it is in place.
// It never holds the secret's value.
type Secret struct {
	Name string
	Set  bool
}

// Step is one workflow step an environment runs.
type Step struct {
	WF   string
	Step string
	Text string
}

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
	secretPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Errors name the field and never the value, so a refused secret is not echoed.
func oneLine(field, s string) error {
	if strings.ContainsAny(s, "\r\n\x00") {
		return fmt.Errorf("environment %s must be one line", field)
	}
	if brain.LooksSecret(s) {
		return fmt.Errorf("environment %s looks like it holds a secret value; name the secret instead", field)
	}
	return nil
}

func validID(id string) error {
	if len(id) > 100 || !idPattern.MatchString(id) || !brain.ValidName(id) || id == "TOPIC" {
		return errors.New("environment id must be lowercase letters, digits and hyphens, at most 100 bytes")
	}
	return oneLine("id", id)
}

// clean returns the entry as it is stored: every string trimmed, empty list
// items refused. It runs before anything is written.
func clean(e Entry) (Entry, error) {
	if err := validID(e.ID); err != nil {
		return Entry{}, err
	}
	out := Entry{ID: e.ID, Approval: e.Approval}
	for _, f := range []struct {
		field string
		in    string
		out   *string
	}{
		{"name", e.Name, &out.Name}, {"kind", e.Kind, &out.Kind}, {"where", e.Where, &out.Where},
		{"runs", e.Runs, &out.Runs}, {"test", e.Test, &out.Test}, {"how", e.How, &out.How}, {"from", e.From, &out.From},
	} {
		if err := oneLine(f.field, f.in); err != nil {
			return Entry{}, err
		}
		*f.out = strings.TrimSpace(f.in)
	}
	if out.Name == "" {
		return Entry{}, errors.New("environment name is required")
	}
	switch out.Kind {
	case "", "local", "remote", "cloud", "deploy":
	default:
		return Entry{}, errors.New("environment kind must be local, remote, cloud or deploy")
	}
	for _, w := range e.Who {
		if err := oneLine("who entry", w); err != nil {
			return Entry{}, err
		}
		if w = strings.TrimSpace(w); w == "" {
			return Entry{}, errors.New("environment who entry is empty")
		}
		out.Who = append(out.Who, w)
	}
	for _, sec := range e.Secrets {
		if err := oneLine("secret name", sec.Name); err != nil {
			return Entry{}, err
		}
		name := strings.TrimSpace(sec.Name)
		if !secretPattern.MatchString(name) {
			return Entry{}, errors.New("environment secret name must be a variable name such as API_TOKEN, never a value")
		}
		out.Secrets = append(out.Secrets, Secret{Name: name, Set: sec.Set})
	}
	for _, st := range e.Steps {
		var s Step
		for _, f := range []struct {
			field string
			in    string
			out   *string
		}{{"step workflow", st.WF, &s.WF}, {"step name", st.Step, &s.Step}, {"step text", st.Text, &s.Text}} {
			if err := oneLine(f.field, f.in); err != nil {
				return Entry{}, err
			}
			*f.out = strings.TrimSpace(f.in)
		}
		if s.WF == "" || s.Step == "" {
			return Entry{}, errors.New("environment step needs a workflow and a step")
		}
		out.Steps = append(out.Steps, s)
	}
	return out, nil
}

// stepJSON fixes the key order wf, step, text and leaves text out when empty.
type stepJSON struct {
	WF   string `json:"wf"`
	Step string `json:"step"`
	Text string `json:"text,omitempty"`
}

// Render returns the entry file: front matter naming the id, then the body the
// app reads. It refuses what Save refuses.
func Render(e Entry) ([]byte, error) {
	e, err := clean(e)
	if err != nil {
		return nil, err
	}
	lines := []string{"## Environments", "### " + e.ID}
	for _, kv := range [][2]string{{"name", e.Name}, {"kind", e.Kind}, {"where", e.Where}, {"runs", e.Runs}, {"test", e.Test}, {"how", e.How}, {"from", e.From}} {
		if kv[1] != "" {
			lines = append(lines, kv[0]+": "+kv[1])
		}
	}
	if len(e.Who) > 0 {
		lines = append(lines, "who:")
		for _, w := range e.Who {
			lines = append(lines, "  - "+w)
		}
	}
	if len(e.Secrets) > 0 {
		lines = append(lines, "secrets:")
		for _, s := range e.Secrets {
			state := "missing"
			if s.Set {
				state = "set"
			}
			lines = append(lines, "  - "+s.Name+" ("+state+")")
		}
	}
	if len(e.Steps) > 0 {
		lines = append(lines, "steps:")
		for _, st := range e.Steps {
			var b bytes.Buffer
			enc := json.NewEncoder(&b)
			enc.SetEscapeHTML(false)
			if err := enc.Encode(stepJSON(st)); err != nil {
				return nil, err
			}
			lines = append(lines, "  - "+strings.TrimSuffix(b.String(), "\n"))
		}
	}
	if e.Approval != nil {
		lines = append(lines, "approval: "+map[bool]string{true: "yes", false: "no"}[*e.Approval])
	}
	return brain.EncodeConfig(map[string]string{"name": e.ID}, strings.Join(lines, "\n"))
}

var scalarKeys = map[string]bool{"name": true, "kind": true, "where": true, "runs": true, "test": true, "how": true, "from": true}

// Parse reads what Render wrote, for the entry stored under id. The front
// matter name and the ### line must both be id.
func Parse(id string, data []byte) (Entry, error) {
	var front struct {
		Name string `yaml:"name"`
	}
	body, err := brain.DecodeConfig(data, &front)
	if err != nil {
		return Entry{}, fmt.Errorf("environment entry: %w", err)
	}
	if front.Name != id {
		return Entry{}, errors.New("environment entry front matter name does not match its id")
	}
	var lines []string
	for _, l := range strings.Split(body, "\n") {
		if l = strings.TrimRight(l, "\r"); strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) < 2 || lines[0] != "## Environments" || lines[1] != "### "+id {
		return Entry{}, errors.New("environment entry must start with ## Environments and ### <id>")
	}
	e := Entry{ID: id}
	seen := map[string]bool{}
	list := "" // the list key that "  - " lines extend
	for _, l := range lines[2:] {
		if item, ok := strings.CutPrefix(l, "  - "); ok {
			if err := e.addItem(list, strings.TrimSpace(item)); err != nil {
				return Entry{}, err
			}
			continue
		}
		key, val, ok := strings.Cut(l, ":")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if !ok || seen[key] {
			return Entry{}, errors.New("environment entry has a malformed or repeated line")
		}
		seen[key] = true
		list = ""
		switch {
		case scalarKeys[key] && val != "":
			e.setScalar(key, val)
		case (key == "who" || key == "secrets" || key == "steps") && val == "":
			list = key
		case key == "approval" && (val == "yes" || val == "no"):
			v := val == "yes"
			e.Approval = &v
		default:
			return Entry{}, errors.New("environment entry has an unknown key or an empty value")
		}
	}
	// The same rules as Save: a hand-edited file that holds a secret value or a
	// bad kind is reported, not passed on.
	got, err := clean(e)
	if err != nil {
		return Entry{}, err
	}
	return got, nil
}

func (e *Entry) setScalar(key, val string) {
	switch key {
	case "name":
		e.Name = val
	case "kind":
		e.Kind = val
	case "where":
		e.Where = val
	case "runs":
		e.Runs = val
	case "test":
		e.Test = val
	case "how":
		e.How = val
	case "from":
		e.From = val
	}
}

func (e *Entry) addItem(list, item string) error {
	switch list {
	case "who":
		e.Who = append(e.Who, item)
	case "secrets":
		name, state, ok := strings.Cut(item, " (")
		if !ok || (state != "set)" && state != "missing)") {
			return errors.New("environment secret line must end in (set) or (missing)")
		}
		e.Secrets = append(e.Secrets, Secret{Name: name, Set: state == "set)"})
	case "steps":
		var s stepJSON
		dec := json.NewDecoder(strings.NewReader(item))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&s); err != nil || dec.More() {
			return errors.New("environment step line must be one JSON object")
		}
		e.Steps = append(e.Steps, Step(s))
	default:
		return errors.New("environment entry has a list item outside a list")
	}
	return nil
}

// Save writes the entry as projects/<project>/knowledge/environments/<id>.md in
// one EditConfig call: one stored version and one audit line per change. A
// refused entry writes nothing.
func Save(s brain.Store, e Entry) error {
	return save(s.EditConfig, e)
}

// SaveBy is Save with the author named: the audit line carries origin, as
// brain.Store.EditConfigBy writes it.
func SaveBy(s brain.Store, e Entry, origin brain.Origin) error {
	return save(editBy(s, origin), e)
}

// EditBy reads the entry stored under id, hands it to change and stores what
// change gives back, all inside one EditConfigBy call, so two writers of one id
// take turns and the second one's change sees the first one's result. exists is
// false when no entry is stored (old is then the zero Entry with ID set). An
// error from change, or an entry that cannot be rendered, writes nothing. The
// answer is the stored entry as Parse reads it back.
func EditBy(s brain.Store, id string, origin brain.Origin, change func(old Entry, exists bool) (Entry, error)) (Entry, error) {
	var stored Entry
	err := s.EditConfigBy(level, topic, id, origin, func(old []byte) ([]byte, error) {
		cur := Entry{ID: id}
		if old != nil {
			var err error
			if cur, err = Parse(id, old); err != nil {
				return nil, fmt.Errorf("environment %s: %w", id, err)
			}
		}
		next, err := change(cur, old != nil)
		if err != nil {
			return nil, err
		}
		if next.ID != id {
			return nil, fmt.Errorf("environment %s: change gave the entry id %q", id, next.ID)
		}
		data, err := Render(next)
		if err != nil {
			return nil, err
		}
		if stored, err = Parse(id, data); err != nil {
			return nil, err
		}
		return data, nil
	})
	if err != nil {
		return Entry{}, err
	}
	return stored, nil
}

// Remove deletes the entry. An id with no entry answers an error that wraps
// fs.ErrNotExist; no entry file is written or removed.
func Remove(s brain.Store, id string) error {
	return remove(s.EditConfig, id)
}

// RemoveBy is Remove with the author named: removing an entry writes one
// "dropped" audit line carrying origin. A missing id writes no line.
func RemoveBy(s brain.Store, id string, origin brain.Origin) error {
	return remove(editBy(s, origin), id)
}

type editFunc func(level, topic, name string, edit func([]byte) ([]byte, error)) error

func editBy(s brain.Store, origin brain.Origin) editFunc {
	return func(level, topic, name string, edit func([]byte) ([]byte, error)) error {
		return s.EditConfigBy(level, topic, name, origin, edit)
	}
}

func save(edit editFunc, e Entry) error {
	data, err := Render(e)
	if err != nil {
		return err
	}
	return edit(level, topic, e.ID, func([]byte) ([]byte, error) { return data, nil })
}

func remove(edit editFunc, id string) error {
	if err := validID(id); err != nil {
		return err
	}
	return edit(level, topic, id, func(old []byte) ([]byte, error) {
		if old == nil {
			return nil, fmt.Errorf("environment %s: %w", id, fs.ErrNotExist)
		}
		return nil, nil
	})
}

// vaultDir is the folder of the project's environments below the brain folder:
// projects/<project>/grid/environments.
func vaultDir(project string) string {
	return path.Join("projects", project, "grid", topic)
}

// readEntry reads an environment: from the brain folder's grid/environments when
// the store names the brain folder, falling back to knowledge/environments for
// one the folder does not hold (Save still writes there). A missing entry wraps
// fs.ErrNotExist.
func readEntry(s brain.Store, name string) ([]byte, error) {
	if s.Vault() && brain.ValidName(s.Project) && brain.ValidName(name) && name != "TOPIC" {
		root, err := os.OpenRoot(s.BrainDir)
		if err == nil {
			defer root.Close()
			data, err := safefile.Read(root, path.Join(vaultDir(s.Project), name+".md"))
			if !errors.Is(err, fs.ErrNotExist) {
				return data, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	return s.ReadConfig(level, topic, name)
}

// entryNames lists the environments: those of knowledge/environments and those of
// the brain folder's grid/environments that it does not hold.
func entryNames(s brain.Store) ([]string, error) {
	names, err := s.ConfigNames(level, topic)
	if err != nil || !s.Vault() || !brain.ValidName(s.Project) {
		return names, err
	}
	root, err := os.OpenRoot(s.BrainDir)
	if errors.Is(err, fs.ErrNotExist) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := safefile.NoSymlinks(root, vaultDir(s.Project)); err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(root.FS(), vaultDir(s.Project))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	for _, e := range entries {
		n, ok := strings.CutSuffix(e.Name(), ".md")
		if e.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("config paths cannot be symbolic links")
		}
		if ok && !e.IsDir() && n != "TOPIC" && !seen[n] {
			seen[n], names = true, append(names, n)
		}
	}
	return names, nil
}

// List returns every saved environment in id order. When the store names the
// brain folder (BrainDir), the environments of projects/<project>/grid/environments
// are read too, beside those of knowledge/environments, since Save still writes
// to knowledge/. A home with no knowledge folder gives an empty list. An entry
// that cannot be read stops the list with an error naming it.
func List(s brain.Store) ([]Entry, error) {
	names, err := entryNames(s)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := []Entry{}
	for _, name := range names {
		data, err := readEntry(s, name)
		if err != nil {
			return nil, fmt.Errorf("environment %s: %w", name, err)
		}
		e, err := Parse(name, data)
		if err != nil {
			return nil, fmt.Errorf("environment %s: %w", name, err)
		}
		out = append(out, e)
	}
	return out, nil
}
