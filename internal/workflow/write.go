package workflow

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/projects"
	"github.com/gridconsole/grid-plugins/internal/safefile"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// ErrChanged is a rejected revision check, not an I/O failure.
var ErrChanged = errors.New("workflow changed since read")

// ErrInvalid identifies invalid input, corresponding to Rust's bad_request or
// too_large. The CLI reports these as usage errors (exit 2).
var ErrInvalid = errors.New("invalid workflow write")

// WriteResult names the file like show does, without revealing a machine path.
type WriteResult struct {
	File string `json:"file"`
	Rev  string `json:"rev"`
}

// WriteDefaultAllowed is the replaceable host policy; applications retain their
// own authorization. Configure it before serving concurrent CLI calls.
var WriteDefaultAllowed = func(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "owner", "member":
		return true
	default:
		return false
	}
}

// ValidWriteRev accepts the explicit absent-file marker or a SHA-256 prefix.
func ValidWriteRev(rev string) bool {
	if rev == "none" {
		return true
	}
	if len(rev) != 12 {
		return false
	}
	for _, ch := range rev {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F') {
			return false
		}
	}
	return true
}

// Write validates one file, then compares and replaces it under a persistent
// per-file lock. The resolver limits project selection to the caller's visibility.
func Write(resolver where.Resolver, selector, expected string, input io.Reader) (WriteResult, error) {
	home := resolver.GridHome
	var result WriteResult
	if !ValidWriteRev(expected) {
		return result, fmt.Errorf("%w: --rev requires 12 hex characters or none", ErrInvalid)
	}
	if home == "" {
		return result, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	home = gridhome.Real(home)
	resolver.GridHome = home
	project := ""
	if selector != "" {
		entries, err := (projects.Store{Resolver: resolver}).List()
		if err != nil {
			if errors.Is(err, localdata.ErrUnsafePath) {
				return result, fmt.Errorf("%w: workflow path is unsafe or inaccessible", ErrInvalid)
			}
			return result, errors.New("cannot read registered projects")
		}
		matches := 0
		for _, entry := range entries {
			if entry.Name == selector || entry.ID == selector {
				project = entry.Name
				matches++
			}
		}
		if matches != 1 {
			return result, errors.New("project is missing or ambiguous")
		}
	}
	result.File = labelOf(project)
	const maxBytes = 32 * 1024
	if input == nil {
		return result, fmt.Errorf("%w: no input", ErrInvalid)
	}
	data, err := io.ReadAll(io.LimitReader(input, maxBytes+1))
	if err != nil {
		return result, errors.New("cannot read workflow input")
	}
	if len(data) > maxBytes {
		return result, fmt.Errorf("%w: file is larger than %d bytes", ErrInvalid, maxBytes)
	}
	if !utf8.Valid(data) {
		return result, fmt.Errorf("%w: file is not UTF-8", ErrInvalid)
	}
	text := string(data)
	// Rust's write validation checks the very first line; Parse permits blank
	// lines before the heading when reading an existing file.
	first, _, _ := strings.Cut(text, "\n")
	first = strings.TrimRightFunc(first, unicode.IsSpace)
	if first != "# Workflows" && (project == "" || first != "# "+project+" workflows") {
		return result, fmt.Errorf("%w: file must start with \"# Workflows\"", ErrInvalid)
	}
	parsed := Parse(text, project)
	if len(parsed.Errors) != 0 {
		messages := make([]string, 0, len(parsed.Errors))
		for _, problem := range parsed.Errors {
			// Section names and messages can contain arbitrary input, including
			// paths. Keep diagnostics useful without echoing absolute paths.
			if problem.Section != nil {
				section := writeDiagnostic(*problem.Section)
				problem.Section = &section
			}
			problem.Message = writeDiagnostic(problem.Message)
			messages = append(messages, problem.String())
		}
		return result, fmt.Errorf("%w:\n%s", ErrInvalid, strings.Join(messages, "\n"))
	}
	// Workspace creation is permitted on an empty home; a selected project's
	// registered folder must already exist.
	if err := os.MkdirAll(home, 0o755); err != nil {
		return result, errors.New("cannot create the Grid home")
	}
	relative := "workflows.md"
	if project != "" {
		relative = filepath.Join("projects", project, relative)
	}
	if _, err := localdata.Safe(home, relative); err != nil {
		return result, fmt.Errorf("%w: workflow path is unsafe or inaccessible", ErrInvalid)
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return result, errors.New("cannot open the Grid home")
	}
	defer root.Close()
	if err := safefile.NoSymlinks(root, relative); err != nil {
		return result, fmt.Errorf("%w: workflow path is unsafe or inaccessible", ErrInvalid)
	}
	dir, err := root.OpenRoot(filepath.Dir(relative))
	if err != nil {
		return result, errors.New("cannot open workflow folder")
	}
	defer dir.Close()
	const lockName = ".workflows.lock"
	if err := safefile.NoSymlinks(dir, lockName); err != nil {
		return result, errors.New("workflow lock is unsafe or inaccessible")
	}
	if st, err := dir.Lstat(lockName); err == nil && !st.Mode().IsRegular() {
		return result, errors.New("workflow lock is not a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("cannot inspect workflow lock")
	}
	lock, err := openLock(dir, lockName)
	if err != nil {
		return result, errors.New("cannot open workflow lock")
	}
	defer lock.Close()
	opened, err := lock.Stat()
	if err != nil {
		return result, errors.New("cannot inspect workflow lock")
	}
	after, err := dir.Lstat(lockName)
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(opened, after) {
		return result, errors.New("workflow lock changed while opening")
	}
	if err := fslock.LockExclusive(lock); err != nil {
		return result, err
	}
	defer fslock.Unlock(lock)
	current := "none"
	before, err := dir.Lstat("workflows.md")
	if err == nil {
		if !before.Mode().IsRegular() {
			return result, fmt.Errorf("%w: workflow target is not a regular file", ErrInvalid)
		}
		f, err := dir.Open("workflows.md")
		if err != nil {
			return result, errors.New("cannot open workflow file")
		}
		defer f.Close()
		opened, err := f.Stat()
		if err != nil {
			return result, errors.New("cannot inspect workflow file")
		}
		after, err := dir.Lstat("workflows.md")
		if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
			return result, errors.New("workflow file changed while opening")
		}
		// Hash all current bytes, even if the old file exceeds the input limit
		// or has invalid UTF-8, so a known revision can repair such a file.
		hash := sha256.New()
		if _, err := io.Copy(hash, f); err != nil {
			return result, errors.New("cannot read workflow file")
		}
		current = fmt.Sprintf("%x", hash.Sum(nil))[:12]
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("cannot inspect workflow file")
	}
	if current != strings.ToLower(expected) {
		return result, ErrChanged
	}
	if err := atomicfile.WriteRoot(dir, "workflows.md", data, 0o644); err != nil {
		return result, errors.New("cannot save workflow file")
	}
	result.Rev = fmt.Sprintf("%x", sha256.Sum256(data))[:12]
	return result, nil
}

// openLock separates creation from opening an existing lock: concurrent
// O_CREATE opens through os.Root can spuriously return ENOENT on Darwin.
func openLock(dir *os.Root, name string) (*os.File, error) {
	var err error
	for range 20 {
		var lock *os.File
		lock, err = dir.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if errors.Is(err, os.ErrExist) {
			lock, err = dir.OpenFile(name, os.O_RDWR, 0)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return lock, err
		}
	}
	return nil, err
}

// Parser errors can quote user-provided values. Redact path-like words, even on
// a different host OS, instead of leaking an input or destination path.
var writePath = regexp.MustCompile(`(?:[A-Za-z]:[\\/]|/|\\\\)[^\s"'<>|,;]*`)

func writeDiagnostic(message string) string {
	return writePath.ReplaceAllString(message, "[path]")
}
