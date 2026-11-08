package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BatchExtensions is the replaceable refusal policy for workflow key
// agent_batch_extensions. Configure it before running agent checks.
var BatchExtensions = []string{".bat", ".cmd"}

// isBatchLauncher is platform independent so every check refuses launchers
// that Windows would run through cmd.exe instead of as a direct child.
func isBatchLauncher(binary string) bool {
	// Recognize either path separator regardless of the host platform, without
	// cleaning away a final dot component before applying Windows trimming.
	base := binary[strings.LastIndexAny(binary, `/\`)+1:]
	base = strings.TrimRight(base, ". ")
	if base == "" {
		return true
	}
	ext := filepath.Ext(base)
	for _, refused := range BatchExtensions {
		if strings.EqualFold(ext, refused) {
			return true
		}
	}
	return false
}

// isResolvedBatchLauncher also checks the final target of a symlink. A link
// whose target cannot be resolved must not reach the version or auth probes.
func isResolvedBatchLauncher(binary string) bool {
	if isBatchLauncher(binary) {
		return true
	}
	info, err := os.Lstat(binary)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		target, err := filepath.EvalSymlinks(binary)
		return err != nil || isBatchLauncher(target)
	}
	return false
}

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
