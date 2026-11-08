package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// A migrated home works with grid where: a git project is found by its remote, a
// folder without git by the folder the record names.
func TestMigrateThenWhere(t *testing.T) {
	h := newHarness(t)
	repo := filepath.Join(h.root, "work", "app")
	plain := filepath.Join(h.root, "notes", "ideas")
	h.mkdir(repo, plain)
	testgit.Init(t, repo, "git@example.com:org/app.git", true)
	old := filepath.Join(h.root, "old")
	h.write(filepath.Join(old, "projects", "app", "project.md"), "---\nfolder: "+repo+"\n---\n# App\n")
	h.write(filepath.Join(old, "projects", "ideas", "project.md"), "---\nfolder: "+plain+"\n---\n")
	h.write(filepath.Join(old, "projects", "app", "tasks", "T-01-first", "task.md"), "---\nid: T-01\nproject: app\n---\n")
	h.write(filepath.Join(h.gridHome, "machine"), h.env.Machine+"\n")
	if code := h.run(h.root, "migrate", "--apply", "--from", old); code != 0 {
		t.Fatalf("apply: %d: %s%s", code, h.out, h.errb)
	}
	for cwd, want := range map[string][]string{
		repo:  {"project: app\n", "record: true\n", "keys: remote:example.com/org/app\n"},
		plain: {"project: ideas\n", "record: true\n", "rule: record\n"},
	} {
		if code := h.run(cwd, "where"); code != 0 {
			t.Fatalf("where in %s: %d: %s", cwd, code, h.errb)
		}
		for _, w := range want {
			if !strings.Contains(h.out.String(), w) {
				t.Errorf("where in %s lacks %q:\n%s", cwd, w, h.out)
			}
		}
	}
	if code := h.run(h.root, "doctor"); code != 0 && strings.Contains(h.errb.String(), "migrate") {
		t.Fatalf("doctor asks for a migration: %s", h.errb)
	}
}

func TestMigrateCommand(t *testing.T) {
	h := newHarness(t)
	old := filepath.Join(h.root, "old")
	h.write(filepath.Join(old, "projects", "example", "project.md"), "---\nfolder: /example/work/project\n---\n")
	h.write(filepath.Join(old, "projects", "example", "tasks", "T-01-example", "task.md"), "---\nid: T-01\nproject: example\n---\n")
	for _, flags := range [][]string{{"--from", "old"}, {"--dry-run", "--from", old}} {
		args := append([]string{"migrate"}, flags...)
		if code := h.run(h.root, args...); code != 0 {
			t.Fatalf("dry run: %d: %s", code, h.errb)
		}
		if !strings.Contains(h.out.String(), "identify:") || !strings.Contains(h.out.String(), "0 conflicts, 0 unmapped") {
			t.Fatal(h.out.String())
		}
		if _, err := os.Stat(filepath.Join(h.gridHome, "VERSION")); !os.IsNotExist(err) {
			t.Fatal("dry run wrote VERSION")
		}
	}
	if code := h.run(h.root, "migrate", "--apply", "--from", old); code != 0 {
		t.Fatalf("apply: %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), "migrate: applied\n") {
		t.Fatal(h.out.String())
	}
	// Default source is the Grid home; repeated apply does not need the old source.
	if code := h.run(h.root, "migrate", "--apply"); code != 0 {
		t.Fatalf("repeat: %d: %s", code, h.errb)
	}
	if h.out.String() != "migrate: already at format 1; no changes\n" {
		t.Fatal(h.out.String())
	}
}
func TestMigrateInPlaceDefault(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "legacy", "projects", "example", "project.md"), "---\nfolder: /example/work/project\n---\n")
	if code := h.run(h.root, "migrate"); code != 0 {
		t.Fatalf("preview: %d: %s", code, h.errb)
	}
	if code := h.run(h.root, "migrate", "--apply"); code != 0 {
		t.Fatalf("apply: %d: %s", code, h.errb)
	}
	if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "example", "project.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(h.gridHome, "legacy")); !os.IsNotExist(err) {
		t.Fatal("legacy folder remains")
	}
}
func TestMigrateErrors(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		code    int
		message string
	}{
		{[]string{"--dry-run", "--apply"}, 2, "mutually exclusive"},
		{[]string{"--from"}, 2, "flag needs an argument"},
		{[]string{"extra"}, 2, "unexpected argument"},
		{[]string{"--rollback"}, 1, "no completed migration journal"},
		{[]string{"--rollback", "--apply"}, 2, "cannot be combined"},
		{[]string{"--rollback", "--dry-run"}, 2, "cannot be combined"},
		{[]string{"--rollback", "--from", "old"}, 2, "cannot be combined"},
		{[]string{"--from", "missing"}, 1, "does not exist"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.root, append([]string{"migrate"}, tc.args...)...); code != tc.code || !strings.Contains(h.errb.String(), tc.message) {
				t.Fatalf("exit %d, stderr %s", code, h.errb)
			}
		})
	}
	h := newHarness(t)
	h.env.GridHome = ""
	if code := h.run(h.root, "migrate"); code != 1 || !strings.Contains(h.errb.String(), "set GRID_HOME") {
		t.Fatalf("exit %d, stderr %s", code, h.errb)
	}
}
func TestMigrateBlockedExitCode(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "legacy/projects/example/project.md"), "---\nfolder: /example/work/project\n---\n")
	h.write(filepath.Join(h.gridHome, "legacy/projects/example/tasks/T-01-broken/task.md"), "invalid")
	for _, flag := range []string{"--dry-run", "--apply"} {
		if code := h.run(h.root, "migrate", flag); code != 1 || !strings.Contains(h.out.String(), "unmapped:") {
			t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out, h.errb)
		}
		if _, err := os.Stat(filepath.Join(h.gridHome, "VERSION")); !os.IsNotExist(err) {
			t.Fatal("blocked plan wrote VERSION")
		}
	}
}

func TestMigrateRollbackCommand(t *testing.T) {
	h := newHarness(t)
	file := filepath.Join(h.gridHome, "legacy/projects/example/project.md")
	original := "---\nfolder: /example/work/project\n---\n"
	h.write(file, original)
	if code := h.run(h.root, "migrate", "--apply"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if code := h.run(h.root, "migrate", "--rollback"); code != 0 {
		t.Fatal(h.errb.String())
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != original {
		t.Fatal("old home not restored", err)
	}
	if !strings.Contains(h.out.String(), "restored ") || !strings.Contains(h.out.String(), "format: unversioned") {
		t.Fatal(h.out.String())
	}
}
