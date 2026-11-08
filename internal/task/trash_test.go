package task

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func trashOptions(t Task) TrashOptions {
	return TrashOptions{WriteOptions: WriteOptions{ID: t.ID, Who: contract.Author{ID: "alex", Role: "owner"}}, Project: t.Project, Role: "owner", PersonalAllowed: true}
}

func TestTrashPolicies(t *testing.T) {
	for _, role := range []string{"", "cli", "owner", "member", "guest", "worker"} {
		if DeleteAllowed(role) != (role == "owner") || RestoreAllowed(role) != (role == "owner") {
			t.Fatalf("policy for %q", role)
		}
	}
	for _, action := range []string{"delete", "restore"} {
		t.Run(action, func(t *testing.T) {
			s, task := writeFixture(t)
			o := trashOptions(task)
			oldDelete, oldRestore := DeleteAllowed, RestoreAllowed
			t.Cleanup(func() { DeleteAllowed, RestoreAllowed = oldDelete, oldRestore })
			DeleteAllowed = func(string) bool { return false }
			RestoreAllowed = DeleteAllowed
			var err error
			if action == "delete" {
				_, err = s.Delete(o)
			} else {
				_, err = s.Restore(o)
			}
			if !errors.Is(err, ErrEditRejected) {
				t.Fatal(err)
			}
			if _, err = os.Stat(filepath.Join(task.Path, ".task.lock")); !os.IsNotExist(err) {
				t.Fatalf("policy wrote lock: %v", err)
			}
		})
	}
}

