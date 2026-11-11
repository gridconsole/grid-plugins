package jobcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const ffTask = "T-01-the-install-page"

// ffWorld is a Grid home with project "handbook" holding one task, and a work folder
// that is in no project (an agent's worktree).
func ffWorld(t *testing.T, caller where.Caller) (Env, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	home := t.TempDir()
	if err := project.Write(home, project.Record{Name: "handbook", ID: "01HANDBOOK"}); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(gridhome.TasksDir(home, "handbook"), ffTask)
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "plan.md"), []byte("# plan\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "J-2.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	e := Env{Resolver: where.Resolver{GridHome: home, Caller: caller}, Cwd: cwd, GridHome: home, Machine: "m1", Stdout: out, Stderr: errs}
	return e, out, errs, task
}

func ffLint(e Env, out, errs *bytes.Buffer, extra ...string) (int, string) {
	out.Reset()
	errs.Reset()
	code := Run(append([]string{"lint", "J-2.json", "--task", ffTask}, extra...), e)
	return code, out.String() + errs.String()
}

func TestFfJobProjectFlagResolvesTask(t *testing.T) {
	e, out, errs, task := ffWorld(t, where.Caller{Role: "owner"})
	_, text := ffLint(e, out, errs, "--project", "handbook")
	if strings.Contains(text, "unknown flag") || strings.Contains(text, "needs --project") || strings.Contains(text, "not found") {
		t.Fatalf("--project did not resolve the task: %s", text)
	}
	e.project = "handbook"
	got, err := resolveTask(ffTask, e)
	if err != nil || got != task {
		t.Fatalf("resolveTask = %q, %v; want %q", got, err, task)
	}
	// The flag also works with a custom ResolveTask hook set (the CLI sets one).
	e.ResolveTask = func(string) (string, error) { return "", os.ErrNotExist }
	if got, err := resolveTask(ffTask, e); err != nil || got != task {
		t.Fatalf("with hook: resolveTask = %q, %v; want %q", got, err, task)
	}
}

func TestFfJobProjectFlagMissingStillAsks(t *testing.T) {
	e, out, errs, _ := ffWorld(t, where.Caller{Role: "owner"})
	code, text := ffLint(e, out, errs)
	if code == 0 || !strings.Contains(text, "needs --project") {
		t.Fatalf("exit %d, want the needs --project error: %s", code, text)
	}
	if code, text := ffLint(e, out, errs, "--project"); code != 2 || !strings.Contains(text, "--project needs a value") {
		t.Fatalf("exit %d: %s", code, text)
	}
}

func TestFfJobProjectFlagGuestNotMember(t *testing.T) {
	e, out, errs, _ := ffWorld(t, where.Caller{Role: "guest", Member: "gus"})
	codeA, without := ffLint(e, out, errs)
	if !strings.Contains(without, "not found") {
		t.Fatalf("guest without --project: %q", without)
	}
	codeB, with := ffLint(e, out, errs, "--project", "handbook")
	codeC, absent := ffLint(e, out, errs, "--project", "no-such-project")
	// The resolver decides: a guest outside the project is refused with the answer it
	// gives for a project that does not exist, and the task is never resolved.
	if codeA == 0 || codeB != codeA || codeC != codeA {
		t.Fatalf("guest exits differ: without %d, with %d, absent %d", codeA, codeB, codeC)
	}
	if with != without || absent != without {
		t.Fatalf("guest answers differ: without %q, with %q, absent %q", without, with, absent)
	}
	if strings.Contains(with, "unknown flag") || strings.Contains(with, ffTask) {
		t.Fatalf("guest answer leaks: %s", with)
	}
}
