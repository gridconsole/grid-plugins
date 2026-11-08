package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// DeleteAllowed is the owner-only default policy. The workflow may replace it
// during initialization, before concurrent writes begin (task_delete_roles).
var DeleteAllowed = func(role string) bool { return role == "owner" }

// RestoreAllowed is the owner-only default policy. The workflow may replace it
// during initialization, before concurrent writes begin (task_restore_roles).
var RestoreAllowed = func(role string) bool { return role == "owner" }

// TrashOptions carries host identity and the caller's person guard, never flags
// claiming a role. PersonalAllowed must be set by a trusted person adapter.
type TrashOptions struct {
	WriteOptions
	Project, Role          string
	Agent, PersonalAllowed bool
}

// TrashResult names the moved folder, without exposing a machine-local path.
type TrashResult struct {
	Project string `json:"project"`
	ID      string `json:"id"`
	Folder  string `json:"folder"`
}

func (o TrashOptions) validate(restore bool) error {
	policy := DeleteAllowed
	if restore {
		policy = RestoreAllowed
	}
	if o.Agent || strings.TrimSpace(o.Who.ID) == "" || !policy(o.Role) {
		return fmt.Errorf("%w: a person with an allowed role must perform this task move", ErrEditRejected)
	}
	if hasEditControls(o.Who.ID) || !idPattern.MatchString(o.ID) || (restore && o.Project == "") {
		return fmt.Errorf("%w: give a task id and, for restore, --project", ErrEditUsage)
	}
	return nil
}