func TestTrashPersonalAndPersonGuards(t *testing.T) {
	for _, which := range []string{"personal", "person", "agent"} {
		t.Run(which, func(t *testing.T) {
			s, task := writeFixture(t)
			o := trashOptions(task)
			switch which {
			case "personal":
				o.PersonalAllowed = false
			case "person":
				o.Who.ID = ""
			case "agent":
				o.Agent = true
			}
			if _, err := s.Delete(o); !errors.Is(err, ErrEditRejected) {
				t.Fatal(err)
			}
			if _, err := s.Restore(o); !errors.Is(err, ErrEditRejected) {
				t.Fatal(err)
			}
			if _, err := os.Stat(task.Path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTrashSameMinuteAndOccupiedID(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	result, err := s.Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	if result.Folder != filepath.Base(task.Path)+"@20260102T0304" {
		t.Fatalf("stamp: %s", result.Folder)
	}
	trash := filepath.Join(s.Resolver.GridHome, "trash", task.Project, result.Folder)
	first, err := os.ReadFile(filepath.Join(trash, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	// Creation can reuse the highest deleted id; restore must not overwrite it.
	replacement, err := s.New(NewOptions{Project: "personal", Words: "Different folder", Who: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	if replacement.ID != task.ID {
		t.Fatalf("expected reused id: %s", replacement.ID)
	}
	if _, err := s.Restore(o); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("occupied: %v", err)
	}
	if err := os.Rename(replacement.Path, task.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(o); !errors.Is(err, ErrEditRejected) {
		t.Fatalf("collision: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(trash, "history.md"))
	if err != nil || string(first) != string(after) {
		t.Fatalf("trash overwritten: %v", err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, "history.md")); !os.IsNotExist(err) {
		t.Fatalf("refused delete wrote history: %v", err)
	}
}

func TestTrashNewestStampBeforeSlug(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	first, err := s.Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Resolver.GridHome, "trash", task.Project)
	old := filepath.Join(dir, task.ID+"-zzz@20260102T0304")
	if err := os.Rename(filepath.Join(dir, first.Folder), old); err != nil {
		t.Fatal(err)
	}
	replacement, err := s.New(NewOptions{Project: "personal", Words: "aaa", Who: "alex"})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(replacement.Path, "marker"), "newest")
	s.Now = func() time.Time { return time.Date(2026, 1, 2, 3, 5, 0, 0, time.UTC) }
	second, err := s.Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.Restore(o)
	if err != nil {
		t.Fatal(err)
	}
	if result.Folder != filepath.Base(replacement.Path) {
		t.Fatalf("selected %s instead of %s", result.Folder, second.Folder)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("older copy lost", err)
	}
	if got, err := os.ReadFile(filepath.Join(replacement.Path, "marker")); err != nil || string(got) != "newest" {
		t.Fatalf("restore marker: %q %v", got, err)
	}
}

func TestTrashSymlinks(t *testing.T) {
	for _, which := range []string{"trash", "trash_project", "copy", "projects", "project", "tasks", "source", "history", "metadata", "lease"} {
		t.Run(which, func(t *testing.T) {
			s, task := writeFixture(t)
			o := trashOptions(task)
			root := s.Resolver.GridHome
			path := filepath.Join(root, "trash")
			restore := which == "copy"
			if restore {
				result, err := s.Delete(o)
				if err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(root, "trash", task.Project, result.Folder)
			}
			switch which {
			case "trash_project":
				path = filepath.Join(root, "trash", "personal")
			case "projects":
				path = filepath.Join(root, "projects")
			case "project":
				path = filepath.Join(root, "projects", "example")
			case "tasks":
				path = filepath.Dir(task.Path)
			case "source":
				path = task.Path
			case "history":
				path = filepath.Join(task.Path, "history.md")
				write(t, path, "keep")
			case "metadata":
				path = filepath.Join(task.Path, "task.md")
			case "lease":
				path = filepath.Join(task.Path, "feed", "lease")
			}
			mkdir(t, filepath.Dir(path))
			target := filepath.Join(root, "linked-target")
			if _, err := os.Lstat(path); err == nil {
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
			} else {
				mkdir(t, target)
			}
			if err := os.Symlink(target, path); err != nil {
				if runtime.GOOS == "windows" {
					t.Skip("symlink creation requires Windows Developer Mode or privilege")
				}
				t.Fatal(err)
			}
			var err error
			if restore {
				_, err = s.Restore(o)
			} else {
				_, err = s.Delete(o)
			}
			if !errors.Is(err, ErrEditUsage) {
				t.Fatalf("symlink error: %v", err)
			}
			if _, err := os.Readlink(path); err != nil {
				t.Fatal("link changed", err)
			}
		})
	}
}

func TestTrashUnappendableHistory(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	path := filepath.Join(task.Path, "history.md")
	write(t, path, "keep\n")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if _, err := s.Delete(o); !errors.Is(err, ErrEditRejected) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Resolver.GridHome, "trash")); !os.IsNotExist(err) {
		t.Fatalf("refusal created trash: %v", err)
	}
}

func TestTrashRenameExistingDirectory(t *testing.T) {
	// Runs unchanged on Windows. Unlike raw Unix rename, our publication must
	// refuse even an empty destination folder and preserve both directories.
	root := t.TempDir()
	from, to := filepath.Join(root, "from"), filepath.Join(root, "to")
	mkdir(t, from)
	mkdir(t, to)
	write(t, filepath.Join(from, "payload"), "keep")
	if err := trashRename(from, to); !errors.Is(err, ErrEditRejected) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(from, "payload")); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(to); err != nil || len(entries) != 0 {
		t.Fatalf("destination changed: %v", err)
	}
}

func TestTrashLockTravelsWithFolder(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	lock, err := fslock.OpenNoFollow(filepath.Join(task.Path, ".task.lock"), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	defer fslock.Unlock(lock)
	done := make(chan error, 1)
	go func() { _, err := s.Delete(o); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("delete bypassed task lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	fslock.Unlock(lock)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("delete stuck")
	}
	held, err := lock.Stat()
	if err != nil {
		t.Fatal(err)
	}
	moved, err := os.Stat(filepath.Join(s.Resolver.GridHome, "trash", "personal", filepath.Base(task.Path)+"@20260102T0304", ".task.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(held, moved) {
		t.Fatal("task lock did not travel with folder")
	}
}

func TestTrashConcurrentRestore(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	if _, err := s.Delete(o); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	done := make(chan error, 2)
	for range 2 {
		go func() { <-start; _, err := s.Restore(o); done <- err }()
	}
	close(start)
	success, rejected := 0, 0
	for range 2 {
		err := <-done
		if err == nil {
			success++
		} else if errors.Is(err, ErrEditRejected) {
			rejected++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || rejected != 1 {
		t.Fatalf("success=%d rejected=%d", success, rejected)
	}
	history, err := os.ReadFile(filepath.Join(task.Path, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(history), `"what":"restore"`) != 1 {
		t.Fatal(string(history))
	}
}

func TestTrashMoveFailureLeavesNoReceipt(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	// Deny creation of the destination to exercise failure before the rename.
	// Root can bypass mode bits, so use a non-directory blocker on every OS.
	write(t, filepath.Join(s.Resolver.GridHome, "trash"), "blocker")
	if _, err := s.Delete(o); err == nil {
		t.Fatal("delete succeeded")
	}
	if _, err := os.Stat(task.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, "history.md")); !os.IsNotExist(err) {
		t.Fatalf("failed move wrote history: %v", err)
	}
	// An actual rename error likewise preserves the source bytes; no copy or
	// delete fallback is allowed when the destination cannot be reached.
	if err := trashRename(task.Path, filepath.Join(s.Resolver.GridHome, "missing", "destination")); err == nil {
		t.Fatal("rename succeeded")
	}
	if _, err := Read(task.Path); err != nil {
		t.Fatal(err)
	}
}

func TestTrashMalformedLeaseAndStamp(t *testing.T) {
	s, task := writeFixture(t)
	o := trashOptions(task)
	leasePath := filepath.Join(task.Path, "feed", "lease", "task.json")
	write(t, leasePath, "{broken")
	if _, err := s.Delete(o); !errors.Is(err, ErrEditRejected) {
		t.Fatal(err)
	}
	if err := os.Remove(leasePath); err != nil {
		t.Fatal(err)
	}
	result, err := s.Delete(o)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(s.Resolver.GridHome, "trash", "personal")
	if err := os.Rename(filepath.Join(dir, result.Folder), filepath.Join(dir, filepath.Base(task.Path)+"@invalid")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Restore(o); !errors.Is(err, ErrEditUsage) {
		t.Fatal(err)
	}
}
