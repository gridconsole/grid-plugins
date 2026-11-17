// Package gridconfig reads and writes ~/.grid/config.json, the one first-run config
// the plugin and the app share: the workspaces this machine knows, the one in use,
// and for each its data folder and member. It reads version 3 and the earlier desktop's
// version 2 and version 1 shapes, writes version 3 only, and keeps every key it does
// not know. It never decides a folder by itself; gridhome resolves the Grid home from it.
package gridconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Version is the config shape this grid writes as "v".
const Version = 3

// File is the config's name in the machine folder.
const File = "config.json"

// MachineDirName is the machine folder's name in the user's home folder. It holds
// config.json and bin/grid; it is also the data folder when no config names another.
const MachineDirName = ".grid"

// FallbackName is the workspace name the v1 rules give when no plain name is found.
const FallbackName = "default"

// v1 keys: the earlier shape kept one workspace in the top level.
var legacyKeys = []string{"workspace", "projectsFolder", "member", "setup", "step"}

// MachineDir is <userHome>/.grid, or "" when userHome is unknown.
func MachineDir(userHome string) string {
	if userHome == "" {
		return ""
	}
	return filepath.Join(userHome, MachineDirName)
}

// Path is <userHome>/.grid/config.json, or "" when userHome is unknown.
func Path(userHome string) string {
	if userHome == "" {
		return ""
	}
	return filepath.Join(MachineDir(userHome), File)
}

// Workspace is one entry of "workspaces".
type Workspace struct {
	Name string
	// Data is the absolute data folder; empty (or relative) means ~/.grid/<name>.
	Data string
	// Brain is the absolute brain folder; empty means the data folder.
	Brain  string
	Member string
	// Folder is the folder that holds the repositories.
	Folder string
	// Setup is new, started, done or skipped.
	Setup string
	// Step is the setup step to open at; nil writes null.
	Step *string
	// extra holds the keys this grid does not know, written back as read.
	extra map[string]json.RawMessage
}

// Config is the whole file.
type Config struct {
	// V is the version the file was read with; Write stamps Version.
	V       int
	Current string
	// Workspaces in the order they were made.
	Workspaces []Workspace
	// Choices are the first-run answers that are not a workspace's (Claude settings,
	// sandbox, test default), kept as raw JSON values.
	Choices map[string]json.RawMessage
	// SetupAt is when setup last saved, RFC 3339.
	SetupAt string
	// Legacy is the v1 top-level workspace when the file has one; Migrate turns it
	// into an entry and Write never writes it back.
	Legacy *Legacy
	// extra holds the top-level keys this grid does not know.
	extra map[string]json.RawMessage
}

// Legacy is the v1 shape's top-level workspace.
type Legacy struct {
	Workspace      string
	ProjectsFolder string
	Member         string
	Setup          string
	Step           *string
}

// Read reads the config at path. A missing file gives found false and no error; a
// file that cannot be read or parsed gives the error, and the caller goes on as if
// there were none. Read never moves or changes the file.
func Read(path string) (cfg Config, found bool, err error) {
	if path == "" {
		return Config{}, false, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, true, err
	}
	cfg, err = Parse(b)
	return cfg, true, err
}

// Parse decodes the file's bytes.
func Parse(b []byte) (Config, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return Config{}, fmt.Errorf("config.json: %w", err)
	}
	if top == nil {
		return Config{}, fmt.Errorf("config.json: not an object")
	}
	c := Config{extra: top}
	c.V = 1
	if raw, ok := take(top, "v"); ok {
		if err := json.Unmarshal(raw, &c.V); err != nil {
			return Config{}, fmt.Errorf("config.json: v: %w", err)
		}
	}
	str(top, "current", &c.Current)
	str(top, "setupAt", &c.SetupAt)
	if raw, ok := take(top, "choices"); ok && !isNull(raw) {
		if err := json.Unmarshal(raw, &c.Choices); err != nil {
			return Config{}, fmt.Errorf("config.json: choices: %w", err)
		}
	}
	if raw, ok := take(top, "workspaces"); ok && !isNull(raw) {
		var list []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &list); err != nil {
			return Config{}, fmt.Errorf("config.json: workspaces: %w", err)
		}
		for _, m := range list {
			if m == nil {
				m = map[string]json.RawMessage{}
			}
			w := Workspace{extra: m}
			str(m, "name", &w.Name)
			str(m, "data", &w.Data)
			str(m, "brain", &w.Brain)
			str(m, "member", &w.Member)
			str(m, "folder", &w.Folder)
			str(m, "setup", &w.Setup)
			w.Step = optStr(m, "step")
			c.Workspaces = append(c.Workspaces, w)
		}
	}
	var old Legacy
	has := false
	for _, k := range legacyKeys {
		if _, ok := top[k]; ok {
			has = true
		}
	}
	str(top, "workspace", &old.Workspace)
	str(top, "projectsFolder", &old.ProjectsFolder)
	str(top, "member", &old.Member)
	str(top, "setup", &old.Setup)
	old.Step = optStr(top, "step")
	if has {
		c.Legacy = &old
	}
	return c, nil
}

