// Package machinefolder finds and fills the machine folder: the one folder per
// computer for the launcher's own files, caches, worktrees and videos. It sits at
// the OS standard place unless GRID_MACHINE_DIR or the machine_folder key of the
// one config names another. When the config has shared_group, several logins on
// the computer share the folder: its directories are 2770 and its files 0660.
//
// Resolve is the only place that computes the folder's path. A person's secrets
// (account tokens, credentials, Claude config folders) never go in it.
package machinefolder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
)

// The config keys and the environment override.
const (
	KeyMachineFolder = "machine_folder"
	KeySharedGroup   = "shared_group"
	// EnvDir overrides the folder's path, for tests and one-off runs.
	EnvDir = "GRID_MACHINE_DIR"
)

// Where a Folder's path came from.
const (
	SourceEnv     = "env"
	SourceConfig  = "config"
	SourceDefault = "default"
)

// maxConfig bounds the config file Grid will read.
const maxConfig = 1 << 20

// Env is what the resolver reads. Tests pass their own, so none can reach the
// real config or a real machine folder.
type Env struct {
	Getenv func(string) string // nil reads the process environment
	Home   string              // the login's home folder
	GOOS   string              // "" is runtime.GOOS
}

func (e Env) getenv(key string) string {
	if e.Getenv == nil {
		return os.Getenv(key)
	}
	return e.Getenv(key)
}

func (e Env) goos() string {
	if e.GOOS == "" {
		return runtime.GOOS
	}
	return e.GOOS
}

// DefaultPath is the OS standard place: ~/.local/state/grid on Linux,
// ~/Library/Application Support/Grid on macOS, %LOCALAPPDATA%\Grid on Windows.
// It is "" when the place cannot be known.
func DefaultPath(e Env) string {
	switch e.goos() {
	case "darwin":
		if e.Home == "" {
			return ""
		}
		return filepath.Join(e.Home, "Library", "Application Support", "Grid")
	case "windows":
		if local := e.getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "Grid")
		}
		if e.Home == "" {
			return ""
		}
		return filepath.Join(e.Home, "AppData", "Local", "Grid")
	}
	if e.Home == "" {
		return ""
	}
	return filepath.Join(e.Home, ".local", "state", "grid")
}

// ConfigPath is the one config: ~/.config/grid/config.json, or
// %APPDATA%\Grid\config.json on Windows. It is "" when the place cannot be known.
func ConfigPath(e Env) string {
	if e.goos() == "windows" {
		if roaming := e.getenv("APPDATA"); roaming != "" {
			return filepath.Join(roaming, "Grid", "config.json")
		}
		if e.Home == "" {
			return ""
		}
		return filepath.Join(e.Home, "AppData", "Roaming", "Grid", "config.json")
	}
	if e.Home == "" {
		return ""
	}
	return filepath.Join(e.Home, ".config", "grid", "config.json")
}

// Config holds the two keys this package owns. Every other key of the file
// belongs to someone else and is neither read nor changed here.
type Config struct {
	MachineFolder string
	SharedGroup   string
}

// ReadConfig reads the two keys. A missing file is an empty Config and no error.
// A file that is not a JSON object, or holds one of the two keys as anything but
// text, is an error naming the file and the key, never its content.
func ReadConfig(e Env) (Config, error) {
	path := ConfigPath(e)
	if path == "" {
		return Config{}, nil
	}
	raw, err := readObject(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	for key, dst := range map[string]*string{KeyMachineFolder: &c.MachineFolder, KeySharedGroup: &c.SharedGroup} {
		v, ok := raw[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(v, dst); err != nil {
			return Config{}, fmt.Errorf("%s: %s must be text", path, key)
		}
		*dst = strings.TrimSpace(*dst)
	}
	return c, nil
}

// readObject reads path as one JSON object, keeping each value as it is written.
func readObject(path string) (map[string]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxConfig+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxConfig {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, maxConfig)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil || raw == nil {
		return nil, fmt.Errorf("%s is not a JSON object", path)
	}
	return raw, nil
}

// WriteConfig sets the two keys in the one config and keeps every other key as it
// was. It refuses a file that is not a JSON object, so a person's edit is never
// replaced. The file stays private to the login (0600) and is written by atomic
// rename. It never writes into the machine folder.
func WriteConfig(e Env, c Config) (string, error) {
	path := ConfigPath(e)
	if path == "" {
		return "", errors.New("cannot find the config folder: the login has no home folder")
	}
	raw, err := readObject(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		raw = map[string]json.RawMessage{}
	case err != nil:
		return "", err
	}
	for key, val := range map[string]string{KeyMachineFolder: c.MachineFolder, KeySharedGroup: c.SharedGroup} {
		if val == "" {
			delete(raw, key)
			continue
		}
		b, _ := json.Marshal(val)
		raw[key] = b
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(raw); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	return path, atomicfile.Write(path, out.Bytes(), 0o600)
}

// Folder is the resolved machine folder.
type Folder struct {
	Path        string // "" when it cannot be known
	Source      string // SourceEnv, SourceConfig or SourceDefault
	SharedGroup string // "" is private mode
	// Problem is why a setting was not used (an unreadable config, a relative
	// path); the folder then falls back to the next source. grid doctor shows it.
	Problem string
}

// Shared reports shared mode: several logins use the folder.
func (f Folder) Shared() bool { return f.SharedGroup != "" }

// Configured reports that something other than the OS standard place in private
// mode was asked for, or went wrong: the cases grid doctor has rows for.
func (f Folder) Configured() bool {
	return f.Source != SourceDefault || f.SharedGroup != "" || f.Problem != ""
}

// Resolve is the one function that computes the machine folder: GRID_MACHINE_DIR
// first, then machine_folder of the config, then the OS standard place. A leading
// ~/ is the login's home; a relative path is not used. shared_group comes from the
// config whichever source named the path. It never fails: what it could not use
// is in Problem.
func Resolve(e Env) Folder {
	f := Folder{Source: SourceDefault}
	cfg, err := ReadConfig(e)
	if err != nil {
		f.Problem = err.Error()
	}
	f.SharedGroup = cfg.SharedGroup
	for _, c := range []struct{ source, key, value string }{
		{SourceEnv, EnvDir, e.getenv(EnvDir)},
		{SourceConfig, KeyMachineFolder, cfg.MachineFolder},
	} {
		if strings.TrimSpace(c.value) == "" {
			continue
		}
		p, ok := e.absolute(strings.TrimSpace(c.value))
		if !ok {
			if f.Problem == "" {
				f.Problem = c.key + " must be an absolute path"
			}
			continue
		}
		f.Path, f.Source = p, c.source
		return f
	}
	f.Path = DefaultPath(e)
	return f
}

// absolute expands a leading ~/ and reports whether the result is absolute.
func (e Env) absolute(p string) (string, bool) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if e.Home == "" {
			return "", false
		}
		p = filepath.Join(e.Home, p[1:])
	}
	if !filepath.IsAbs(p) {
		return "", false
	}
	return filepath.Clean(p), true
}