// trashPath checks every existing path component without following symlinks,
// including dangling links and ancestors of directories not created yet.
func trashPath(home, path string) error {
	if home == "" || !filepath.IsAbs(home) {
		return fmt.Errorf("%w: an absolute Grid home is required", ErrEditUsage)
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%w: task move escapes the Grid home", ErrEditUsage)
	}
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return fmt.Errorf("%w: task move requires real directories without symlinks", ErrEditUsage)
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

// withTrashAllocation shares New's allocation lock: restore's id collision
// check and publication must exclude creation and other folder moves.
func (s Store) withTrashAllocation(fn func() error) error {
	home := s.Resolver.GridHome
	for _, path := range []string{home, filepath.Join(home, "projects"), filepath.Join(home, "personal"), filepath.Join(home, "trash")} {
		if err := trashPath(home, path); err != nil {
			return err
		}
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(home, ".task-alloc.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	if err = checkFormat(home); err != nil {
		return err
	}
	// Project discovery must not silently skip a linked project or let the
	// resolver read project.md through one, even when selecting by project id.
	entries, err := os.ReadDir(filepath.Join(home, "projects"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			if err := trashPath(home, filepath.Join(home, "projects", entry.Name())); err != nil {
				return err
			}
		}
	}
	return fn()
}

// withTrashTask uses the existing task lock, whose handle permits Windows
// delete sharing. The lock travels with the folder and remains held on rename.
func withTrashTask(path string, fn func() error) error {
	lock, err := fslock.OpenNoFollow(filepath.Join(path, ".task.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = fslock.LockExclusive(lock); err != nil {
		return err
	}
	defer fslock.Unlock(lock)
	held, err := lock.Stat()
	if err != nil {
		return err
	}
	current, err := os.Lstat(filepath.Join(path, ".task.lock"))
	if err != nil {
		return err
	}
	if !os.SameFile(held, current) {
		return fmt.Errorf("%w: task folder changed while locking", ErrEditRejected)
	}
	return fn()
}

func (s Store) trashProject(o TrashOptions) (string, error) {
	if o.Project == "" {
		return "", nil
	}
	r, err := s.resolve(o.Cwd, o.Project)
	if err != nil {
		return "", err
	}
	if err := trashPath(s.Resolver.GridHome, gridhome.TasksDir(s.Resolver.GridHome, r.Project)); err != nil {
		return "", err
	}
	return r.Project, nil
}

func trashPersonal(o TrashOptions, project string) error {
	if project == gridhome.Personal && !o.PersonalAllowed {
		return fmt.Errorf("%w: Personal tasks require the person guard", ErrEditRejected)
	}
	return nil
}

func trashHistoryReady(path string) error {
	name := filepath.Join(path, "history.md")
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: history.md must be a regular file", ErrEditUsage)
	}
	if info.Mode().Perm()&0o222 == 0 {
		return fmt.Errorf("%w: history.md is not appendable", ErrEditRejected)
	}
	f, err := fslock.OpenNoFollow(name, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("%w: history.md is not appendable: %v", ErrEditRejected, err)
	}
	return f.Close()
}

// trashRecord folds a trashed task under its original folder name, so Fold's
// identity checks still apply; metadata and intent are never read via symlinks.
func trashRecord(path, folder, project string) (Task, error) {
	for _, name := range []string{"task.md", "intent.md"} {
		if err := writableFile(path, name); err != nil {
			return Task{}, fmt.Errorf("%w: %v", ErrEditUsage, err)
		}
	}
	data, err := readOptional(filepath.Join(path, "task.md"))
	if err != nil {
		return Task{}, err
	}
	intent, err := readOptional(filepath.Join(path, "intent.md"))
	if err != nil {
		return Task{}, err
	}
	t, err := Fold(filepath.Join(filepath.Dir(path), folder), string(data), string(intent))
	if err != nil {
		return Task{}, err
	}
	t.Path = path
	if t.Project != project {
		return Task{}, fmt.Errorf("%w: task project does not match its folder", ErrEditRejected)
	}
	return t, nil
}

func trashNoLease(path string) error {
	if err := trashPath(path, filepath.Join(path, "feed", "lease")); err != nil {
		return err
	}
	f, err := fslock.OpenNoFollow(filepath.Join(path, "feed", "lease", "task.json"), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: cannot read task lease: %v", ErrEditRejected, err)
	}
	defer f.Close()
	if err := fslock.LockShared(f); err != nil {
		return err
	}
	defer fslock.Unlock(f)
	data, err := io.ReadAll(f)
	if err == nil {
		err = contract.Validate("lease", data)
	}
	var tenure feed.Lease
	if err == nil {
		err = json.Unmarshal(data, &tenure)
	}
	if err == nil && tenure.Lease != "task" {
		err = errors.New("task lease identity mismatch")
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: cannot establish that the task lease is free: %v", ErrEditRejected, err)
	}
	// Even a stale or remote holder is not permission to break a tenure.
	if tenure.Holder != nil {
		return fmt.Errorf("%w: task has a live task lease; stop its orchestrator or use grid task stop before deleting; this command never breaks leases", ErrEditRejected)
	}
	return nil
}

// Delete moves any non-Planner task, including closed tasks, under its existing
// task lock. Like the Rust writer, it appends history only after a successful
// rename: failed moves leave no false receipt and never copy across filesystems.
func (s Store) Delete(o TrashOptions) (result TrashResult, err error) {
	if err = o.validate(false); err != nil {
		return
	}
	err = s.withTrashAllocation(func() error {
		project, err := s.trashProject(o)
		if err != nil {
			return err
		}
		dirs, err := s.Resolver.TasksDirs()
		if err != nil {
			return err
		}
		var path, proj string
		for _, dir := range dirs {
			if project != "" && filepath.Base(filepath.Dir(dir)) != project {
				continue
			}
			if err := trashPath(s.Resolver.GridHome, dir); err != nil {
				return err
			}
			entries, err := os.ReadDir(dir)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if trashMatches(entry.Name(), o.ID) {
					if path != "" {
						return fmt.Errorf("%w: task id has multiple folders", ErrEditRejected)
					}
					path, proj = filepath.Join(dir, entry.Name()), filepath.Base(filepath.Dir(dir))
				}
			}
		}
		if path == "" {
			return fmt.Errorf("task %s not found in the Grid home", o.ID)
		}
		if err := trashPersonal(o, proj); err != nil {
			return err
		}
		if err := trashPath(s.Resolver.GridHome, path); err != nil {
			return err
		}
		return withTrashTask(path, func() error {
			if err := recoverWrite(path); err != nil {
				return err
			}
			t, err := trashRecord(path, filepath.Base(path), proj)
			if err != nil {
				return err
			}
			doc, err := Document(path)
			if err != nil {
				return err
			}
			if doc.Data["kind"] == "planner" {
				return fmt.Errorf("%w: %s is the project's standing Planner; it cannot be deleted", ErrEditRejected, t.ID)
			}
			if err := trashHistoryReady(path); err != nil {
				return err
			}
			if err := trashNoLease(path); err != nil {
				return err
			}
			folder := filepath.Base(path) + "@" + s.clock().UTC().Format("20060102T1504")
			dest := filepath.Join(s.Resolver.GridHome, "trash", proj, folder)
			if err := trashPath(s.Resolver.GridHome, filepath.Dir(dest)); err != nil {
				return err
			}
			if err := trashVacant(dest); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}
			if err := trashRename(path, dest); err != nil {
				return err
			}
			result = TrashResult{proj, t.ID, folder}
			return s.trashReceipt(t, o, dest, "delete", "deleted")
		})
	})
	return
}

