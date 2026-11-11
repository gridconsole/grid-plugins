package braincmd

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

func proposeArgs(task string, extra ...string) []string {
	args := []string{"propose"}
	if task != "" {
		args = append(args, "--task", task)
	}
	args = append(args, "--name", "fresh", "--topic", "conventions", "--description", "When checking changes", "--summary", "Run the checks", "--source", "T-01", "--body", "Run checks before landing.\n")
	return append(args, extra...)
}

func runProposeTask(t *testing.T, env Env, args []string) (int, string, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	env.Stdout, env.Stderr = &out, &stderr
	return Run(args, env), out.String(), stderr.String()
}

func historyLines(t *testing.T, dir, text string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), text)
}

func TestProposeTaskWritesTheTaskFolderAndHistory(t *testing.T) {
	env, dir := decideFixture(t)
	code, out, stderr := runProposeTask(t, env, proposeArgs("T-01", "--json"))
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if !strings.Contains(out, `"path":"task:brain-proposals/fresh.md"`) {
		t.Fatalf("output %s", out)
	}
	b, err := os.ReadFile(filepath.Join(dir, "brain-proposals", "fresh.md"))
	if err != nil {
		t.Fatalf("proposal is not in the task folder: %v", err)
	}
	if n, err := brain.Parse(b); err != nil || n.Status != "proposed" {
		t.Fatalf("parsed %+v, %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(env.GridHome, "projects", "sample", "brain-proposals", "fresh.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a task proposal must not land under the project: %v", err)
	}
	if n := historyLines(t, dir, "brain proposed · fresh"); n != 1 {
		t.Fatalf("history has %d brain proposed lines, want 1", n)
	}
	// A second propose of the same name is refused and adds no second line.
	if code, _, _ = runProposeTask(t, env, proposeArgs("T-01")); code != 1 {
		t.Fatalf("duplicate exit %d, want 1", code)
	}
	if n := historyLines(t, dir, "brain proposed · fresh"); n != 1 {
		t.Fatalf("history has %d brain proposed lines after a refused duplicate, want 1", n)
	}
}

// The proposal lands where decide reads it.
func TestProposeTaskThenDecideRoundTrip(t *testing.T) {
	env, dir := decideFixture(t)
	if code, _, stderr := runProposeTask(t, env, proposeArgs("T-01")); code != 0 {
		t.Fatalf("propose exit %d: %s", code, stderr)
	}
	code, _, stderr := runProposeTask(t, env, []string{"decide", "T-01", "fresh", "accept", "--note", "Reviewed"})
	if code != 0 {
		t.Fatalf("decide exit %d: %s", code, stderr)
	}
	note := filepath.Join(env.GridHome, "projects", "sample", "knowledge", "conventions", "fresh.md")
	b, err := os.ReadFile(note)
	if err != nil {
		t.Fatalf("note not filed in knowledge: %v", err)
	}
	if n, err := brain.Parse(b); err != nil || n.Status != "accepted" || !strings.Contains(n.Body, "Run checks before landing.") {
		t.Fatalf("filed note %+v, %v", n, err)
	}
	b, err = os.ReadFile(filepath.Join(dir, "brain-proposals", "fresh.md"))
	if err != nil {
		t.Fatal(err)
	}
	if n, err := brain.Parse(b); err != nil || n.Status != "accepted" {
		t.Fatalf("proposal after decide %+v, %v", n, err)
	}
}

func TestProposeTaskIsListedByKeep(t *testing.T) {
	env, _ := decideFixture(t)
	if code, _, stderr := runProposeTask(t, env, proposeArgs("T-01")); code != 0 {
		t.Fatalf("propose exit %d: %s", code, stderr)
	}
	code, out, stderr := runProposeTask(t, env, []string{"keep", "--task", "T-01"})
	if code != 0 {
		t.Fatalf("keep exit %d: %s", code, stderr)
	}
	if !strings.Contains(out, "task:brain-proposals/fresh.md -> conventions") {
		t.Fatalf("keep did not list the task proposal:\n%s", out)
	}
}

// Without --task the proposal stays at the project level, even from inside a task.
func TestProposeWithoutTaskStaysAtTheProjectLevel(t *testing.T) {
	env, dir := decideFixture(t)
	if code, _, stderr := runProposeTask(t, env, proposeArgs("")); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(env.GridHome, "projects", "sample", "brain-proposals", "fresh.md")); err != nil {
		t.Fatalf("project proposal missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "brain-proposals", "fresh.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a proposal without --task reached the task folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "history.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a proposal without --task wrote task history: %v", err)
	}
}

func TestProposeTaskThatDoesNotResolveWritesNothing(t *testing.T) {
	env, dir := decideFixture(t)
	code, _, stderr := runProposeTask(t, env, proposeArgs("T-99"))
	if code == 0 {
		t.Fatal("an unresolved --task exited 0")
	}
	if stderr == "" {
		t.Fatal("no error text for an unresolved --task")
	}
	var found []string
	filepath.WalkDir(env.GridHome, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, "fresh.md") {
			found = append(found, p)
		}
		return nil
	})
	if len(found) != 0 {
		t.Fatalf("an unresolved --task wrote %v", found)
	}
	if _, err := os.Stat(filepath.Join(dir, "history.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("an unresolved --task wrote history: %v", err)
	}
}
