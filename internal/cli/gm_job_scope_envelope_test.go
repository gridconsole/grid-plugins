package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// TestGmJobFailureWithoutSentinelKeepsOutput pins what did not change: a job
// failure that carries no lookup sentinel (here no task selected) keeps its stderr
// line and writes nothing on stdout with --json. It passes on the base too.
func TestGmJobFailureWithoutSentinelKeepsOutput(t *testing.T) {
	h, _, _ := actFixture(t, "new")
	h.env.Getenv = func(string) string { return "" }
	cwd := filepath.Join(h.root, "checkout")
	h.mkdir(cwd)
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
	const want = "no task selected; use --task\n"
	for _, args := range [][]string{{"job", "tree"}, {"job", "tree", "--json"}} {
		if exit := h.run(cwd, args...); exit != 1 {
			t.Fatalf("%v: exit = %d, want 1: %s", args, exit, h.errb)
		}
		if h.errb.String() != want || h.out.Len() != 0 {
			t.Fatalf("%v: stderr = %q, stdout = %q; want %q and empty", args, h.errb, h.out, want)
		}
	}
}

// TestGmJobUnknownScope holds the eight job verbs and feed tick that printed the
// lookup error on stderr and wrote nothing on stdout with --json: each gives the
// no_such_project envelope for a project that is not registered and the
// no_such_task envelope for a task that does not exist, the same refusal a guest
// reads for a project or a task it cannot see. The arguments are those the walks
// build (guestParityCommands): none beyond the scope flag.
func TestGmJobUnknownScope(t *testing.T) {
	const (
		noProject = `project "absent" is not registered` + "\n"
		noTask    = "task T-99 not found in project example\n"
	)
	verbs := []struct {
		name string
		args []string
	}{
		{"job ack", []string{"job", "ack"}},
		{"job blocked", []string{"job", "blocked"}},
		{"job cancel", []string{"job", "cancel"}},
		{"job next", []string{"job", "next"}},
		{"job post", []string{"job", "post"}},
		{"job result", []string{"job", "result"}},
		{"job tree", []string{"job", "tree"}},
		{"feed tick", []string{"feed", "tick"}},
	}
	scopes := []struct {
		name, flag, value, code, stderr string
	}{
		{"project", "--project", "absent", CodeNoSuchProject, noProject},
		{"task", "--task", "T-99", CodeNoSuchTask, noTask},
	}
	for _, scope := range scopes {
		for _, verb := range verbs {
			t.Run(verb.name+" "+scope.name, func(t *testing.T) {
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

				args := append(append([]string{}, verb.args...), scope.flag, scope.value)

				if exit := h.run(cwd, args...); exit != 1 {
					t.Fatalf("plain exit = %d, want 1: %s", exit, h.errb)
				}
				stderr := h.errb.String()
				if stderr != scope.stderr {
					t.Fatalf("plain stderr = %q, want %q", stderr, scope.stderr)
				}
				if h.out.Len() != 0 {
					t.Fatalf("plain stdout = %q", h.out)
				}

				if exit := h.run(cwd, append(append([]string{}, args...), "--json")...); exit != 1 || h.errb.String() != stderr {
					t.Fatalf("--json changed exit or stderr: %d, %q; want 1, %q", exit, h.errb, stderr)
				}
				assertErrorOutput(t, h.out.String(), stderr, scope.code)
			})
		}
	}
}
