package jobgraph_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/jobgraph"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// TestGeNoFloor: the review floor is gone, and the plan check keeps what is not
// a level: write patterns, the plain risk, kind and visible values, and verify.
func TestGeNoFloor(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/job/task/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	base := string(raw)
	edit := func(t *testing.T, old, next string) string {
		t.Helper()
		if !strings.Contains(base, old) {
			t.Fatalf("fixture lacks %q", old)
		}
		return strings.Replace(base, old, next, 1)
	}

	t.Run("a level below the old floor passes", func(t *testing.T) {
		// review: L0 is in the fixture; risk and irreversible would have set floor
		// L2 with classify, and no review_reason is given.
		text := edit(t, "risk: []", "risk: [security]\n      irreversible: true")
		if !strings.Contains(text, "review: L0") || strings.Contains(text, "review_reason") {
			t.Fatal("fixture no longer has review: L0 without a review_reason")
		}
		if problems := jobgraph.CheckPlanReport(text).Problems; len(problems) != 0 {
			t.Fatalf("problems: %v", problems)
		}
	})

	for _, tc := range []struct{ name, old, next, want string }{
		{"unknown risk", "risk: []", "risk: [unknown]", `unknown risk "unknown"`},
		{"unknown work kind", "  kind: non-code", "  kind: paint", `unknown work kind "paint"`},
		{"unknown visible value", "risk: []", "risk: []\n      visible: huge", `unknown visible value "huge"`},
		// scope.go validatePattern: fmt.Errorf("invalid scope pattern %q", pattern)
		{"write pattern", "write: [docs/export.md]", "write: [../src]", `invalid scope pattern "../src"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems := strings.Join(jobgraph.CheckPlanReport(edit(t, tc.old, tc.next)).Problems, "\n")
			if !strings.Contains(problems, tc.want) {
				t.Fatalf("problems %q lack %q", problems, tc.want)
			}
		})
	}

	// Commands that TestVerifyCommands in floor_test.go refuses.
	for _, command := range []string{"make check", "go build ./...", "echo test", "make contest"} {
		t.Run("verify "+command, func(t *testing.T) {
			want := jobgraph.ValidateVerify(job.Job{Verify: job.Verify{Command: command, MinTests: 1}})
			if want == nil {
				t.Fatalf("ValidateVerify accepts %q", command)
			}
			text := edit(t, "verify: {kind: file-exists, path: docs/export.md}", "verify: {command: "+command+", min_tests: 1}")
			problems := strings.Join(jobgraph.CheckPlanReport(text).Problems, "\n")
			if !strings.Contains(problems, want.Error()) {
				t.Fatalf("problems %q lack %q", problems, want)
			}
		})
	}

	t.Run("grid job floor is not a verb", func(t *testing.T) {
		testgit.Isolate(t)
		root := t.TempDir()
		home := filepath.Join(root, "home")
		work := filepath.Join(root, "work")
		for _, dir := range []string{home, filepath.Join(home, ".grid"), work} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(work, "x.json"), []byte("[]"), 0o644); err != nil {
			t.Fatal(err)
		}
		run := func(args ...string) (int, string, string) {
			var out, errb bytes.Buffer
			code := cli.Run(args, cli.Env{Stdout: &out, Stderr: &errb, Stdin: strings.NewReader(""), Cwd: work,
				Home: home, GridHome: filepath.Join(home, ".grid"), TempRoots: []string{work}, Machine: "01TEST", Ceiling: root,
				GitVersion: func() (string, error) { return "git version 2.0.0-test", nil }})
			return code, out.String(), errb.String()
		}
		code, out, errb := run("job", "--help")
		if code != 0 || !strings.Contains(out, "grid job cycles") || strings.Contains(out, "grid job floor") {
			t.Fatalf("job --help: exit %d, stdout %q, stderr %q", code, out, errb)
		}
		// jobcmd.Run answers a verb it does not know with exit 2.
		code, out, errb = run("job", "floor", "x.json")
		if code != 2 || out != "" || !strings.Contains(errb, `unknown job command "floor"`) {
			t.Fatalf("job floor: exit %d, stdout %q, stderr %q", code, out, errb)
		}
		if code, out, errb = run("job", "cycles", "x.json"); code != 0 || out != "No Job cycles.\n" {
			t.Fatalf("job cycles: exit %d, stdout %q, stderr %q", code, out, errb)
		}
	})
}
