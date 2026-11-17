package migrate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/machinefolder"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// These tests are about the old <home>/worktrees/<name> folders when a machine folder
// is configured: migrate puts them in <machine>/worktrees/<project>, outside the home,
// and repairs what points at them. In-place runs (from == home) are used wherever a test
// says the old folder is gone: a separate --from source is left intact by design.

var machineWorkers = []string{"worker-b", "worker-c"}

// machineFixture is the worktree fixture (two real git worktrees with dirty files under
// alpha/worktrees/widget/T-07) and a machine folder beside it.
func machineFixture(t *testing.T) (from, repo, machine string, folder machinefolder.Folder) {
	t.Helper()
	testgit.Isolate(t)
	from, _, repo = worktreeFixture(t)
	machine = filepath.Join(filepath.Dir(from), "machine")
	return from, repo, machine, machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv}
}

func machineChild(machine, worker string) string {
	return filepath.Join(machine, "worktrees", "widget", "T-07", worker)
}

func applyInPlace(t *testing.T, from string, folder machinefolder.Folder) *Plan {
	t.Helper()
	p := scan(t, from, from, WithMachineFolder(folder))
	if len(p.Conflicts) > 0 || len(p.Unmapped) > 0 {
		t.Fatalf("conflicts %v, unmapped %v", p.Conflicts, p.Unmapped)
	}
	if _, err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	return p
}

// Apply: the old worktrees are in the machine folder with their files, and the Grid home
// holds none of them.
func TestMachineFolderApplyMovesTheWorktreesOutOfTheHome(t *testing.T) {
	from, repo, machine, folder := machineFixture(t)
	p := applyInPlace(t, from, folder)
	var outside int
	for _, a := range p.Actions {
		if filepath.IsAbs(a.To) {
			outside++
			if !strings.HasPrefix(a.To, filepath.Join(machine, "worktrees", "widget")+string(filepath.Separator)) {
				t.Errorf("action target outside the worktrees folder of the project: %s", a.To)
			}
		}
	}
	if outside == 0 {
		t.Fatal("the plan has no action with a target in the machine folder")
	}
	for _, w := range machineWorkers {
		dir := machineChild(machine, w)
		if read(t, dir, "untracked.txt") != "keep dirty work\r\n" {
			t.Fatalf("%s: the dirty file did not move", dir)
		}
		if testgit.Run(t, dir, "status", "--porcelain") != "?? untracked.txt" {
			t.Fatalf("%s: worktree data lost", dir)
		}
	}
	absent(t, filepath.Join(from, "projects/widget/worktrees"))
	entries, err := os.ReadDir(filepath.Join(machine, "worktrees"))
	if err != nil || len(entries) != 1 || entries[0].Name() != "widget" {
		t.Fatalf("machine worktrees folder holds %v (%v): only widget is expected, no stage folder", entries, err)
	}
	if listing := testgit.Run(t, repo, "worktree", "list", "--porcelain"); strings.Contains(listing, filepath.Join(from, "projects")) {
		t.Fatal(listing)
	}
}

// Rollback: the old worktrees are back where they were, git names the old paths, and the
// machine folder copy of the project is gone.
func TestMachineFolderRollbackPutsTheWorktreesBack(t *testing.T) {
	from, repo, machine, folder := machineFixture(t)
	applyInPlace(t, from, folder)
	for _, w := range machineWorkers {
		if read(t, machineChild(machine, w), "untracked.txt") != "keep dirty work\r\n" {
			t.Fatalf("%s: apply did not put the worktree in the machine folder", w)
		}
	}
	if _, err := Rollback(from); err != nil {
		t.Fatal(err)
	}
	listing := testgit.Run(t, repo, "worktree", "list", "--porcelain")
	for _, w := range machineWorkers {
		old := filepath.Join(from, "alpha/worktrees/widget/T-07", w)
		if !strings.Contains(listing, "worktree "+old+"\n") {
			t.Fatalf("git does not name the old path %s:\n%s", old, listing)
		}
		if read(t, old, "untracked.txt") != "keep dirty work\r\n" {
			t.Fatalf("%s: dirty file not back", old)
		}
	}
	absent(t, filepath.Join(machine, "worktrees", "widget"))
	if strings.Contains(listing, machine) {
		t.Fatalf("git still names the machine folder:\n%s", listing)
	}
}

