// Package osplace holds what both the config reader (gridconfig) and the machine
// folder resolver (machinefolder) must spell the same way: the two config keys that
// name the machine folder, the OS standard place, and what counts as an absolute
// path. It imports nothing of Grid, so the config package can use it without a cycle.
// The folder itself is still resolved in one place only: machinefolder.Resolve.
package osplace

import (
	"path/filepath"
	"strings"
)

// The config keys. They are camelCase like every other key of the one config.
const (
	KeyMachineFolder = "machineFolder"
	KeySharedGroup   = "sharedGroup"
)

// DefaultPath is the OS standard place: ~/.local/state/grid on Linux,
// ~/Library/Application Support/Grid on macOS, %LOCALAPPDATA%\Grid on Windows. It is
// "" when the place cannot be known.
func DefaultPath(goos string, getenv func(string) string, home string) string {
	switch goos {
	case "darwin", "ios":
		if home == "" {
			return ""
		}
		return filepath.Join(home, "Library", "Application Support", "Grid")
	case "windows":
		if local := getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "Grid")
		}
		if home == "" {
			return ""
		}
		return filepath.Join(home, "AppData", "Local", "Grid")
	}
	if home == "" {
		return ""
	}
	return filepath.Join(home, ".local", "state", "grid")
}

// Absolute expands a leading ~/ to home and reports whether the result is an
// absolute path; it returns the cleaned path.
func Absolute(home, p string) (string, bool) {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home == "" {
			return "", false
		}
		p = filepath.Join(home, p[1:])
	}
	if !filepath.IsAbs(p) {
		return "", false
	}
	return filepath.Clean(p), true
}
