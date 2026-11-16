package routine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// ctxWrite makes a file and its folders under root.
func ctxWrite(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ctxRunArgv(t *testing.T, p ProcessExecutor, s Store, e Entry) []string {
	t.Helper()
	var args []string
	p.runCommand = func(cmd *exec.Cmd) error {
		args = cmd.Args
		return nil
	}
	if err := p.Execute(context.Background(), s, e); err != nil {
		t.Fatal(err)
	}
	return args
}

// A skill routine runs claude with a per-run settings layer: bundled skills off,
// and every skill of the person's own (user skills, user commands, and the skills
// of the Grid home it runs in) listed by name only. The person's settings files
// are not written, so their own sessions keep their full listing.
func TestCtxRoutineSkillLayerInArgv(t *testing.T) {
	home, gridHome := t.TempDir(), t.TempDir()
	ctxWrite(t, home, ".claude/skills/alpha/SKILL.md", "---\nname: alpha\ndescription: A.\n---\n")
	ctxWrite(t, home, ".claude/skills/beta/SKILL.md", "---\ndescription: B.\n---\n")
	ctxWrite(t, home, ".claude/skills/notaskill/README.md", "no skill file\n")
	ctxWrite(t, home, ".claude/commands/gamma.md", "Do gamma.\n")
	ctxWrite(t, home, ".claude/commands/notes.txt", "not a command\n")
	ctxWrite(t, gridHome, ".claude/skills/proj/SKILL.md", "---\ndescription: P.\n---\n")
	args := ctxRunArgv(t, ProcessExecutor{Home: home}, Store{GridHome: gridHome, Project: "example"}, Entry{Run: "signals"})
	want := []string{"claude", "-p", "Run the grid:signals skill once. Work only in the Grid data folder. The project is example.", "--allowedTools", "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)", "--max-budget-usd", "1", "--max-turns", "10", "--permission-mode", "dontAsk",
		"--settings", `{"disableBundledSkills":true,"skillOverrides":{"alpha":"name-only","beta":"name-only","gamma":"name-only","proj":"name-only"}}`}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("argv\n%q\nwant\n%q", args, want)
	}
	for _, f := range []string{filepath.Join(home, ".claude", "settings.json"), filepath.Join(home, ".claude", "settings.local.json"), filepath.Join(gridHome, ".claude", "settings.json"), filepath.Join(gridHome, ".claude", "settings.local.json")} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("the routine wrote %s: %v", f, err)
		}
	}
}

// A person with no skills of their own still gets bundled skills off.
func TestCtxRoutineSkillLayerWithoutOwnSkills(t *testing.T) {
	args := ctxRunArgv(t, ProcessExecutor{Home: t.TempDir()}, Store{GridHome: t.TempDir()}, Entry{Run: "signals"})
	if n := len(args); n < 2 || args[n-2] != "--settings" || args[n-1] != `{"disableBundledSkills":true}` {
		t.Fatalf("argv tail %q, want --settings {\"disableBundledSkills\":true}", args)
	}
}

// A grid command routine starts no agent, so it gets no settings layer.
func TestCtxGridRoutineHasNoSkillLayer(t *testing.T) {
	home := t.TempDir()
	ctxWrite(t, home, ".claude/skills/alpha/SKILL.md", "---\ndescription: A.\n---\n")
	args := ctxRunArgv(t, ProcessExecutor{GridBinary: "grid", Home: home}, Store{GridHome: t.TempDir()}, Entry{Run: "grid brain keep"})
	if want := []string{"grid", "brain", "keep"}; !reflect.DeepEqual(args, want) {
		t.Fatalf("argv %q, want %q", args, want)
	}
}

// A skill a Job or step names stays fully listed; the others are name-only.
func TestCtxSkillLayerKeepsNamedSkills(t *testing.T) {
	home := t.TempDir()
	ctxWrite(t, home, ".claude/skills/alpha/SKILL.md", "---\ndescription: A.\n---\n")
	ctxWrite(t, home, ".claude/skills/go-conventions/SKILL.md", "---\ndescription: G.\n---\n")
	got, err := SkillLayer(home, "", []string{"go-conventions"})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"disableBundledSkills":true,"skillOverrides":{"alpha":"name-only"}}`; got != want {
		t.Fatalf("layer %s, want %s", got, want)
	}
}
