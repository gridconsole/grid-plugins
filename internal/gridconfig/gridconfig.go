// Package gridconfig reads and writes the one first-run config the plugin and the
// app share: the workspaces this machine knows, the one in use, and for each its
// data folder, brain folder and member, plus the machine folder when it is not at
// the OS standard place.
//
// The config is ~/.config/grid/config.json on macOS and Linux and
// %APPDATA%\Grid\config.json on Windows; XDG_CONFIG_HOME is not read, so the app
// and the plugin find the same file from a Finder launch too. The older
// ~/.grid/config.json is read when the new file is missing, and Load copies it once
// to the new path, leaving the old file in place.
//
// The machine folder (locks, caches, worktrees, runner logs, bin/grid) is
// ~/Library/Application Support/Grid on macOS, ~/.local/state/grid on Linux and
// %LOCALAPPDATA%\Grid on Windows (XDG_STATE_HOME is not read), unless the config's
// machineFolder key names another absolute folder (and sharedGroup, when set, makes
// it shared by several logins). machinefolder.Resolve is the one resolver of the
// folder; this package only holds the two keys. ~/.grid stays the default data
// folder and the parent of <name> data folders.
//
// It reads version 3 and the earlier desktop's version 2 and version 1 shapes,
// writes version 3 only, and keeps every key it does not know. It never decides a
// folder by itself; gridhome resolves the Grid home from it.
package gridconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/machinefolder/osplace"
)

// Version is the config shape this grid writes as "v".
const Version = 3

// File is the config's file name.
const File = "config.json"

// DataDirName is the default data folder's name in the user's home folder. It is
// the data folder when no config names another, the parent of ~/.grid/<name> data
// folders, and where the older config and launcher sit.
const DataDirName = ".grid"

// FallbackName is the workspace name the v1 rules give when no plain name is found.
const FallbackName = "default"

// v1 keys: the earlier shape kept one workspace in the top level.
var legacyKeys = []string{"workspace", "projectsFolder", "member", "setup", "step"}

// DataRoot is <userHome>/.grid, or "" when userHome is unknown.
func DataRoot(userHome string) string {
	if userHome == "" {
		return ""
	}
	return filepath.Join(userHome, DataDirName)
}

// OldPath is the older config <userHome>/.grid/config.json, or "" when userHome is
// unknown. It is read only when Path has no file.
func OldPath(userHome string) string {
	if userHome == "" {
		return ""
	}
	return filepath.Join(DataRoot(userHome), File)
}

// Path is the config on this OS; see PathFor.
func Path(getenv func(string) string, userHome string) string {
	return PathFor(runtime.GOOS, getenv, userHome)
}

// PathFor is the config for goos: %APPDATA%\Grid\config.json on Windows (APPDATA
// unset: <userHome>\AppData\Roaming), else <userHome>/.config/grid/config.json.
// It is "" when the folder cannot be known.
func PathFor(goos string, getenv func(string) string, userHome string) string {
	if goos == "windows" {
		base := getenv("APPDATA")
		if base == "" && userHome != "" {
			base = filepath.Join(userHome, "AppData", "Roaming")
		}
		if base == "" {
			return ""
		}
		return filepath.Join(base, "Grid", File)
	}
	if userHome == "" {
		return ""
	}
	return filepath.Join(userHome, ".config", "grid", File)
}

// MachineDirFor is the OS standard machine folder for goos: ~/Library/Application
// Support/Grid on macOS, %LOCALAPPDATA%\Grid on Windows (LOCALAPPDATA unset:
// <userHome>\AppData\Local), else ~/.local/state/grid. It is "" when the folder
// cannot be known. The config's MachineFolder, when set, is used instead.
func MachineDirFor(goos string, getenv func(string) string, userHome string) string {
	if goos == "ios" {
		goos = "darwin"
	}
	// osplace, under machinefolder, owns the OS standard place; no other package spells it.
	return osplace.DefaultPath(goos, getenv, userHome)
}

