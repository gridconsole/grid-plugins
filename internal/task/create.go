package task

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gitx"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
	"go.yaml.in/yaml/v4"
)

// New serializes resolution, project registration and allocation across processes.
// The kernel releases the flock (internal/fslock) on exit, including a crash.
// The lock file stays in place so every writer locks the same inode. Readers see
// a task only after its complete staging folder is renamed into place. Where
// fslock is unsupported, New fails with fslock.ErrUnsupported before it makes a task.
func (s Store) New(o NewOptions) (Task, error) {
	if err := o.Validate(); err != nil {
		return Task{}, err
	}
	if s.Resolver.GridHome == "" {
		return Task{}, errors.New("cannot find the Grid home; set GRID_HOME")
	}
	home := gridhome.Real(s.Resolver.GridHome)
	s.Resolver.GridHome = home
	if err := os.MkdirAll(home, 0o755); err != nil {
		return Task{}, err
	}
	lock, err := fslock.OpenNoFollow(filepath.Join(home, ".task-alloc.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return Task{}, fmt.Errorf("open task allocation lock: %w", err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		return Task{}, fmt.Errorf("lock task allocation: %w", err)
	}
	defer fslock.Unlock(lock)
	if err := checkFormat(home); err != nil {
		return Task{}, err
	}
	// Reload machine identity after taking the lock: another first writer may
	// have registered this same folder while this process was waiting.
	machine, err := os.ReadFile(filepath.Join(home, "machine"))
	if err == nil {
		s.Resolver.Machine = strings.TrimSpace(string(machine))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return Task{}, err
	}
	if s.Resolver.Machine == "" {
		s.Resolver.Machine, err = ulid(time.Now())
		if err != nil {
			return Task{}, err
		}
	}
	r, err := s.resolve(o.Cwd, o.Project)
	if err != nil {
		return Task{}, err
	}
	dir := taskDir(home, r.Project)
	if err := safeDir(home, dir); err != nil {
		return Task{}, err
	}
	id, err := nextID(home)
	if err != nil {
		return Task{}, err
	}
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	if o.Size == "" {
		o.Size = "auto"
	}
	title := strings.TrimSpace(strings.SplitN(strings.TrimSpace(o.Words), "\n", 2)[0])
	if chars := []rune(title); len(chars) > 120 {
		title = string(chars[:120])
	}
	t := Task{Record: Record{ID: id, Title: title, Project: r.Project,
		Size: o.Size, Workflow: o.Workflow, State: "queued", Created: now.UTC().Format(time.RFC3339)},
		Path: filepath.Join(dir, id+"-"+slug(title)), Intent: o.Words}
	front, err := yaml.Dump(t.Record, yaml.WithIndent(2), yaml.WithLineWidth(-1))
	if err != nil {
		return Task{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Task{}, err
	}
	stage, err := os.MkdirTemp(dir, ".new-")
	if err != nil {
		return Task{}, err
	}
	defer os.RemoveAll(stage)
	if err := writeSynced(filepath.Join(stage, "intent.md"), []byte(o.Words)); err != nil {
		return Task{}, err
	}
	if err := writeSynced(filepath.Join(stage, "task.md"), append(append([]byte("---\n"), front...), []byte("---\n")...)); err != nil {
		return Task{}, err
	}
	if err := syncDir(stage); err != nil {
		return Task{}, err
	}
	if err := atomicWrite(filepath.Join(home, "VERSION"), []byte(fmt.Sprintf("%d\n", gridhome.FormatVersion))); err != nil {
		return Task{}, err
	}
	if err := atomicWrite(filepath.Join(home, "machine"), []byte(s.Resolver.Machine+"\n")); err != nil {
		return Task{}, err
	}
	if r.Project != "personal" {
		if err := s.register(r, now); err != nil {
			return Task{}, err
		}
	}
	if err := os.Rename(stage, t.Path); err != nil {
		return Task{}, err
	}
	// Until the tasks folder is synced a power loss can drop the published entry,
	// after the caller was told the id.
	if err := syncDir(dir); err != nil {
		return Task{}, err
	}
	return t, nil
}

func checkFormat(home string) error {
	n, err := gridhome.ReadFormat(home)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if problem := gridhome.FormatProblem(n); problem != "" {
		return errors.New(problem)
	}
	return nil
}

func safeDir(home, dir string) error {
	rel, err := filepath.Rel(home, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || gridhome.Real(dir) != dir {
		return fmt.Errorf("task directory %s must stay inside the Grid home without symlinks", dir)
	}
	return nil
}

// nextID counts all project tasks and Personal tasks, including incomplete task
// folders from older writers. Hidden staging folders are never display ids.
func nextID(home string) (string, error) {
	dirs, err := taskDirs(home)
	if err != nil {
		return "", err
	}
	var maxID uint64
	for _, dir := range dirs {
		paths, err := folders(dir)
		if err != nil {
			return "", err
		}
		for _, path := range paths {
			n, err := number(filepath.Base(path))
			if err != nil {
				return "", err
			}
			maxID = max(maxID, n)
		}
	}
	if maxID == math.MaxUint64 {
		return "", errors.New("task ids exhausted")
	}
	return fmt.Sprintf("T-%02d", maxID+1), nil
}

func slug(title string) string {
	var b strings.Builder
	sep := false
	for _, c := range strings.ToLower(title) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			if sep && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(c)
			sep = false
		} else {
			sep = true
		}
		if b.Len() >= 60 {
			break
		}
	}
	if b.Len() == 0 {
		return "task"
	}
	return strings.TrimRight(b.String()[:min(b.Len(), 60)], "-")
}

func (s Store) register(r where.Result, now time.Time) error {
	records, err := project.Load(s.Resolver.GridHome)
	if err != nil {
		return err
	}
	rec := project.Record{Name: r.Project}
	for _, existing := range records {
		if existing.Name == r.Project {
			rec = existing
			break
		}
	}
	if rec.ID == "" {
		rec.ID, err = ulid(now)
		if err != nil {
			return err
		}
	}
	if r.Folder != "" {
		folder := project.Folder{Machine: s.Resolver.Machine, Path: r.Folder}
		found := false
		for _, f := range rec.Folders {
			found = found || f == folder
		}
		if !found {
			rec.Folders = append(rec.Folders, folder)
		}
	}
	for _, key := range r.Keys {
		addKey(&rec, project.NewKey(s.Resolver.Machine, key))
	}
	repos := append([]string{}, r.Repos...)
	if r.Folder != "" && gitx.IsRepoRoot(r.Folder) {
		repos = append(repos, r.Folder)
	}
	for _, path := range repos {
		rel, err := filepath.Rel(r.Folder, path)
		if err != nil {
			return err
		}
		repo := project.Repo{Path: filepath.ToSlash(rel)}
		for _, raw := range gitx.Remotes(path) {
			remote := project.NormalizeRemote(raw)
			if remote != "" {
				if repo.Remote == "" {
					repo.Remote = remote
				}
				addKey(&rec, project.NewKey(s.Resolver.Machine, "remote:"+remote))
			}
		}
		if common, err := gitx.CommonDir(path); err == nil {
			addKey(&rec, project.NewKey(s.Resolver.Machine, "common:"+common))
		}
		if repo.Remote == "" {
			if root := gitx.RootCommit(path); root != "" {
				addKey(&rec, project.NewKey(s.Resolver.Machine, "root:"+root))
			}
		}
		found := false
		for i, old := range rec.Repos {
			if old.Path == repo.Path {
				rec.Repos[i], found = repo, true
			}
		}
		if !found {
			rec.Repos = append(rec.Repos, repo)
		}
	}
	data, err := project.Format(rec)
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(s.Resolver.GridHome, "projects", rec.Name, "project.md"), data)
}

func addKey(rec *project.Record, key project.Key) {
	for _, old := range rec.Keys {
		if old == key {
			return
		}
	}
	rec.Keys = append(rec.Keys, key)
}

// atomicWrite replaces path with data. The data is synced before the rename and
// the folder after it, so a power loss leaves the old file or the new one, never
// an empty file under the published name.
func atomicWrite(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".write-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := writeAndSync(f, data); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// writeSynced creates path, which must not exist, with data on disk.
func writeSynced(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	return writeAndSync(f, data)
}

// writeAndSync writes data, syncs and closes f.
func writeAndSync(f *os.File, data []byte) error {
	_, err := f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	err = d.Sync()
	if closeErr := d.Close(); err == nil {
		err = closeErr
	}
	return err
}

// ulid makes stable machine and project identities with 80 random bits.
func ulid(now time.Time) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", err
	}
	ms := uint64(now.UnixMilli())
	for i := 5; i >= 0; i-- {
		raw[i] = byte(ms)
		ms >>= 8
	}
	n := new(big.Int).SetBytes(raw[:])
	var out [26]byte
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = alphabet[n.Uint64()&31]
		n.Rsh(n, 5)
	}
	return string(out[:]), nil
}