func trashMatches(folder, id string) bool { return folder == id || strings.HasPrefix(folder, id+"-") }

func trashVacant(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: destination must not be a symlink", ErrEditUsage)
		}
		return fmt.Errorf("%w: destination folder already exists (possibly deleted this minute)", ErrEditRejected)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Every cooperating folder publisher holds the allocation lock. The explicit
// destination check also gives identical refusal semantics on Unix and Windows,
// including an existing empty directory, which Unix rename could replace.
func trashRename(from, to string) error {
	if err := trashVacant(to); err != nil {
		return err
	}
	return os.Rename(from, to)
}

// Restore picks the latest valid minute stamp, then folder name to break ties.
// It refuses an occupied id even if the new task uses a different folder slug.
func (s Store) Restore(o TrashOptions) (result TrashResult, err error) {
	if err = o.validate(true); err != nil {
		return
	}
	err = s.withTrashAllocation(func() error {
		project, err := s.trashProject(o)
		if err != nil {
			return err
		}
		if err := trashPersonal(o, project); err != nil {
			return err
		}
		tasks := gridhome.TasksDir(s.Resolver.GridHome, project)
		from := filepath.Join(s.Resolver.GridHome, "trash", project)
		if err := trashPath(s.Resolver.GridHome, from); err != nil {
			return err
		}
		entries, err := os.ReadDir(from)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		type candidate struct{ folder, stamp string }
		var copies []candidate
		for _, entry := range entries {
			cut := strings.LastIndexByte(entry.Name(), '@')
			if cut < 0 {
				continue
			}
			folder, stamp := entry.Name()[:cut], entry.Name()[cut+1:]
			if !trashMatches(folder, o.ID) {
				continue
			}
			if err := trashPath(s.Resolver.GridHome, filepath.Join(from, entry.Name())); err != nil {
				return err
			}
			parsed, err := time.Parse("20060102T1504", stamp)
			if err != nil || parsed.Format("20060102T1504") != stamp {
				return fmt.Errorf("%w: invalid trash minute stamp", ErrEditUsage)
			}
			copies = append(copies, candidate{folder, stamp})
		}
		if len(copies) == 0 {
			return fmt.Errorf("%w: %s has no trashed copy", ErrEditRejected, o.ID)
		}
		sort.Slice(copies, func(i, j int) bool {
			if copies[i].stamp == copies[j].stamp {
				return copies[i].folder < copies[j].folder
			}
			return copies[i].stamp < copies[j].stamp
		})
		newest := copies[len(copies)-1]
		source := filepath.Join(from, newest.folder+"@"+newest.stamp)
		dest := filepath.Join(tasks, newest.folder)
		entries, err = os.ReadDir(tasks)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, entry := range entries {
			if trashMatches(entry.Name(), o.ID) {
				return fmt.Errorf("%w: a task folder with id %s already exists", ErrEditRejected, o.ID)
			}
		}
		return withTrashTask(source, func() error {
			if err := trashHistoryReady(source); err != nil {
				return err
			}
			if err := recoverWrite(source); err != nil {
				return err
			}
			t, err := trashRecord(source, newest.folder, project)
			if err != nil {
				return err
			}
			if err := trashHistoryReady(source); err != nil {
				return err
			}
			if err := trashNoLease(source); err != nil {
				return err
			}
			if err := os.MkdirAll(tasks, 0o755); err != nil {
				return err
			}
			if err := trashRename(source, dest); err != nil {
				return err
			}
			result = TrashResult{project, t.ID, newest.folder}
			return s.trashReceipt(t, o, dest, "restore", "restored")
		})
	})
	return
}

func (s Store) trashReceipt(t Task, o TrashOptions, dest, action, note string) error {
	o.Action, o.Note = action, note
	err := commitWrite(dest, s.event(t, o.WriteOptions, t.State), map[string][]byte{})
	// A completed rename is never rolled back after a receipt or sync failure.
	if err != nil {
		return fmt.Errorf("task was %s but history could not be committed: %w", note, err)
	}
	// Directory syncing is unsupported on Windows in the existing atomic writer;
	// do not weaken that writer's durability/error policy here.
	return errors.Join(syncDir(filepath.Dir(t.Path)), syncDir(filepath.Dir(dest)))
}
