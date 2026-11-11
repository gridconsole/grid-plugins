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

// Tilde spells a path under home as ~/.... A path that is under home only through
// a link (home is a link, path was resolved by Real) is spelled the same way: when
// the literal prefix does not match, it tries again with Real(home). A path under
// neither spelling stays as it is.
func Tilde(path, home string) string {
	if home == "" {
		return path
	}
	if s, ok := tildeUnder(path, home); ok {
		return s
	}
	if real := Real(home); real != home {
		if s, ok := tildeUnder(path, real); ok {
			return s
		}
	}
	return path
}

// tildeUnder spells path as ~ or ~/... when it is home or under home, literally.
func tildeUnder(path, home string) (string, bool) {
	if path == home {
		return "~", true
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):], true
	}
	return "", false
}

// Machine reads this computer's id from <dir>/machine, or "" before it is made
// and while the file is empty or torn. A file holding some other id (an older
// grid's) is returned as it is. It is the one reader of that file.
func Machine(dir string) string {
	s, err := machineFile(dir)
	if err != nil || torn(s) {
		return ""
	}
	return s
}

// ReadMachine is Machine for a writer: a missing, empty or torn file (a writer
// stopped before the id was complete) gives "" and no error, and a file that holds
// anything but a ULID is an error, so a person's edit is never replaced.
func ReadMachine(dir string) (string, error) {
	s, err := machineFile(dir)
	switch {
	case err != nil:
		return "", err
	case ValidID(s):
		return s, nil
	case torn(s):
		return "", nil
	}
	return "", fmt.Errorf("machine file does not contain a ULID")
}

// machineFile returns the trimmed content of <dir>/machine, "" when it is missing.
func machineFile(dir string) (string, error) {
	path := filepath.Join(dir, "machine")
	st, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !st.Mode().IsRegular() || st.Size() > 128 {
		return "", fmt.Errorf("machine is not a regular ULID file")
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.Trim(string(b), " \t\r\n\x00"), nil
}

// torn reports content a writer left before the whole id was down: nothing, or
// a short run of ULID characters.
func torn(s string) bool { return len(s) < ulidLen && ulidChars(s) }

const ulidLen = 26

// ValidID reports whether s is a canonical ULID.
func ValidID(s string) bool { return len(s) == ulidLen && s[0] <= '7' && ulidChars(s) }

func ulidChars(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789ABCDEFGHJKMNPQRSTVWXYZ", c) {
			return false
		}
	}
	return true
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
