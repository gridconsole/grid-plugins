package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

var rollbackSteps = []string{"rollback-locked", "rollback-preparing", "rollback-staged", "rollback-publishing", "rollback-displaced", "rollback-published", "receipt-written", "rollback-previous-removed", "rollback-done"}

func finishRollback(t *testing.T, home string) {
	t.Helper()
	for i := 0; i < 3; i++ {
		if !exists(statePath(home)) && !exists(journalPath(home)) {
			return
		}
		if _, err := Rollback(home); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("rollback did not finish")
}

func TestRollbackCrashMatrix(t *testing.T) {
	for _, layout := range crashLayouts() {
		for _, step := range rollbackSteps {
			t.Run(layout.name+"/"+step, func(t *testing.T) {
				from, home := layout.build(t, gridhome.Real(t.TempDir()))
				before := view(t, home)
				res, err := scan(t, from, home).Apply()
				if err != nil {
					t.Fatal(err)
				}
				backup := view(t, res.Backup)
				t.Setenv("GRID_CRASH_OPERATION", "rollback")
				crashAt(t, step, from, home)
				// Preview recovery is still read-only, including the rename gap.
				state := snapshotState(t, filepath.Dir(home))
				if exists(journalPath(home)) {
					_ = scan(t, from, home)
				}
				if snapshotState(t, filepath.Dir(home)) != state {
					t.Fatal("preview wrote data")
				}
				finishRollback(t, home)
				if !sameView(before, view(t, home)) {
					t.Fatal("old home not restored exactly")
				}
				if !sameView(backup, view(t, res.Backup)) {
					t.Fatal("backup changed")
				}
				absent(t, statePath(home))
				absent(t, journalPath(home))
			})
		}
	}
}

func TestRollbackRefusals(t *testing.T) {
	for _, kind := range []string{"new file", "changed file", "same bytes", "deleted file", "permissions", "backup", "manifest", "missing backup", "migration log", "directory write"} {
		t.Run(kind, func(t *testing.T) {
			from, _ := fixture(t)
			res, err := scan(t, from, from).Apply()
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "new file":
				put(t, filepath.Join(from, "new.txt"), "new")
			case "changed file":
				put(t, filepath.Join(from, "VERSION"), "0\n")
			case "same bytes":
				f := filepath.Join(from, "VERSION")
				info, err := os.Stat(f)
				if err != nil {
					t.Fatal(err)
				}
				put(t, f, read(t, from, "VERSION"))
				stamp := info.ModTime().Add(time.Hour)
				if err := os.Chtimes(f, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			case "deleted file":
				if err := os.Remove(filepath.Join(from, "VERSION")); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(filepath.Join(from, "VERSION"), 0644); err != nil {
					t.Fatal(err)
				}
			case "backup":
				put(t, filepath.Join(res.Backup, "source/alpha/projects/widget/project.md"), "changed")
			case "manifest":
				put(t, filepath.Join(res.Backup, "manifest.json"), "{}")
			case "missing backup":
				if err := os.RemoveAll(res.Backup); err != nil {
					t.Fatal(err)
				}
			case "directory write":
				stamp := time.Now().Add(time.Hour)
				if err := os.Chtimes(from, stamp, stamp); err != nil {
					t.Fatal(err)
				}
			case "migration log":
				put(t, filepath.Join(from, "migration.log"), "{}")
			}
			before := view(t, filepath.Dir(from))
			if _, err := Rollback(from); err == nil {
				t.Fatal("rollback accepted changes")
			}
			if !sameView(before, view(t, filepath.Dir(from))) {
				t.Fatal("refusal changed data")
			}
		})
	}
}

func TestRollbackWorktreeCrash(t *testing.T) {
	testgit.Isolate(t)
	for _, step := range []string{"rollback-displaced", "worktree-rewritten", "recover-renamed"} {
		t.Run(step, func(t *testing.T) {
			from, _, repo := worktreeFixture(t)
			before := view(t, from)
			if _, err := scan(t, from, from).Apply(); err != nil {
				t.Fatal(err)
			}
			t.Setenv("GRID_CRASH_OPERATION", "rollback")
			if step == "recover-renamed" {
				crashAt(t, "rollback-displaced", from, from)
			}
			crashAt(t, step, from, from)
			finishRollback(t, from)
			if !sameView(before, view(t, from)) {
				t.Fatal("worktree bytes not restored")
			}
			listing := testgit.Run(t, repo, "worktree", "list", "--porcelain")
			for _, worker := range []string{"worker-b", "worker-c"} {
				dir := filepath.Join(from, "alpha/worktrees/widget/T-07", worker)
				if !strings.Contains(listing, "worktree "+dir+"\n") {
					t.Fatal(listing)
				}
				if testgit.Run(t, dir, "status", "--porcelain") != "?? untracked.txt" {
					t.Fatal("worktree invalid after rollback")
				}
			}
		})
	}
}

func TestRollbackRefusesWriteDuringRestore(t *testing.T) {
	from, _ := fixture(t)
	if _, err := scan(t, from, from).Apply(); err != nil {
		t.Fatal(err)
	}
	afterStep = func(step string) {
		if step == "rollback-staged" {
			put(t, filepath.Join(from, "new.txt"), "new")
		}
	}
	defer func() { afterStep = nil }()
	if _, err := Rollback(from); err == nil || !strings.Contains(err.Error(), "written after") {
		t.Fatal(err)
	}
	finish, err := Rollback(from)
	if err != nil || !finish.Rerun {
		t.Fatalf("cleanup: %+v %v", finish, err)
	}
	if read(t, from, "new.txt") != "new" {
		t.Fatal("new data lost")
	}
}

func TestRollbackRefusesWriteAfterDisplacement(t *testing.T) {
	from, _ := fixture(t)
	if _, err := scan(t, from, from).Apply(); err != nil {
		t.Fatal(err)
	}
	afterStep = func(step string) {
		if step == "rollback-displaced" {
			previous := globOne(from + ".migrate-previous-*")
			if len(previous) != 1 {
				t.Fatal(previous)
			}
			put(t, filepath.Join(previous[0], "late.txt"), "late write")
		}
	}
	defer func() { afterStep = nil }()
	if _, err := Rollback(from); err == nil || !strings.Contains(err.Error(), "displaced new home was written") {
		t.Fatal(err)
	}
	previous := globOne(from + ".migrate-previous-*")
	if len(previous) != 1 || read(t, previous[0], "late.txt") != "late write" {
		t.Fatal("late write lost")
	}
	if _, err := Rollback(from); err == nil {
		t.Fatal("recovery discarded late write")
	}
}

func TestRollbackAfterPublicationCrash(t *testing.T) {
	from, _ := fixture(t)
	before := view(t, from)
	crashAt(t, "published", from, from)
	if _, err := Rollback(from); err == nil || !strings.Contains(err.Error(), "--apply") {
		t.Fatal(err)
	}
	finishAfterCrash(t, from, from)
	if _, err := Rollback(from); err != nil {
		t.Fatal(err)
	}
	if !sameView(before, view(t, from)) {
		t.Fatal("old home not restored")
	}
}

// A crash while the displaced new home is being deleted leaves a part of it, which a
// rerun must delete too instead of checking it against the digests again.
func TestRollbackFinishesAfterPartialRemovalOfDisplacedHome(t *testing.T) {
	from, _ := fixture(t)
	before := view(t, from)
	if _, err := scan(t, from, from).Apply(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_CRASH_OPERATION", "rollback")
	crashAt(t, "receipt-written", from, from)
	previous := globOne(from + ".migrate-previous-*")
	if len(previous) != 1 {
		t.Fatal(previous)
	}
	var one string
	_ = filepath.WalkDir(previous[0], func(p string, d os.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() && one == "" {
			one = p
		}
		return nil
	})
	if err := os.Remove(one); err != nil {
		t.Fatal(err)
	}
	finishRollback(t, from)
	if !sameView(before, view(t, from)) {
		t.Fatal("old home not restored exactly")
	}
	absent(t, previous[0])
}
