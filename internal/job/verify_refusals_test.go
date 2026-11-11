package job

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// refusalOneLine is the text RunVerify puts in VerifyResult.Reason (verify.go:45-48).
func refusalOneLine(err error) string { return strings.Join(strings.Fields(err.Error()), " ") }

// wantRefusal checks the error text and that Reason carries the same text on one line.
func wantRefusal(t *testing.T, r VerifyResult, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("accepted: %+v", r)
	}
	if want != "" && err.Error() != want {
		t.Fatalf("error %q, want %q", err, want)
	}
	if r.Reason != refusalOneLine(err) || strings.Contains(r.Reason, "\n") {
		t.Fatalf("reason %q, want %q", r.Reason, refusalOneLine(err))
	}
	if r.Pass || r.ExitCode != -1 {
		t.Fatalf("refusal result %+v", r)
	}
}

// refusalTask makes a task folder that task.Read accepts, in project demo.
func refusalTask(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "T-01-example")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"task.md":   "---\nid: T-01\ntitle: Example\nproject: demo\nstate: active\ncreated: 2026-11-08T00:00:00Z\n---\n",
		"intent.md": "Example intent",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// verify.go:50-119: each refusal returns the error and the same text in Reason.
func TestRunVerifyRefusals(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	fileExists := Job{Verify: Verify{Kind: "file-exists", Path: "file.txt"}}
	t.Run("cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r, err := RunVerify(ctx, fileExists, dir)
		wantRefusal(t, r, err, "context canceled")
	})
	t.Run("two options values", func(t *testing.T) {
		r, err := RunVerify(context.Background(), fileExists, dir, VerifyOptions{}, VerifyOptions{})
		wantRefusal(t, r, err, "verify accepts at most one options value")
	})
	t.Run("folder that does not exist", func(t *testing.T) {
		r, err := RunVerify(context.Background(), fileExists, filepath.Join(dir, "missing"))
		if !os.IsNotExist(err) {
			t.Fatalf("error %v, want a not-exist error", err)
		}
		wantRefusal(t, r, err, "")
	})
	t.Run("folder that is a file", func(t *testing.T) {
		r, err := RunVerify(context.Background(), fileExists, file)
		wantRefusal(t, r, err, "verification directory is not a folder")
	})
	for _, tc := range []struct {
		name string
		v    Verify
		want string
	}{
		{"negative min_tests", Verify{Kind: "file-exists", Path: "file.txt", MinTests: -1}, "verify requires a command or kind and nonnegative min_tests"},
		{"negative min_tests with a command", Verify{Command: "true", MinTests: -1}, "verify requires a command or kind and nonnegative min_tests"},
		{"command together with a kind", Verify{Command: "true", Kind: "file-exists", Path: "file.txt"}, "verify requires a command or kind and nonnegative min_tests"},
		{"file-exists with no paths", Verify{Kind: "file-exists"}, "file-exists requires at least one path"},
		{"no command and no kind", Verify{}, "verify requires a command, file-exists or reviewer-check"},
		{"no command and an unknown kind", Verify{Kind: "shell"}, "verify requires a command, file-exists or reviewer-check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := RunVerify(context.Background(), Job{Verify: tc.v}, dir)
			wantRefusal(t, r, err, tc.want)
		})
	}
}

// verify_paths.go: resolveVerifyPaths refusals, reached through RunVerify with no dir.
func TestRunVerifyPathRefusals(t *testing.T) {
	ctx := context.Background()
	cwd := t.TempDir()
	proof := Verify{Kind: "file-exists", Path: "file.txt"}
	t.Run("relative recorded worktree and a task.md that is a folder", func(t *testing.T) {
		taskPath := t.TempDir()
		if err := os.Mkdir(filepath.Join(taskPath, "task.md"), 0700); err != nil {
			t.Fatal(err)
		}
		_, want := task.Read(taskPath)
		if want == nil {
			t.Fatal("task.Read accepted a task.md folder")
		}
		r, err := RunVerify(ctx, Job{Worktree: Worktree{"": "tree"}, Verify: proof}, "", VerifyOptions{GridHome: cwd, TaskPath: taskPath, Cwd: cwd})
		wantRefusal(t, r, err, want.Error())
	})
	t.Run("recorded worktree that is not one name", func(t *testing.T) {
		taskPath := refusalTask(t)
		for _, recorded := range []string{"a/b", `a\b`, `C:\x`} {
			if runtime.GOOS == "windows" && recorded == `C:\x` {
				continue // an absolute path on Windows; see the next test
			}
			r, err := RunVerify(ctx, Job{Worktree: Worktree{"": recorded}, Verify: proof}, "", VerifyOptions{GridHome: cwd, TaskPath: taskPath, Cwd: cwd})
			wantRefusal(t, r, err, "invalid recorded worktree name; specify --dir")
		}
	})
	t.Run("drive path is absolute only on Windows", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			// Linux and macOS: the backslash makes it a name holding a separator.
			if _, err := WorktreePath(cwd, "demo", `C:\x`); err == nil || err.Error() != "invalid recorded worktree name; specify --dir" {
				t.Fatalf("WorktreePath: %v", err)
			}
			return
		}
		// compiled, needs Windows run: verify_paths.go:16-17 returns it as it is.
		if got, err := WorktreePath(cwd, "demo", `C:\x`); err != nil || got != `C:\x` {
			t.Fatalf("WorktreePath: %q, %v", got, err)
		}
	})
	t.Run("two recorded worktrees", func(t *testing.T) {
		r, err := RunVerify(ctx, Job{Worktree: Worktree{"a": "one", "b": "two"}, Verify: proof}, "", VerifyOptions{Cwd: cwd})
		wantRefusal(t, r, err, "multiple worktrees; specify --dir")
	})
	t.Run("task proof path without a task folder", func(t *testing.T) {
		for _, v := range []Verify{
			{Kind: "file-exists", Path: "task:research.md"},
			{Kind: "file-exists", Paths: []string{"task:research.md"}},
		} {
			r, err := RunVerify(ctx, Job{Verify: v}, cwd, VerifyOptions{Cwd: cwd})
			wantRefusal(t, r, err, "task: proof path requires a task folder")
		}
	})
}