// A worktree that was used after the migration is not thrown away by a rollback.
func TestMachineFolderRollbackRefusesAWorktreeWrittenSince(t *testing.T) {
	from, _, machine, folder := machineFixture(t)
	applyInPlace(t, from, folder)
	put(t, filepath.Join(machineChild(machine, "worker-b"), "new-work.txt"), "written after migration")
	if _, err := Rollback(from); err == nil || !strings.Contains(err.Error(), "rollback refused") {
		t.Fatalf("rollback must refuse: %v", err)
	}
	if read(t, machineChild(machine, "worker-b"), "new-work.txt") != "written after migration" {
		t.Fatal("work written after migration was removed")
	}
}

// rewriteWorktrees: the backlink of each worktree in the repository's metadata is set
// to the new path of its .git file, and git names the machine folder after apply.
func TestMachineFolderRewritesTheWorktreeBacklinks(t *testing.T) {
	from, repo, machine, folder := machineFixture(t)
	p := scan(t, from, from, WithMachineFolder(folder))
	if len(p.pointers) != 2 {
		t.Fatalf("pointers: %v; unmapped: %v", p.pointers, p.Unmapped)
	}
	for _, c := range p.pointers {
		if !strings.HasPrefix(c.After, filepath.Join(machine, "worktrees", "widget", "T-07")+string(filepath.Separator)) || !strings.HasSuffix(c.After, "/.git\n") {
			t.Errorf("backlink after = %q, want a .git file in the machine folder", c.After)
		}
	}
	if _, err := p.Apply(); err != nil {
		t.Fatal(err)
	}
	listing := testgit.Run(t, repo, "worktree", "list", "--porcelain")
	for _, w := range machineWorkers {
		if want := "worktree " + machineChild(machine, w) + "\n"; !strings.Contains(listing, want) {
			t.Fatalf("git does not name %s:\n%s", want, listing)
		}
	}
}

// rewriteSessions: the cwd of a session that ran in an old worktree names the new place.
func TestMachineFolderRewritesTheSessionCwd(t *testing.T) {
	from, home := fixture(t)
	machine := filepath.Join(filepath.Dir(from), "machine")
	folder := machinefolder.Folder{Path: machine, Source: machinefolder.SourceEnv}
	p := scan(t, from, home, WithMachineFolder(folder))
	var s struct {
		Cwd string `json:"cwd"`
	}
	data := p.result["projects/widget/tasks/T-07-first/sessions/session-a.json"].Data
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("%v: %q", err, data)
	}
	if want := filepath.Join(machine, "worktrees", "widget", "T-07", "worker-a"); s.Cwd != want {
		t.Fatalf("session cwd = %q, want %q", s.Cwd, want)
	}
}

// A path that is not configured does not move: with no machineFolder in the config and no
// GRID_MACHINE_DIR, the plan with the resolved folder is the plan without one.
func TestUnconfiguredMachineFolderMovesNoWorktree(t *testing.T) {
	from, home := fixture(t)
	folder := machinefolder.Resolve(machinefolder.Env{Home: filepath.Join(filepath.Dir(from), "login"), Getenv: func(string) string { return "" }})
	if folder.Active() || folder.Source != machinefolder.SourceDefault {
		t.Fatalf("nothing is configured, but the folder is %+v", folder)
	}
	with, without := scan(t, from, home, WithMachineFolder(folder)), scan(t, from, home)
	var a, b bytes.Buffer
	if err := with.WriteText(&a); err != nil {
		t.Fatal(err)
	}
	if err := without.WriteText(&b); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Fatalf("an unconfigured machine folder changed the plan:\n%s\nwant\n%s", a.String(), b.String())
	}
	for _, act := range with.Actions {
		if filepath.IsAbs(act.To) {
			t.Errorf("action with a target outside the home: %+v", act)
		}
	}
}
