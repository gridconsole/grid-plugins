package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestGmWorkflowUnknownProject pins contract/README.md:38 for the two workflow
// commands that take --project: an unknown project exits 1 with the same message
// plain and with --json, and --json adds the no_such_project envelope on stdout.
func TestGmWorkflowUnknownProject(t *testing.T) {
	base := t.TempDir()
	for name, dir := range map[string]string{"HOME": "home", "GRID_HOME": "grid", "CLAUDE_CONFIG_DIR": "claude", "CODEX_HOME": "codex"} {
		path := filepath.Join(base, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv(name, path)
	}
	if home := os.Getenv("HOME"); !strings.HasPrefix(home, base+string(os.PathSeparator)) {
		t.Fatalf("HOME = %q is not under the test folder %q", home, base)
	}

	showMessage := "grid: project \"absent\" is missing or ambiguous\n"
	writeMessage := "grid workflow write: project \"absent\" is missing or ambiguous\n"
	// workflow write puts --json in its argv helper; the plain form drops it.
	writeArgs := slices.DeleteFunc(workflowWriteArgs("none"), func(arg string) bool { return arg == "--json" })
	for _, tc := range []struct {
		name    string
		args    []string
		message string
		// --policy without --json is a usage error (exit 2), so only --json applies.
		jsonOnly bool
	}{
		{"show", []string{"workflow", "show", "--project", "absent"}, showMessage, false},
		{"show-policy", []string{"workflow", "show", "--policy", "--project", "absent"}, showMessage, true},
		{"write", append(writeArgs, "--project", "absent"), writeMessage, false},
	} {
		for _, asJSON := range []bool{false, true} {
			if tc.jsonOnly && !asJSON {
				continue
			}
			mode := "plain"
			if asJSON {
				mode = "json"
			}
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				h := newHarness(t)
				h.env.Getenv = func(string) string { return "" }
				h.env.Stdin = strings.NewReader("# Workflows\n")
				h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
				args := slices.Clone(tc.args)
				if asJSON {
					args = append(args, "--json")
				}
				code := h.run(h.root, args...)
				if code != 1 || h.errb.String() != tc.message {
					t.Fatalf("exit %d: stderr %q, want exit 1 and %q", code, h.errb, tc.message)
				}
				if asJSON {
					assertErrorOutput(t, h.out.String(), h.errb.String(), CodeNoSuchProject)
				} else if h.out.Len() != 0 {
					t.Fatalf("plain stdout = %q, want empty", h.out)
				}
			})
		}
	}
}
