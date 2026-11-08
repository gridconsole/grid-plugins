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
	taskPath := filepath.Join(e.Cwd, "task")
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
		args := []string{"verify", "J-1", "--task", "task", "--json"}
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
	if code := Run([]string{"verify", "J-1", "--task", "task"}, e); code != 3 || !strings.Contains(out.String(), "reviewer must check") {
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
