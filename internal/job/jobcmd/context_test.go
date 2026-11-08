package jobcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMissingMemberContext(t *testing.T) {
	base, out, errs := fixture(t)
	// Deliberately omit caller context, as a new adapter might accidentally do.
	env := Env{Cwd: base.Cwd, GridHome: base.GridHome, Machine: base.Machine, Stdout: out, Stderr: errs}
	if code := Run([]string{"render", "task", "1"}, env); code != 2 {
		t.Fatalf("exit %d, want 2; stdout %s stderr %s", code, out, errs)
	}
	if !strings.Contains(errs.String(), "missing member context") {
		t.Fatalf("missing context diagnostic: %s", errs)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(env.Cwd, "task", "jobs")); !os.IsNotExist(err) {
		t.Fatalf("refused render created artifacts: %v", err)
	}
}
