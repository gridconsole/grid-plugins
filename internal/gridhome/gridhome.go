// Package gridhome finds the Grid data folder and reads its format version.
package gridhome

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FormatVersion is the layout version of the data folder this grid reads and writes.
const FormatVersion = 1

// Dir returns the Grid home: GRID_HOME when set, else <userHome>/.grid.
// It returns "" when neither is known.
func Dir(getenv func(string) string, userHome string) string {
	if d := getenv("GRID_HOME"); d != "" {
		return filepath.Clean(d)
	}
	if userHome == "" {
		return ""
	}
	return filepath.Join(userHome, ".grid")
}

// Real makes path absolute and resolves symlinks in its longest existing prefix, so
// paths compare equal whether or not the folder exists yet.
func Real(path string) string {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}
	rest := ""
	for p := path; ; {
		if real, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// Tilde spells a path under home as ~/....
func Tilde(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}

// Machine reads this computer's id from <dir>/machine, or "" before it is made.
func Machine(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "machine"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// FormatProblem says why this grid cannot use data of format n, or "" when it can.
func FormatProblem(n int) string {
	switch {
	case n > FormatVersion:
		return fmt.Sprintf("This data is format %d; this grid reads up to %d. Update grid.", n, FormatVersion)
	case n < FormatVersion:
		return fmt.Sprintf("This data is format %d; this grid reads %d. Run: grid migrate", n, FormatVersion)
	}
	return ""
}

// ReadFormat reads the format version from <dir>/VERSION.
func ReadFormat(dir string) (int, error) {
	b, err := os.ReadFile(filepath.Join(dir, "VERSION"))
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, fmt.Errorf("VERSION does not hold a number")
	}
	return n, nil
}

// Personal is the project name, and the top-level folder name, of tasks that
// belong to no project.
const Personal = "personal"

// TasksDir is the folder under home that holds a project's task folders:
// personal/tasks for Personal, else projects/<project>/tasks. It is the one
// owner of that layout.
func TasksDir(home, proj string) string {
	if proj == Personal {
		return filepath.Join(home, Personal, "tasks")
	}
	return filepath.Join(home, "projects", proj, "tasks")
}

// TasksDirs lists the task folders of Personal and of every project folder
// under home, whether or not they exist yet.
func TasksDirs(home string) ([]string, error) {
	dirs := []string{TasksDir(home, Personal)}
	entries, err := os.ReadDir(filepath.Join(home, "projects"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, TasksDir(home, e.Name()))
		}
	}
	return dirs, nil
}
