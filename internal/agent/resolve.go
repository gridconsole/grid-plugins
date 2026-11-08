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
