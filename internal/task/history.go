package task

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// FolderAt finds an existing task beneath a canonical root visible to the caller.
// It accepts legacy folders for hook migration without rewriting task.md.
func FolderAt(r where.Resolver, path string) (string, error) {
	r.GridHome = gridhome.Real(r.GridHome)
	roots, err := r.TasksDirs()
	if err != nil {
		return "", err
	}
	path = gridhome.Real(path)
	for _, root := range roots {
		if gridhome.Real(root) != filepath.Clean(root) {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || filepath.IsAbs(rel) {
			continue
		}
		name := strings.Split(rel, string(filepath.Separator))[0]
		if !folderPattern.MatchString(name) {
			continue
		}
		dir := filepath.Join(root, name)
		if gridhome.Real(dir) != dir {
			continue
		}
		info, err := os.Lstat(filepath.Join(dir, "task.md"))
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", errors.New("task metadata must be a regular file")
		}
		f, err := fslock.OpenNoFollow(filepath.Join(dir, "task.md"), os.O_RDONLY, 0)
		if err != nil {
			return "", err
		}
		f.Close()
		return dir, nil
	}
	return "", nil
}

// Document reads task metadata using the existing migration reader.
func Document(dir string) (*taskfile.Frontmatter, error) {
	info, err := os.Lstat(filepath.Join(dir, "task.md"))
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("task metadata must be a regular file")
	}
	f, err := fslock.OpenNoFollow(filepath.Join(dir, "task.md"), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 1<<20 {
		return nil, errors.New("task metadata exceeds hook read limit")
	}
	doc := taskfile.ParseFrontmatter(string(b))
	if doc == nil || doc.Odd {
		return nil, errors.New("invalid task metadata")
	}
	return doc, nil
}

func CleanHistory(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	s = strings.NewReplacer("·", "-", "·", "-").Replace(s)
	r := []rune(s)
	if len(r) > 160 {
		s = string(r[:160])
	}
	return s
}

// AppendHistory serializes append and consecutive duplicate suppression. It
// never follows a symlink or creates a history file for an uninitialized task.
// The line lands by renaming a new file into place under .task.lock, so a hard
// link planted beside history.md keeps the old inode and never aliases it.
func AppendHistory(dir, event, detail, who, via string) error {
	path := filepath.Join(dir, "history.md")
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("history must be a regular file")
	}

	lock, err := fslock.OpenNoFollow(filepath.Join(dir, ".task.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	f, err := fslock.OpenNoFollow(path, os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	b, err := io.ReadAll(f)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	event, detail, who, via = CleanHistory(event), CleanHistory(detail), CleanHistory(who), CleanHistory(via)
	switch strings.ToLower(detail) {
	case "plan accepted", "preview approved", "ship approved":
		detail = fmt.Sprintf("%q", detail)
	}
	line := taskfile.HistoryLine(time.Now().UTC().Format("2006-01-02T15:04"), event, detail, who, via)
	tail := strings.TrimRight(string(b), "\r\n")
	if i := strings.LastIndexByte(tail, '\n'); i >= 0 {
		tail = tail[i+1:]
	}
	_, old, _ := strings.Cut(tail, " · ")
	_, next, _ := strings.Cut(line, " · ")
	if old == next {
		return nil
	}
	prefix := ""
	if len(b) > 0 && b[len(b)-1] != '\n' {
		prefix = "\n"
	}
	return atomicWrite(path, append(b, prefix+line+"\n"...))
}
