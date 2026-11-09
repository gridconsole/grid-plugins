// Package workflow reads workflow template files from the Grid home.
package workflow

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// File is a source file. Rev is the first 12 hex characters of its SHA-256,
// including invalid UTF-8 and oversized files; absent files have a nil revision.
// Workspace files have no project.
type File struct {
	Scope   string  `json:"scope"`
	Project *string `json:"project"`
	Rev     *string `json:"rev"`
	Text    string  `json:"text"`
}

// fileProblem ties an I/O failure to its source-file entry.
type fileProblem struct {
	index int
	err   error
}

// beforeWorkflowOpen is a test hook for replacing a leaf after its Lstat.
// Tests that set it run in a separate process so concurrent reads are unaffected.
var beforeWorkflowOpen func(relative string)

var (
	errNotRegular = errors.New("not a regular file")
	errSymlink    = errors.New("symlink refused")
	errChanged    = errors.New("changed while opening")
	errTooLarge   = errors.New("larger than 32768 bytes")
	errNotUTF8    = errors.New("not UTF-8")
)

// fileReason is the only boundary from filesystem errors to public messages.
// Never expose an OS error string, even for an unexpected failure.
func fileReason(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "cannot read"
	case errors.Is(err, fs.ErrNotExist):
		return errChanged.Error()
	case errors.Is(err, ErrLinkedFile):
		return "has more than one hard link (linked_file)"
	}
	for _, reason := range []error{errNotRegular, errSymlink, errChanged, errTooLarge, errNotUTF8} {
		if errors.Is(err, reason) {
			return reason.Error()
		}
	}
	return "cannot read"
}

type workflowProject struct {
	name, id string
	err      error
}

// selectProject applies where's name-before-ID rule to already-visible entries.
// Keep the entry intact so show can still report damaged project records.
func selectProject(entries []workflowProject, selector string) (workflowProject, bool) {
	for _, entry := range entries {
		if entry.name == selector {
			return entry, true
		}
	}
	var found workflowProject
	ok := false
	for _, entry := range entries {
		if entry.id != "" && entry.id == selector {
			if ok {
				return workflowProject{}, false
			}
			found, ok = entry, true
		}
	}
	return found, ok
}

func projectSelectionError(r where.Resolver, selector string) error {
	// Callers with hidden projects must get identical diagnostics for every
	// unavailable selector, whether it is a private name, private ID or unknown.
	if r.Caller.IsGuest() || !r.PersonalOK() {
		return errors.New("project is missing or ambiguous")
	}
	return fmt.Errorf("project %q is missing or ambiguous", writeDiagnostic(selector))
}

// Discover records independently: Store.List deliberately fails closed for
// registry mutations, but one damaged record must not hide other workflows.
func workflowProjects(r where.Resolver) ([]workflowProject, error) {
	home := r.GridHome
	root, err := os.OpenRoot(home)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, errors.New("cannot read projects")
	}
	defer root.Close()
	before, err := sourceInfo(root, "projects")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("projects: %s", fileReason(err))
	}
	// OpenRoot requires a directory, so a concurrent FIFO replacement cannot
	// block discovery either. Read entries through the pinned directory.
	dir, err := root.OpenRoot("projects")
	if err != nil {
		return nil, errors.New("cannot read projects")
	}
	defer dir.Close()
	opened, err := dir.Stat(".")
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("projects: changed while opening")
	}
	f, err := dir.Open(".")
	if err != nil {
		return nil, errors.New("cannot read projects")
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	if err != nil {
		return nil, errors.New("cannot read projects")
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	var out []workflowProject
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		if !r.PersonalProjectAllowed(entry.Name()) || !r.GuestProjectAllowed(entry.Name()) {
			continue
		}
		p := workflowProject{name: entry.Name()}
		file, err := openSource(home, filepath.Join("projects", p.name, "project.md"))
		if err == nil && file == nil {
			continue // An unregistered project folder, as in project.Load.
		}
		if err == nil {
			var data []byte
			data, err = io.ReadAll(file)
			_ = file.Close()
			if err == nil {
				var record project.Record
				record, err = project.Parse(p.name, string(data))
				p.id = record.ID
			}
		}
		p.err = err
		out = append(out, p)
	}
	return out, nil
}

