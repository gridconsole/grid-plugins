package usage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/atomicfile"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// WorkspaceBudgetReadAllowed is the replaceable token_limit_read_roles policy.
// Configure policies before serving concurrent CLI calls.
var WorkspaceBudgetReadAllowed = func(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "owner", "member":
		return true
	default:
		return false
	}
}

// WorkspaceBudgetWriteAllowed is the replaceable token_limit_write_roles policy.
var WorkspaceBudgetWriteAllowed = func(role string) bool {
	switch strings.TrimSpace(role) {
	case "", "owner":
		return true
	default:
		return false
	}
}

// WorkspaceTokens reads the replaceable default tokens_per_task from workspace.md
// using the existing front-matter reader. Missing, linked, non-regular, unreadable,
// oversized, invalid UTF-8 or non-positive/non-digit settings mean no default.
func (s Store) WorkspaceTokens() *uint64 {
	if s.GridHome == "" {
		return nil
	}
	path := filepath.Join(s.GridHome, "workspace.md")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxWorkspaceBytes {
		return nil
	}
	f, err := fslock.OpenNoFollow(path, os.O_RDONLY, 0)
	if err != nil {
		return nil
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, maxWorkspaceBytes+1))
	if err != nil || len(data) > maxWorkspaceBytes || !utf8.Valid(data) {
		return nil
	}
	front := taskfile.ParseFrontmatter(string(data))
	if front == nil {
		return nil
	}
	value, ok := front.Data["tokens_per_task"].(string)
	if !ok {
		return nil
	}
	n, err := ParseTokens(value)
	if err != nil || n == 0 {
		return nil
	}
	return &n
}

// ErrWorkspaceTokensInput identifies an invalid caller-supplied token budget.
var ErrWorkspaceTokensInput = errors.New("workspace token budget must be at least 1")

const maxWorkspaceBytes = 64 * 1024

// SetWorkspaceTokens changes only tokens_per_task lines, or clears them when
// value is nil. The caller authorizes the operation before invoking this method.
// All writers must share the persistent lock, including writers of other keys.
func (s Store) SetWorkspaceTokens(value *uint64) error {
	if value != nil && *value == 0 {
		return fmt.Errorf("%w", ErrWorkspaceTokensInput)
	}
	if s.GridHome == "" {
		return errors.New("cannot find the Grid home; set GRID_HOME")
	}
	root, err := os.OpenRoot(s.GridHome)
	if err != nil {
		return errors.New("cannot open the Grid home")
	}
	defer root.Close()
	lock, err := workspaceLock(root)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return errors.New("cannot lock workspace budget")
	}
	defer fslock.Unlock(lock)
	// Check the persistent lock is still the entry we locked before reading.
	if !workspaceSameFile(root, workspaceLockName, lock) {
		return errors.New("workspace lock changed while locking")
	}
	data, err := readWorkspaceForWrite(root)
	if err != nil {
		return err
	}
	updated, err := editWorkspaceTokens(data, value)
	if err != nil {
		return err
	}
	if bytes.Equal(data, updated) {
		return nil
	}
	if len(updated) > maxWorkspaceBytes {
		return errors.New("workspace budget would exceed the size limit")
	}
	if err := atomicfile.WriteRoot(root, "workspace.md", updated, 0o644); err != nil {
		return errors.New("cannot save workspace budget")
	}
	return nil
}

const workspaceLockName = ".workspace.lock"

func workspaceLock(root *os.Root) (*os.File, error) {
	// Exclusive creation through the root cannot follow a link. Open an existing
	// lock without O_CREATE to avoid Darwin's concurrent root-create ENOENT race.
	for range 20 {
		lock, err := root.OpenFile(workspaceLockName, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if errors.Is(err, os.ErrExist) {
			info, statErr := root.Lstat(workspaceLockName)
			if statErr != nil || !info.Mode().IsRegular() {
				return nil, errors.New("workspace lock is not a regular file")
			}
			lock, err = fslock.OpenNoFollow(filepath.Join(root.Name(), workspaceLockName), os.O_RDWR, 0)
		}
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, errors.New("cannot open workspace lock")
		}
		if !workspaceSameFile(root, workspaceLockName, lock) {
			lock.Close()
			return nil, errors.New("workspace lock changed while opening")
		}
		return lock, nil
	}
	return nil, errors.New("cannot open workspace lock")
}

func workspaceSameFile(root *os.Root, name string, f *os.File) bool {
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return false
	}
	after, err := root.Lstat(name)
	return err == nil && after.Mode().IsRegular() && os.SameFile(opened, after)
}

func readWorkspaceForWrite(root *os.Root) ([]byte, error) {
	info, err := root.Lstat("workspace.md")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxWorkspaceBytes {
		return nil, errors.New("workspace must be a regular file within the size limit")
	}
	f, err := fslock.OpenNoFollow(filepath.Join(root.Name(), "workspace.md"), os.O_RDONLY, 0)
	if err != nil {
		return nil, errors.New("cannot open workspace file")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !workspaceSameFile(root, "workspace.md", f) {
		return nil, errors.New("workspace file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxWorkspaceBytes+1))
	if err != nil || len(data) > maxWorkspaceBytes || !utf8.Valid(data) {
		return nil, errors.New("workspace must be UTF-8 within the size limit")
	}
	return data, nil
}

// Match the existing reader's block boundaries; never serialize its parsed map.
var workspaceFrontmatter = regexp.MustCompile(`^---\r?\n([\s\S]*?\r?\n)---\r?\n?`)
var workspaceEmptyFrontmatter = regexp.MustCompile(`^---\r?\n()---(?:\r?\n|$)`)

func editWorkspaceTokens(data []byte, value *uint64) ([]byte, error) {
	text := string(data)
	bounds := workspaceFrontmatter.FindStringSubmatchIndex(text)
	if bounds == nil {
		bounds = workspaceEmptyFrontmatter.FindStringSubmatchIndex(text)
	}
	if bounds == nil {
		first, _, _ := strings.Cut(text, "\n")
		if strings.TrimSpace(first) == "---" {
			return nil, errors.New("workspace front matter does not parse")
		}
		if value == nil {
			return data, nil
		}
		return []byte(fmt.Sprintf("---\ntokens_per_task: %d\n---\n%s", *value, text)), nil
	}
	// An empty header is valid for editing even though the legacy reader
	// returns nil for it; it has no default token budget.
	if bounds[2] != bounds[3] && taskfile.ParseFrontmatter(text) == nil {
		return nil, errors.New("workspace front matter does not parse")
	}
	end := bounds[3]
	header := text[bounds[2]:end]
	newline := "\n"
	if strings.HasPrefix(text, "---\r\n") {
		newline = "\r\n"
	}
	var out strings.Builder
	out.WriteString(text[:bounds[2]])
	found := false
	for _, line := range strings.SplitAfter(header, "\n") {
		if strings.HasPrefix(line, "tokens_per_task:") {
			if value != nil && !found {
				ending := "\n"
				if strings.HasSuffix(line, "\r\n") {
					ending = "\r\n"
				}
				out.WriteString("tokens_per_task: " + strconv.FormatUint(*value, 10) + ending)
			}
			found = true
		} else {
			out.WriteString(line)
		}
	}
	if value != nil && !found {
		out.WriteString("tokens_per_task: " + strconv.FormatUint(*value, 10) + newline)
	}
	out.WriteString(text[end:])
	return []byte(out.String()), nil
}
