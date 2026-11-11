package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func renameNoStage(t *testing.T, dir string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(dir, ".grid-claude-*"))
	if len(left) != 0 {
		t.Fatalf("stage files left: %v", left)
	}
}

// ClaudeSet replaces settings.json through atomicfile.Rename.
func TestRenameClaudeSetReplacesFile(t *testing.T) {
	o, path := setOpts(t)
	put(t, path, reindent(t, userFile), 0o600)
	o.Key, o.Value = "autoCompactWindow", json.RawMessage("250000")
	res, err := ClaudeSet(o)
	must(t, err)
	if !res.Written {
		t.Fatal("expected a write")
	}
	if _, vals := keysAndValues(t, contents(t, path)); vals["autoCompactWindow"] != "250000" {
		t.Fatalf("new value is not in the file: %v", vals)
	}
	renameNoStage(t, filepath.Dir(path))
}

// A non-empty folder at settings.json cannot be replaced on any OS: the error
// surfaces and nothing under the folder changed. (The regular-file check on the
// read side refuses it before the rename is tried.)
func TestRenameClaudeSetOverFolderFails(t *testing.T) {
	o, path := setOpts(t)
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(path, "keep.txt")
	put(t, inner, []byte("keep\n"), 0o600)
	o.Key, o.Value = "autoCompactWindow", json.RawMessage("250000")
	if _, err := ClaudeSet(o); err == nil {
		t.Fatal("setting over a folder succeeded")
	}
	if got := contents(t, inner); string(got) != "keep\n" {
		t.Fatalf("folder content changed: %q", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "settings.json" {
		t.Fatalf("something else changed beside the folder: %v, %v", entries, err)
	}
}

// RunClaudeSetup replaces both settings.json and the setup record through
// atomicfile.Rename; a second run with the block removed rewrites both again.
func TestRenameClaudeSetupReplacesSettingsAndRecord(t *testing.T) {
	f := newClaudeFixture(t)
	f.write(f.path, `{"theme":"dark"}`+"\n")
	f.o.Yes = true
	f.wantCode(0)
	if got := string(f.read(f.path)); !strings.Contains(got, `"theme"`) || !strings.Contains(got, "permissions") {
		t.Fatalf("settings.json after setup:\n%s", got)
	}
	added := f.read(f.record)
	f.o.Remove = true
	f.wantCode(0)
	if _, err := os.Stat(f.record); !os.IsNotExist(err) {
		t.Fatalf("record should be gone after remove: %v", err)
	}
	f.o.Remove = false
	f.wantCode(0)
	if string(f.read(f.record)) != string(added) {
		t.Fatal("record was not rewritten with the same rules")
	}
	renameNoStage(t, filepath.Dir(f.path))
	renameNoStage(t, filepath.Dir(f.record))
}

// A non-empty folder at the setup record path fails the setup with an error and
// leaves what is under it untouched.
func TestRenameClaudeSetupOverFolderFails(t *testing.T) {
	f := newClaudeFixture(t)
	if err := os.MkdirAll(f.record, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(f.record, "keep.txt")
	f.write(inner, "keep\n")
	f.o.Yes = true
	if got := f.run(); got == 0 {
		t.Fatalf("setup over a folder succeeded; stdout=%s", &f.out)
	}
	if got := string(f.read(inner)); got != "keep\n" {
		t.Fatalf("folder content changed: %q", got)
	}
	entries, err := os.ReadDir(f.record)
	if err != nil || len(entries) != 1 {
		t.Fatalf("something else changed under the folder: %v, %v", entries, err)
	}
}

// BudgetApply renames every staged file into place; the agent files and the
// overlay hold the new content afterwards.
func TestRenameBudgetApplyReplacesFiles(t *testing.T) {
	o, agents := budgetFixture(t)
	o.Preset, o.With = "high", []string{"compact"}
	pv, err := BudgetPreviewFor(o)
	must(t, err)
	o.Expect = pv.Digest
	if _, err := BudgetApply(o); err != nil {
		t.Fatal(err)
	}
	if got := string(contents(t, filepath.Join(agents, "grid-builder.md"))); !strings.Contains(got, "model: opus\neffort: high\n") {
		t.Fatalf("agent file did not take the preset:\n%s", got)
	}
	if overlay := string(contents(t, filepath.Join(o.GridHome, "settings.json"))); !strings.Contains(overlay, `"budget": "high"`) {
		t.Fatalf("overlay: %s", overlay)
	}
	if left, _ := filepath.Glob(filepath.Join(agents, ".grid-claude-*")); len(left) != 0 {
		t.Fatalf("stage files left: %v", left)
	}
}

// A non-empty folder at the overlay path fails the apply with an error, and no
// agent file, settings file or snapshot changed.
func TestRenameBudgetApplyOverFolderFails(t *testing.T) {
	o, agents := budgetFixture(t)
	overlay := filepath.Join(o.GridHome, "settings.json")
	if err := os.MkdirAll(overlay, 0o700); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(overlay, "keep.txt")
	put(t, inner, []byte("keep\n"), 0o600)
	before := map[string]string{}
	for _, n := range []string{"grid-planner", "grid-reviewer", "grid-researcher", "grid-builder", "grid-designer"} {
		before[n] = string(contents(t, filepath.Join(agents, n+".md")))
	}
	user := string(contents(t, filepath.Join(o.Home, ".claude", "settings.json")))
	o.Preset, o.With = "high", []string{"compact"}
	if _, err := BudgetApply(o); err == nil {
		t.Fatal("apply over a folder succeeded")
	}
	for n, want := range before {
		if got := string(contents(t, filepath.Join(agents, n+".md"))); got != want {
			t.Fatalf("%s changed after a failed apply", n)
		}
	}
	if got := string(contents(t, filepath.Join(o.Home, ".claude", "settings.json"))); got != user {
		t.Fatal("settings.json changed after a failed apply")
	}
	if got := string(contents(t, inner)); got != "keep\n" {
		t.Fatalf("folder content changed: %q", got)
	}
	if _, err := os.Stat(snapshotPath(o)); err == nil {
		t.Fatal("an undo snapshot was left for an apply that did not happen")
	}
}
