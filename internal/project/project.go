// Package project reads and writes project records: <grid home>/projects/<name>/project.md.
//
// The record is YAML front matter between two "---" lines:
//
//	id: 01J...                        the stable project id (a ULID)
//	folders:                          the project folder, per machine
//	  - machine: <machine id>
//	    path: /abs/path/on/that/machine
//	repos:                            the repos, relative to the project folder
//	  - path: app                     ("." for a one-repo project)
//	    remote: example.com/org/app   the normalised remote, empty with none
//	keys:                             what a repo is matched by
//	  - key: remote:example.com/org/app
//	  - key: root:<commit>
//	  - key: common:/abs/path/.git    the git-common-dir, valid on one machine only
//	    machine: <machine id>
//
// remote: and root: keys hold on every machine; any other key holds only on the
// machine it names. Records are written by the task commands; where only reads them.
package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"go.yaml.in/yaml/v4"
)

// Record is one project's identity.
type Record struct {
	Name    string   `yaml:"-"` // folder name under projects/
	ID      string   `yaml:"id,omitempty"`
	Folders []Folder `yaml:"folders,omitempty"`
	Repos   []Repo   `yaml:"repos,omitempty"`
	Keys    []Key    `yaml:"keys,omitempty"`
}

// Folder is where the project lives on one machine.
type Folder struct {
	Machine string `yaml:"machine"`
	Path    string `yaml:"path"`
}

// Repo is one git repo of the project, by its path below the project folder.
type Repo struct {
	Path   string `yaml:"path"`
	Remote string `yaml:"remote,omitempty"`
}

// Key is one match key; Machine is set for keys that hold on one machine only.
type Key struct {
	Key     string `yaml:"key"`
	Machine string `yaml:"machine,omitempty"`
}

// shared reports keys that name the same repo on every machine.
func shared(key string) bool {
	return strings.HasPrefix(key, "remote:") || strings.HasPrefix(key, "root:")
}

// NewKey makes the stored form of key as seen on machine.
func NewKey(machine, key string) Key {
	if shared(key) {
		return Key{Key: key}
	}
	return Key{Key: key, Machine: machine}
}

// FoldersOn returns the record's folders on machine, symlinks resolved.
func (r Record) FoldersOn(machine string) []string {
	var out []string
	for _, f := range r.Folders {
		if machine != "" && f.Machine == machine && f.Path != "" {
			out = append(out, gridhome.Real(f.Path))
		}
	}
	return out
}

// KeysOn returns the keys that hold on machine: the shared ones and its own.
func (r Record) KeysOn(machine string) []string {
	var out []string
	for _, k := range r.Keys {
		if k.Key != "" && (shared(k.Key) || (machine != "" && k.Machine == machine)) {
			out = append(out, k.Key)
		}
	}
	return out
}

// HasAny reports whether any of keys holds for the record on machine.
func (r Record) HasAny(machine string, keys []string) bool {
	for _, rk := range r.KeysOn(machine) {
		for _, k := range keys {
			if k == rk {
				return true
			}
		}
	}
	return false
}

// Load reads every record under <gridHome>/projects. A missing folder is no records;
// a project folder without project.md is skipped. A record that does not parse is an
// error, because skipping it could make a second project for the same repo.
func Load(gridHome string) ([]Record, error) {
	entries, err := os.ReadDir(filepath.Join(gridHome, "projects"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(gridHome, "projects", e.Name(), "project.md")
		b, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue // a project folder with tasks but no record yet
		}
		if err != nil {
			return nil, err
		}
		rec, err := Parse(e.Name(), string(b))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		recs = append(recs, rec)
	}
	return recs, nil
}

// Parse reads one record's text: front matter, then any body, which it ignores.
func Parse(name, text string) (Record, error) {
	front, ok := frontmatter(text)
	if !ok {
		return Record{}, errors.New("no front matter between --- lines")
	}
	rec := Record{}
	if strings.TrimSpace(front) != "" {
		if err := yaml.Load([]byte(front), &rec); err != nil {
			return Record{}, fmt.Errorf("front matter: %w", err)
		}
	}
	rec.Name = name
	return rec, nil
}

// Format renders a record as project.md text.
func Format(rec Record) ([]byte, error) {
	front, err := yaml.Dump(rec, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	if err != nil {
		return nil, err
	}
	if bytes.Equal(bytes.TrimSpace(front), []byte("{}")) {
		front = nil
	}
	return append(append([]byte("---\n"), front...), "---\n"...), nil
}

// Write writes rec to <gridHome>/projects/<rec.Name>/project.md.
func Write(gridHome string, rec Record) error {
	if rec.Name == "" || rec.Name != filepath.Base(rec.Name) || strings.HasPrefix(rec.Name, ".") {
		return fmt.Errorf("project name %q is not one folder name", rec.Name)
	}
	text, err := Format(rec)
	if err != nil {
		return err
	}
	dir := filepath.Join(gridHome, "projects", rec.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "project.md"), text, 0o644)
}

// frontmatter returns the text between a first "---" line and the next one.
func frontmatter(text string) (string, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return "", false
	}
	for i, l := range lines[1:] {
		if strings.TrimSpace(l) == "---" {
			return strings.Join(lines[1:i+1], "\n") + "\n", true
		}
	}
	return "", false
}

// NormalizeRemote reduces a remote URL to host/path in lower case, so an https and an
// ssh clone of one repo, with or without .git, give the same key. Credentials, ports
// and the scheme are dropped.
func NormalizeRemote(raw string) string {
	s := strings.TrimSpace(raw)
	scheme := false
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		scheme = true
	}
	end := len(s)
	if i := strings.IndexByte(s, '/'); i >= 0 {
		end = i
	}
	auth, rest := s[:end], s[end:]
	if at := strings.LastIndexByte(auth, '@'); at >= 0 {
		auth = auth[at+1:]
	}
	if c := strings.IndexByte(auth, ':'); c >= 0 {
		if !scheme { // scp-like host:path
			rest = "/" + auth[c+1:] + rest
		}
		auth = auth[:c]
	}
	s = strings.ToLower(auth + rest)
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return strings.TrimSuffix(s, "/")
}