// take removes key from m and returns its raw value.
func take(m map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	raw, ok := m[key]
	delete(m, key)
	return raw, ok
}

// str takes a string value; a value of another type is dropped.
func str(m map[string]json.RawMessage, key string, dst *string) {
	if raw, ok := take(m, key); ok {
		_ = json.Unmarshal(raw, dst)
	}
}

func optStr(m map[string]json.RawMessage, key string) *string {
	raw, ok := take(m, key)
	if !ok || isNull(raw) {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return nil
	}
	return &s
}

func isNull(raw json.RawMessage) bool { return string(bytes.TrimSpace(raw)) == "null" }

// Entry is the workspace called name, when name is a plain name.
func (c *Config) Entry(name string) *Workspace {
	if !ValidName(name) {
		return nil
	}
	for i := range c.Workspaces {
		if c.Workspaces[i].Name == name {
			return &c.Workspaces[i]
		}
	}
	return nil
}

// CurrentEntry is the workspace "current" names, or nil.
func (c *Config) CurrentEntry() *Workspace { return c.Entry(c.Current) }

// DataFolder is the entry's data folder: its own data when that is an absolute
// path, else <userHome>/.grid/<name>. It is "" for a name that is not a plain one,
// for the name "machine" and "bin" (files of the machine folder) and without a userHome.
func (w Workspace) DataFolder(userHome string) string {
	if w.Data != "" && filepath.IsAbs(w.Data) {
		return filepath.Clean(w.Data)
	}
	if userHome == "" || !ValidName(w.Name) || reserved(w.Name) {
		return ""
	}
	return filepath.Join(MachineDir(userHome), w.Name)
}

// BrainFolder is the entry's brain folder: its own brain when absolute, else the data folder.
func (w Workspace) BrainFolder(userHome string) string {
	if w.Brain != "" && filepath.IsAbs(w.Brain) {
		return filepath.Clean(w.Brain)
	}
	return w.DataFolder(userHome)
}

// reserved names a name whose folder under ~/.grid is something else.
func reserved(name string) bool {
	switch strings.ToLower(name) {
	case "machine", "bin":
		return true
	}
	return false
}

// Migrate turns the v1 top-level workspace into one entry and makes it current, by the
// earlier desktop's rules: the name is the last part of an absolute "workspace" when
// that is a plain name, else "member", else login (a handle from the login name),
// else FallbackName; the data folder is ~/.grid/<name>, and an absolute "workspace"
// anywhere else stays the entry's data; the folder is "projectsFolder" without a
// trailing "projects", else ~/workspace. A file that already has workspaces only drops
// the old keys. It reports whether it made an entry. Nothing is written.
func (c *Config) Migrate(userHome, login string) bool {
	old := c.Legacy
	if old == nil {
		return false
	}
	c.Legacy = nil
	if len(c.Workspaces) > 0 {
		return false
	}
	folder := ""
	switch repos := old.ProjectsFolder; {
	case repos != "" && filepath.IsAbs(repos) && filepath.Base(repos) == "projects":
		folder = filepath.Dir(repos)
	case repos != "" && filepath.IsAbs(repos):
		folder = filepath.Clean(repos)
	case userHome != "":
		folder = filepath.Join(userHome, "workspace")
	default:
		c.Legacy = old
		return false
	}
	oldData := ""
	if old.Workspace != "" && filepath.IsAbs(old.Workspace) {
		oldData = filepath.Clean(old.Workspace)
	}
	name := FallbackName
	switch {
	case oldData != "" && ValidName(filepath.Base(oldData)):
		name = filepath.Base(oldData)
	case ValidName(old.Member):
		name = old.Member
	case ValidName(login):
		name = login
	}
	w := Workspace{Name: name, Folder: folder, Member: old.Member, Setup: old.Setup, Step: old.Step}
	if w.Setup == "" {
		w.Setup = "new"
	}
	if oldData != "" && (userHome == "" || oldData != filepath.Join(MachineDir(userHome), name)) {
		w.Data = oldData
	}
	c.Workspaces = append(c.Workspaces, w)
	c.Current = name
	return true
}

