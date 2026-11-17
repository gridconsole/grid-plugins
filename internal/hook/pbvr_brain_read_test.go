package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// pbVrHookVault is a task of project "demo" whose Grid config names a brain
// folder: an Obsidian vault with notes at any depth.
func pbVrHookVault(t *testing.T) (*eventFixture, string) {
	t.Helper()
	f := eventSetup(t)
	if err := project.Write(f.home, project.Record{Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	made, err := (task.Store{Resolver: f.o.Resolver}).New(task.NewOptions{Cwd: filepath.Dir(f.home), Words: "Vault notes", Project: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(made.Path, "history.md"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	f.dir, f.o.Cwd = made.Path, made.Path
	f.env["GRID_TASK"] = made.Path
	vault := filepath.Join(filepath.Dir(f.home), "vault")
	for rel, text := range map[string]string{
		"Ideas/Café plan.md":     "# Plan\n\nPlan the café.\n",
		"Plain.md":               "No front matter.\n",
		"projects/demo/Proj.md":  "A project note.\n",
		"projects/other/Else.md": "Another project.\n",
		".obsidian/Hidden.md":    "hidden\n",
		"Ideas/readme.txt":       "text\n",
	} {
		p := filepath.Join(vault, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: f.home, Brain: vault, Member: "Sam"}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, f.o.Getenv, f.o.Resolver.Home), cfg); err != nil {
		t.Fatal(err)
	}
	return f, vault
}

func pbVrRead(t *testing.T, f *eventFixture, path string) string {
	t.Helper()
	before := historyOf(t, f)
	out, log, code := f.run(t, "PostToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": path}})
	if out != "" || log != "" || code != 0 {
		t.Fatalf("read %s: %q %q %d", path, out, log, code)
	}
	return strings.TrimPrefix(historyOf(t, f), before)
}

// A Read of Ideas/Café plan.md in the vault logs a brain read of its path id.
func TestPbVrReadOfVaultNoteLogsBrainRead(t *testing.T) {
	f, vault := pbVrHookVault(t)
	added := pbVrRead(t, f, filepath.Join(vault, "Ideas", "Café plan.md"))
	if !strings.Contains(added, "· brain read · brain:home/Ideas/Café plan · by claude · via session session-demo") {
		t.Fatalf("history after the Read: %q", added)
	}
	// The same note again in the same session is not logged twice.
	if again := pbVrRead(t, f, filepath.Join(vault, "Ideas", "Café plan.md")); again != "" {
		t.Fatalf("a repeat Read added %q", again)
	}
	if added := pbVrRead(t, f, filepath.Join(vault, "Plain.md")); !strings.Contains(added, "brain read · brain:home/Plain ·") {
		t.Fatalf("Plain.md: %q", added)
	}
	if added := pbVrRead(t, f, filepath.Join(vault, "projects", "demo", "Proj.md")); !strings.Contains(added, "brain read · brain:project/Proj ·") {
		t.Fatalf("project note: %q", added)
	}
}

func TestPbVrReadOutsideTheVaultNotesLogsNothing(t *testing.T) {
	f, vault := pbVrHookVault(t)
	for _, p := range []string{
		filepath.Join(vault, ".obsidian", "Hidden.md"),
		filepath.Join(vault, "projects", "other", "Else.md"),
		filepath.Join(vault, "Ideas", "readme.txt"),
		filepath.Join(vault, "Missing.md"),
		filepath.Join(filepath.Dir(vault), "Plain.md"),
	} {
		if added := pbVrRead(t, f, p); strings.Contains(added, "brain read") {
			t.Errorf("%s logged %q", p, added)
		}
	}
}
