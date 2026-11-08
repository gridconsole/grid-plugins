package jobcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

func TestVerifyCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}
	e, out, errs := fixture(t)
	taskPath := filepath.Join(e.Cwd, "T-01")
	j, err := job.LoadPlan(taskPath, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	// Use saved metadata to prove it takes precedence over the plan.
	saved := filepath.Join(taskPath, "jobs", "J-1")
	if err = os.MkdirAll(saved, 0755); err != nil {
		t.Fatal(err)
	}
	j.Verify = job.Verify{Command: "printf '# tests 2\\n'", MinTests: 2}
	j.Worktree = job.Worktree{"": t.TempDir()}
	save := func() {
		t.Helper()
		b, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(saved, "job.yaml"), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	save()
	for _, dir := range []string{"", e.Cwd} {
		out.Reset()
		errs.Reset()
		args := []string{"verify", "J-1", "--task", "T-01", "--json"}
		if dir != "" {
			args = append(args, "--dir", dir)
		}
		if code := Run(args, e); code != 0 {
			t.Fatalf("%d: %s %s", code, out, errs)
		}
		var r job.VerifyResult
		if err = json.Unmarshal(out.Bytes(), &r); err != nil || !r.Pass || r.Tests == nil || *r.Tests != 2 {
			t.Fatalf("%s %v", out, err)
		}
	}
	j.Verify = job.Verify{Kind: "reviewer-check"}
	save()
	out.Reset()
	if code := Run([]string{"verify", "J-1", "--task", "T-01"}, e); code != 3 || !strings.Contains(out.String(), "reviewer must check") {
		t.Fatalf("%d: %s %s", code, out, errs)
	}
	if err = os.Remove(filepath.Join(saved, "job.yaml")); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadVerifyJob(taskPath, "J-1")
	if err != nil || loaded.Verify.Kind != "file-exists" {
		t.Fatalf("plan fallback: %+v %v", loaded, err)
	}
}

func TestVerifyNamedWorktreeAndTaskProof(t *testing.T) {
	e, out, errs := fixture(t)
	oldTask := filepath.Join(e.Cwd, "T-01")
	taskPath := filepath.Join(e.Cwd, "T-01-example")
	if err := os.Rename(oldTask, taskPath); err != nil {
		t.Fatal(err)
	}
	record := "---\nid: T-01\ntitle: Example\nproject: demo\nstate: active\ncreated: 2026-11-08T00:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(taskPath, "task.md"), []byte(record), 0600); err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(e.GridHome, "projects", "demo", "worktrees", "example-tree")
	if err := os.MkdirAll(worktree, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "proof.txt"), []byte("proof"), 0600); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(taskPath, "jobs", "J-1")
	if err := os.MkdirAll(saved, 0700); err != nil {
		t.Fatal(err)
	}
	for _, worktreeRecord := range []string{"example-tree", "example-tree@b44a66c9", "{repo: example-tree@b44a66c9}"} {
		metadata := "id: J-1\nworktree: " + worktreeRecord + "\nverify:\n  kind: file-exists\n  path: 'task:research.md'\n  paths: [proof.txt, 'task:research.md']\n"
		if err := os.WriteFile(filepath.Join(saved, "job.yaml"), []byte(metadata), 0600); err != nil {
			t.Fatal(err)
		}
		if code := Run([]string{"verify", "J-1", "--task", taskPath}, e); code != 0 {
			t.Fatalf("%s: %d: %s %s", worktreeRecord, code, out, errs)
		}
	}
	out.Reset()
	if code := Run([]string{"verify", "J-1", "--task", taskPath, "--dir", e.Cwd, "--json"}, e); code != 1 {
		t.Fatalf("%d: %s %s", code, out, errs)
	}
	var result job.VerifyResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || !strings.Contains(result.Reason, "proof.txt") {
		t.Fatalf("%s: %v", out, err)
	}
	// Malformed saved metadata must fail instead of silently running the plan.
	if err := os.WriteFile(filepath.Join(saved, "job.yaml"), []byte("id: [broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadVerifyJob(taskPath, "J-1"); err == nil {
		t.Fatal("invalid saved Job accepted")
	}
}
