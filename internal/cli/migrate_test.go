package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		{[]string{"--rollback"}, 2, "flag provided but not defined"},
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
	h.write(filepath.Join(h.gridHome, "unknown.json"), "{}")
	for _, flag := range []string{"--dry-run", "--apply"} {
		if code := h.run(h.root, "migrate", flag); code != 1 || !strings.Contains(h.out.String(), "unmapped:") {
			t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out, h.errb)
		}
		if _, err := os.Stat(filepath.Join(h.gridHome, "VERSION")); !os.IsNotExist(err) {
			t.Fatal("blocked plan wrote VERSION")
		}
	}
}
