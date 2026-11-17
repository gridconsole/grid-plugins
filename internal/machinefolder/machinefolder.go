// Package machinefolder finds and fills the machine folder: the one folder per
// computer for the launcher's own files, caches, worktrees and videos. It sits at
// the OS standard place unless GRID_MACHINE_DIR or the machineFolder key of the
// one config names another. When the config has sharedGroup, several logins on
// the computer share the folder: its directories are 2770 and its files 0660.
//
// Resolve is the only place that computes the folder's path. The two keys are read
// through gridconfig, the reader every other key of the one config goes through, so
// no second parser of the file exists. A person's secrets (account tokens,
// credentials, Claude config folders) never go in the folder.
package machinefolder

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/machinefolder/osplace"
)

// The config keys and the environment override.
const (
	KeyMachineFolder = osplace.KeyMachineFolder
	KeySharedGroup   = osplace.KeySharedGroup
	// EnvDir overrides the folder's path, for tests and one-off runs.
	EnvDir = "GRID_MACHINE_DIR"
)

// Where a Folder's path came from.
const (
	SourceEnv     = "env"
	SourceConfig  = "config"
	SourceDefault = "default"
)

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
	return osplace.DefaultPath(e.goos(), e.getenv, e.Home)
}

// ConfigPath is the one config: ~/.config/grid/config.json, or
// %APPDATA%\Grid\config.json on Windows. It is "" when the place cannot be known.
func ConfigPath(e Env) string {
	return gridconfig.PathFor(e.goos(), e.getenv, e.Home)
}

// Config holds the two keys this package owns. Every other key of the file
// belongs to someone else and is neither read nor changed here.
type Config struct {
	MachineFolder string
	SharedGroup   string
}

// load reads the one config the way gridhome does: the file at ConfigPath, or, when
// that has none, the older ~/.grid/config.json, copied once to the new place. An
// error names the file and never says what it holds.
func load(e Env) (gridconfig.Config, string, error) {
	path := ConfigPath(e)
	if path == "" {
		return gridconfig.Config{}, "", nil
	}
	l := gridconfig.Load(path, gridconfig.OldPath(e.Home))
	if l.Err != nil {
		var pe *fs.PathError
		if errors.As(l.Err, &pe) {
			return gridconfig.Config{}, path, fmt.Errorf("%s: %v", l.Path, pe.Err)
		}
		return gridconfig.Config{}, path, fmt.Errorf("%s is not a JSON object the Grid config can be read from", l.Path)
	}
	return l.Config, path, nil
}

// ReadConfig reads the two keys. A missing file is an empty Config and no error.
// A file that is not a JSON object, or holds one of the two keys as anything but
// text, is an error naming the file and the key, never its content.
func ReadConfig(e Env) (Config, error) {
	cfg, path, err := load(e)
	if err != nil {
		return Config{}, err
	}
	if len(cfg.NotText) > 0 {
		return Config{}, fmt.Errorf("%s: %s must be text", path, strings.Join(cfg.NotText, " and "))
	}
	return Config{MachineFolder: cfg.MachineFolder, SharedGroup: cfg.SharedGroup}, nil
}

// WriteConfig sets the two keys in the one config and keeps every other key as it
// was; gridconfig writes the file (version 3 shape, a copy of an older version kept,
// private to the login, atomic rename). It refuses a file that is not a JSON object
// and a file still in the version 1 shape, so a person's edit is never replaced and
// nothing is lost. An empty value removes the key. It never writes into the machine
// folder.
func WriteConfig(e Env, c Config) (string, error) {
	cfg, path, err := load(e)
	if path == "" {
		return "", errors.New("cannot find the config folder: the login has no home folder")
	}
	if err != nil {
		return "", err
	}
	if cfg.Legacy != nil {
		return "", fmt.Errorf("%s is still in the version 1 shape; open grid setup once before changing the machine folder", path)
	}
	cfg.MachineFolder, cfg.SharedGroup = c.MachineFolder, c.SharedGroup
	if err := gridconfig.Write(path, cfg); err != nil {
		return "", err
	}
	return path, nil
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
// first, then the machineFolder key of the config, then the OS standard place. A
// leading ~/ is the login's home; a relative path is not used. The sharedGroup key
// comes from the config whichever source named the path. It never fails: what it
// could not use is in Problem.
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
		p, ok := osplace.Absolute(e.Home, strings.TrimSpace(c.value))
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
