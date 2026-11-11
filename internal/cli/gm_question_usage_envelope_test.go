package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// TestGmQuestionUsageUnknownTask holds the eight members that printed the lookup
// error on stderr and wrote nothing on stdout with --json: each gives the
// no_such_task envelope for a task that does not exist, the same refusal a guest
// reads for a task it cannot see.
func TestGmQuestionUsageUnknownTask(t *testing.T) {
	const want = "T-99 not found in project example"
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"question add", []string{"question", "add", "parity?", "--applies", "2030-01-01T00:00:00Z", "--default", "defer", "--task", "T-99"}},
		{"question ask", []string{"question", "ask", "parity?", "--applies", "2030-01-01T00:00:00Z", "--default", "defer", "--task", "T-99"}},
		{"question list", []string{"question", "list", "--task", "T-99"}},
		{"question sync", []string{"question", "sync", "--task", "T-99"}},
		{"question tick", []string{"question", "tick", "--task", "T-99"}},
		{"question withdraw", []string{"question", "withdraw", "Q-1", "--task", "T-99"}},
		{"answer", []string{"answer", "T-99", "Q-1", "yes"}},
		{"usage", []string{"usage", "--task", "T-99"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			// Nothing reads or writes a real home; no child process runs.
			for key, dir := range map[string]string{"HOME": h.home, "GRID_HOME": h.gridHome,
				"CLAUDE_CONFIG_DIR": filepath.Join(h.home, ".claude"), "CODEX_HOME": filepath.Join(h.home, ".codex")} {
				t.Setenv(key, dir)
			}
			if home := os.Getenv("HOME"); !strings.HasPrefix(home, h.root+string(filepath.Separator)) {
				t.Fatalf("HOME %q is not under the test folder %q", home, h.root)
			}
			h.env.Getenv = func(string) string { return "" }
			cwd := filepath.Join(h.root, "checkout")
			h.mkdir(cwd)
			h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})

			if exit := h.run(cwd, tc.args...); exit != 1 {
				t.Fatalf("plain exit = %d, want 1: %s", exit, h.errb)
			}
			stderr := h.errb.String()
			if !strings.HasSuffix(stderr, want+"\n") {
				t.Fatalf("plain stderr = %q, want it to end with %q", stderr, want)
			}
			if h.out.Len() != 0 {
				t.Fatalf("plain stdout = %q", h.out)
			}

			args := append(append([]string{}, tc.args...), "--json")
			if exit := h.run(cwd, args...); exit != 1 || h.errb.String() != stderr {
				t.Fatalf("--json changed exit or stderr: %d, %q; want 1, %q", exit, h.errb, stderr)
			}
			assertErrorOutput(t, h.out.String(), stderr, CodeNoSuchTask)
		})
	}
}