// Workspace is one entry of "workspaces".
type Workspace struct {
	Name string
	// Data is the absolute data folder; empty (or relative) means ~/.grid/<name>.
	Data string
	// Brain is the absolute brain folder, apart from the data folder that holds
	// tasks, accounts and usage; empty means the brain stays in the data folder.
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
	// MachineFolder is the machineFolder key: an absolute machine folder instead of
	// the OS standard place (on the VM, one folder shared by several logins); "" or a
	// relative path means the OS standard place.
	MachineFolder string
	// SharedGroup is the sharedGroup key: the group that shares the machine folder
	// between logins; "" is private mode.
	SharedGroup string
	// NotText lists the keys of MachineFolder and SharedGroup that the file holds as
	// something other than text. They are read as empty; machinefolder.Resolve turns
	// each into a Problem for grid doctor.
	NotText []string
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

// Loaded is what Load found.
type Loaded struct {
	Config Config
	// Found is false when neither file is there.
	Found bool
	// Path is the file read: the new path, or the old one when the copy could not
	// be made. "" when nothing was found.
	Path string
	// CopiedFrom is the old path when this Load copied it to the new path.
	CopiedFrom string
	// Err says why the file read could not be used; the caller goes on as if there
	// were no config.
	Err error
}

// Load reads the config at path. When path has no file and old has one that
// parses, it copies old's bytes once to path (through a temporary file and a
// rename, the old file left in place) and reads them; when the copy cannot be
// made it reads old in place. A file at path always wins over old.
func Load(path, old string) Loaded {
	cfg, found, err := Read(path)
	if found && err != nil {
		// A path that cannot hold a file (a file where its folder should be) has none.
		if st, serr := os.Stat(path); serr != nil && !errors.Is(serr, fs.ErrPermission) || serr == nil && st.IsDir() {
			cfg, found, err = Config{}, false, nil
		}
	}
	if found || old == "" || old == path {
		l := Loaded{Config: cfg, Found: found, Err: err}
		if found {
			l.Path = path
		}
		return l
	}
	b, err := os.ReadFile(old)
	if errors.Is(err, fs.ErrNotExist) {
		return Loaded{}
	}
	if err != nil {
		return Loaded{Found: true, Path: old, Err: err}
	}
	cfg, err = Parse(b)
	l := Loaded{Config: cfg, Found: true, Path: old, Err: err}
	if err == nil && path != "" && copyOnce(b, path) == nil {
		l.Path, l.CopiedFrom = path, old
	}
	return l
}

// copyOnce writes b at path when path has no file yet, through a temporary file in
// the same folder and a rename, so a reader never sees half a file.
func copyOnce(b []byte, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		return errors.Join(werr, cerr)
	}
	if _, err := os.Lstat(path); err == nil {
		// Another grid made it first, or something else sits there: read old in place.
		return fs.ErrExist
	}
	return os.Rename(name, path)
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
	machineKey(top, osplace.KeyMachineFolder, &c.MachineFolder, &c.NotText)
	machineKey(top, osplace.KeySharedGroup, &c.SharedGroup, &c.NotText)
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

// machineKey takes one of the two machine folder keys: trimmed text, or, for any
// other type, empty with the key noted in notText.
func machineKey(m map[string]json.RawMessage, key string, dst *string, notText *[]string) {
	raw, ok := take(m, key)
	if !ok || isNull(raw) {
		return
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		*dst = ""
		*notText = append(*notText, key)
		return
	}
	*dst = strings.TrimSpace(*dst)
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
	return filepath.Join(DataRoot(userHome), w.Name)
}

// BrainFolder is the entry's own brain folder when "brain" is an absolute path,
// else "": the brain then stays in the data folder, as it does today.
func (w Workspace) BrainFolder() string {
	if w.Brain != "" && filepath.IsAbs(w.Brain) {
		return filepath.Clean(w.Brain)
	}
	return ""
}

// MachineDir is the machine folder the file alone names for goos: the config's
// MachineFolder when it is an absolute path (a leading ~/ is userHome), else
// MachineDirFor. fromConfig says which. It reads no environment
// override and nothing else outside the file, so machinefolder.Resolve, which does, is the one
// resolver callers use; for a config with no env override both give one folder.
func (c *Config) MachineDir(goos string, getenv func(string) string, userHome string) (dir string, fromConfig bool) {
	if p, ok := osplace.Absolute(userHome, c.MachineFolder); ok {
		return p, true
	}
	return MachineDirFor(goos, getenv, userHome), false
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
	if oldData != "" && (userHome == "" || oldData != filepath.Join(DataRoot(userHome), name)) {
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
	if c.MachineFolder != "" {
		top[osplace.KeyMachineFolder] = c.MachineFolder
	}
	if c.SharedGroup != "" {
		top[osplace.KeySharedGroup] = c.SharedGroup
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