// HandleFromLogin makes a handle from a login name: lower case, every character
// outside [a-z0-9] a "-", with no leading, trailing or doubled "-"; "" when no plain
// name is left.
func HandleFromLogin(login string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(login) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "-"):
			b.WriteByte('-')
		}
	}
	h := strings.TrimRight(b.String(), "-")
	if !ValidName(h) {
		return ""
	}
	return h
}

// ValidName reports a plain name: [A-Za-z0-9][A-Za-z0-9._-]{0,127}, not ending in a
// dot and not a Windows device name (con, prn, aux, nul, com1-9, lpt1-9, any case,
// with or without an extension).
func ValidName(s string) bool {
	if s == "" || len(s) > 128 || strings.HasSuffix(s, ".") {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		alnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !alnum && (i == 0 || c != '.' && c != '_' && c != '-') {
			return false
		}
	}
	stem, _, _ := strings.Cut(strings.ToLower(s), ".")
	switch stem {
	case "con", "prn", "aux", "nul":
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "com") || strings.HasPrefix(stem, "lpt")) && stem[3] >= '1' && stem[3] <= '9' {
		return false
	}
	return true
}

// Marshal encodes c in the version 3 shape: the known keys, then every unknown key as
// it was read. The v1 top-level keys are not written.
func (c Config) Marshal() ([]byte, error) {
	top := map[string]any{}
	for k, v := range c.extra {
		top[k] = v
	}
	for _, k := range legacyKeys {
		delete(top, k)
	}
	top["v"] = Version
	if c.Current != "" {
		top["current"] = c.Current
	}
	list := make([]map[string]any, 0, len(c.Workspaces))
	for _, w := range c.Workspaces {
		m := map[string]any{}
		for k, v := range w.extra {
			m[k] = v
		}
		m["name"] = w.Name
		set := func(key, v string) {
			if v != "" {
				m[key] = v
			}
		}
		set("data", w.Data)
		set("brain", w.Brain)
		set("member", w.Member)
		set("folder", w.Folder)
		set("setup", w.Setup)
		m["step"] = w.Step
		list = append(list, m)
	}
	top["workspaces"] = list
	if len(c.Choices) > 0 {
		top["choices"] = c.Choices
	}
	if c.SetupAt != "" {
		top["setupAt"] = c.SetupAt
	}
	b, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Write saves c at path in the version 3 shape through a temporary file and a rename.
// A file of a newer version is never rewritten. When the file on disk is an older
// version, it is kept as <path>.v<n> first (once; an existing copy is not replaced).
func Write(path string, c Config) error {
	if c.V > Version {
		return fmt.Errorf("config.json is version %d; this grid writes %d. Update grid", c.V, Version)
	}
	b, err := c.Marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if old, err := os.ReadFile(path); err == nil {
		if prev, perr := Parse(old); perr == nil && prev.V > Version {
			return fmt.Errorf("config.json is version %d; this grid writes %d. Update grid", prev.V, Version)
		} else if perr == nil && prev.V < Version {
			keep := fmt.Sprintf("%s.v%d", path, prev.V)
			if _, err := os.Stat(keep); errors.Is(err, fs.ErrNotExist) {
				if err := os.WriteFile(keep, old, 0o600); err != nil {
					return err
				}
			}
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(name)
		return errors.Join(werr, cerr)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}
