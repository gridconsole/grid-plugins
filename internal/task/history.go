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
// never follows a symlink. A task folder is a folder that holds a regular
// task.md: there the first line creates history.md, because Create writes none
// and a hook line before the first journal write would otherwise be lost. In any
// other folder, where task.md is absent, a link or a directory, nothing is
// written and the answer is nil.
// It refuses a task with a linked protected file and writes nothing, so a
// rename can never replace the linked inode and erase the evidence of the link.
// Otherwise the line lands by renaming a new file into place under .task.lock.
// Callers must not hold a session lock: .task.lock is always taken first.
func AppendHistory(dir, event, detail, who, via string) error {
	return AppendHistoryWhy(dir, event, detail, who, via, "")
}

// AppendHistoryWhy is AppendHistory with a reason: a non-empty why ends the line
// as " · why <text>". The why goes through CleanHistory like every other field, so
// it never adds a separator, and it is part of the repeat suppression: the same
// line with another why is a new line.
func AppendHistoryWhy(dir, event, detail, who, via, why string) error {
	path := filepath.Join(dir, "history.md")
	isTask := func() bool {
		info, err := os.Lstat(filepath.Join(dir, "task.md"))
		return err == nil && info.Mode().IsRegular()
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if !isTask() {
			return nil
		}
	} else if err != nil {
		return err
	} else if !info.Mode().IsRegular() {
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
	// A task writer that died after writing its journal leaves a whole history
	// image behind. Replay it first, under this same lock, so the next task write
	// cannot replace history.md with that older image and drop the line added
	// here. A journal that cannot be replayed is an error and nothing is appended.
	if isTask() {
		if err = recoverWrite(dir); err != nil {
			return err
		}
	}
	// Under the lock, the file is read again: another writer may have created
	// it since the check above. A missing file starts empty in a task folder.
	var b []byte
	f, err := fslock.OpenNoFollow(path, os.O_RDWR, 0)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if !isTask() {
			return nil
		}
	case err != nil:
		return err
	default:
		b, err = io.ReadAll(f)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			return err
		}
	}
	event, detail, who, via, why = CleanHistory(event), CleanHistory(detail), CleanHistory(who), CleanHistory(via), CleanHistory(why)
	switch strings.ToLower(detail) {
	case "plan accepted", "preview approved", "ship approved":
		detail = fmt.Sprintf("%q", detail)
	}
	line := taskfile.HistoryLine(time.Now().UTC().Format("2006-01-02T15:04"), event, detail, who, via, why)
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