func readFiles(r where.Resolver, selector string) ([]File, []fileProblem, error) {
	home := r.GridHome
	if home == "" {
		return nil, nil, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	home = gridhome.Real(home)
	r.GridHome = home
	entries, err := workflowProjects(r)
	if err != nil {
		return nil, nil, err
	}
	if selector != "" {
		selected, ok := selectProject(entries, selector)
		if !ok {
			return nil, nil, projectSelectionError(r, selector)
		}
		entries = []workflowProject{selected}
	}
	files := []File{{Scope: "workspace"}}
	paths := []string{"workflows.md"}
	for _, entry := range entries {
		files = append(files, File{Scope: "project", Project: &entry.name})
		paths = append(paths, filepath.Join("projects", entry.name, "workflows.md"))
	}
	problems := []fileProblem{}
	for i, path := range paths {
		if i > 0 && entries[i-1].err != nil {
			problems = append(problems, fileProblem{index: i, err: entries[i-1].err})
			continue
		}
		text, rev, err := readFile(home, path)
		files[i].Text, files[i].Rev = text, rev
		if err != nil {
			problems = append(problems, fileProblem{index: i, err: err})
		}
	}
	return files, problems, nil
}

// sourceInfo refuses symlinks in every component without relying on the text of
// localdata.Safe's errors. Root confines every lookup if an ancestor is swapped.
func sourceInfo(root *os.Root, relative string) (os.FileInfo, error) {
	var info os.FileInfo
	var err error
	path := ""
	for _, part := range strings.Split(relative, string(filepath.Separator)) {
		path = filepath.Join(path, part)
		info, err = root.Lstat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errSymlink
		}
	}
	return info, nil
}

// openSource returns nil for an absent file. It checks the opened descriptor's
// type and identity before any read, including after a nonblocking FIFO open.
func openSource(home, relative string) (*os.File, error) {
	root, err := os.OpenRoot(home)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer root.Close()
	before, err := sourceInfo(root, relative)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errNotRegular
	}
	if beforeWorkflowOpen != nil {
		beforeWorkflowOpen(relative)
	}
	f, err := openWorkflowFile(root, relative)
	if err != nil {
		// Sockets cannot be opened as files, and Root may refuse a swapped
		// symlink before returning a descriptor. Classify the replacement
		// without exposing the platform-specific open error.
		if after, statErr := root.Lstat(relative); statErr == nil {
			if after.Mode()&os.ModeSymlink != 0 {
				return nil, errChanged
			}
			if !after.Mode().IsRegular() {
				return nil, errNotRegular
			}
			if !os.SameFile(before, after) {
				return nil, errChanged
			}
		}
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = f.Close()
		}
	}()
	opened, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !opened.Mode().IsRegular() {
		return nil, errNotRegular
	}
	after, err := root.Lstat(relative)
	if err != nil {
		return nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
		return nil, errChanged
	}
	keep = true
	return f, nil
}

func readFile(home, relative string) (string, *string, error) {
	f, err := openSource(home, relative)
	if err != nil || f == nil {
		return "", nil, err
	}
	defer f.Close()
	// A linked workflows.md is unreadable: TaskSteps fails, so no preview can be
	// raised, and every verb whose person gate read it refuses.
	if err := refuseLinked(f, relative); err != nil {
		return "", nil, err
	}
	const maxFileBytes = 32 * 1024
	data, err := io.ReadAll(io.LimitReader(f, maxFileBytes+1))
	if err != nil {
		return "", nil, err
	}
	hash := sha256.New()
	_, _ = hash.Write(data)
	if _, err := io.Copy(hash, f); err != nil {
		return "", nil, err
	}
	rev := fmt.Sprintf("%x", hash.Sum(nil))[:12]
	if len(data) > maxFileBytes {
		return "", &rev, errTooLarge
	}
	if !utf8.Valid(data) {
		return "", &rev, errNotUTF8
	}
	return string(data), &rev, nil
}
