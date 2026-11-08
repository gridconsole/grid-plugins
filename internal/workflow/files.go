// Package workflow reads workflow template files from the Grid home.
package workflow

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/localdata"
	"github.com/gridconsole/grid-plugins/internal/projects"
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

func readFiles(home, selector string) ([]File, []fileProblem, error) {
	if home == "" {
		return nil, nil, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	home = gridhome.Real(home)
	entries, err := (projects.Store{Resolver: where.Resolver{GridHome: home}}).List()
	if err != nil {
		return nil, nil, err
	}
	if selector != "" {
		matches := []projects.Entry{}
		for _, entry := range entries {
			if entry.Name == selector || entry.ID == selector {
				matches = append(matches, entry)
			}
		}
		if len(matches) != 1 {
			return nil, nil, fmt.Errorf("project %q is missing or ambiguous", selector)
		}
		entries = matches
	}
	files := []File{{Scope: "workspace"}}
	paths := []string{"workflows.md"}
	for _, entry := range entries {
		files = append(files, File{Scope: "project", Project: &entry.Name})
		paths = append(paths, filepath.Join("projects", entry.Name, "workflows.md"))
	}
	problems := []fileProblem{}
	for i, path := range paths {
		text, rev, err := readFile(home, path)
		files[i].Text, files[i].Rev = text, rev
		if err != nil {
			problems = append(problems, fileProblem{index: i, err: err})
		}
	}
	return files, problems, nil
}

func readFile(home, relative string) (string, *string, error) {
	path, err := localdata.Safe(home, relative)
	if err != nil {
		return "", nil, err
	}
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	if !before.Mode().IsRegular() {
		return "", nil, fmt.Errorf("%s is not a regular file", relative)
	}
	// Root confines resolution to the Grid home even if an ancestor is replaced
	// after Safe. Check the opened identity before reading any bytes; a leaf
	// replaced by a symlink or another file must not supply the template text.
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", nil, err
	}
	defer root.Close()
	if beforeWorkflowOpen != nil {
		beforeWorkflowOpen(relative)
	}
	f, err := root.Open(relative)
	if err != nil {
		return "", nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return "", nil, err
	}
	after, err := root.Lstat(relative)
	if err != nil {
		return "", nil, err
	}
	if !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(after, opened) {
		return "", nil, fmt.Errorf("%s changed while opening", relative)
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
		return "", &rev, fmt.Errorf("larger than %d bytes", maxFileBytes)
	}
	if !utf8.Valid(data) {
		return "", &rev, errors.New("not UTF-8")
	}
	return string(data), &rev, nil
}
