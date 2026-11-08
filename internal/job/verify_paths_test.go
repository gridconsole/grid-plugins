package job

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreePath(t *testing.T) {
	gh := t.TempDir()
	for _, recorded := range []string{"example-tree", "example-tree@b44a66c9"} {
		got, err := WorktreePath(gh, "demo", recorded)
		want := filepath.Join(gh, "projects", "demo", "worktrees", "example-tree")
		if err != nil || got != want {
			t.Fatalf("%q: got %q, %v; want %q", recorded, got, err, want)
		}
	}
	absolute := t.TempDir()
	if got, err := WorktreePath("", "", absolute); err != nil || got != absolute {
		t.Fatalf("absolute worktree: %q, %v", got, err)
	}
	for _, tc := range []struct{ project, name string }{
		{"demo", ""}, {"demo", "@sha"}, {"demo", "..@sha"},
		{"demo", "../outside"}, {"demo", "repo/tree"}, {"demo", `repo\tree`},
		{"", "tree"}, {"..", "tree"}, {"../outside", "tree"},
	} {
		if _, err := WorktreePath(gh, tc.project, tc.name); err == nil {
			t.Fatalf("accepted project=%q worktree=%q", tc.project, tc.name)
		}
	}
}

func TestRunVerifyResolvesPaths(t *testing.T) {
	gh := t.TempDir()
	taskPath := filepath.Join(gh, "T-01-example")
	worktree := filepath.Join(gh, "projects", "demo", "worktrees", "example-tree")
	cwd := filepath.Join(gh, "cwd")
	override := filepath.Join(cwd, "override")
	for _, dir := range []string{taskPath, worktree, override} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		filepath.Join(taskPath, "task.md"):      "---\nid: T-01\ntitle: Example\nproject: demo\nstate: active\ncreated: 2026-11-08T00:00:00Z\n---\n",
		filepath.Join(taskPath, "intent.md"):    "Example intent",
		filepath.Join(taskPath, "proof.txt"):    "task proof",
		filepath.Join(worktree, "proof.txt"):    "worktree proof",
		filepath.Join(override, "override.txt"): "override proof",
		filepath.Join(cwd, "cwd.txt"):           "cwd proof",
	} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	o := VerifyOptions{GridHome: gh, TaskPath: taskPath, Cwd: cwd}
	for _, key := range []string{"", "repo"} {
		j := Job{Worktree: Worktree{key: "example-tree@b44a66c9"}, Verify: Verify{
			Kind: "file-exists", Path: "task:proof.txt", Paths: []string{"proof.txt", "task:proof.txt"},
		}}
		r, err := RunVerify(context.Background(), j, "", o)
		if err != nil || !r.Pass {
			t.Fatalf("worktree key=%q: %+v, %v", key, r, err)
		}
		if j.Verify.Paths[1] != "task:proof.txt" || j.Verify.Path != "task:proof.txt" {
			t.Fatal("mutated caller's proof paths")
		}
		j.Worktree["second"] = "other-tree@sha"
		if _, err := RunVerify(context.Background(), j, "", o); err == nil {
			t.Fatal("accepted ambiguous worktrees")
		}
		j.Verify.Paths = []string{"override.txt"}
		r, err = RunVerify(context.Background(), j, "override", o)
		if err != nil || !r.Pass {
			t.Fatalf("relative override: %+v, %v", r, err)
		}
	}
	r, err := RunVerify(context.Background(), Job{Verify: Verify{Kind: "file-exists", Path: "cwd.txt"}}, "", o)
	if err != nil || !r.Pass {
		t.Fatalf("cwd fallback: %+v, %v", r, err)
	}
	for _, proof := range []Verify{
		{Kind: "file-exists", Path: "task:../cwd/cwd.txt"},
		{Kind: "file-exists", Paths: []string{"task:../cwd/cwd.txt"}},
	} {
		if _, err := RunVerify(context.Background(), Job{Verify: proof}, "", o); err == nil {
			t.Fatal("accepted task proof outside task")
		}
	}
	if _, err := RunVerify(context.Background(), Job{Verify: Verify{Kind: "file-exists", Path: "task:proof.txt"}}, cwd); err == nil {
		t.Fatal("accepted task pointer without task context")
	}
}
