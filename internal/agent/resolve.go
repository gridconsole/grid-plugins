package agent

import (
	"os/exec"
	"path/filepath"
)

// resolveAgent searches only the supplied launch PATH, in directory order.
// Qualifying each candidate lets LookPath apply the platform's executable
// rules (including PATHEXT on Windows) without changing the process PATH.
func resolveAgent(path, name string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			dir = "."
		}
		absolute, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		candidate, err := exec.LookPath(filepath.Join(absolute, name))
		if err != nil {
			continue
		}
		// Keep LookPath's refusal to execute a relative PATH match implicitly.
		if !filepath.IsAbs(dir) {
			return "", exec.ErrDot
		}
		return candidate, nil
	}
	return "", exec.ErrNotFound
}

// resolve preserves Rust's standard-directory order, then searches the launch
// PATH instead of asking a login shell (the plugin never executes a shell).
func (e Env) resolve(name string) (string, error) {
	dirs := []string{}
	if e.Home != "" {
		dirs = append(dirs, filepath.Join(e.Home, ".local", "bin"))
	}
	dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	return resolveIn(dirs, e.PATH, name)
}

func resolveIn(dirs []string, path, name string) (string, error) {
	for _, dir := range dirs {
		if !filepath.IsAbs(dir) {
			continue
		}
		if binary, err := exec.LookPath(filepath.Join(dir, name)); err == nil {
			return binary, nil
		}
	}
	return resolveAgent(path, name)
}
