package jobcmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// saveVerifyJob writes jobs/<id>/job.yaml under the fixture's T-01 and returns
// the task folder.
func saveVerifyJob(t *testing.T, e Env, id, body string) string {
	t.Helper()
	taskPath := filepath.Join(e.Cwd, "T-01")
	dir := filepath.Join(taskPath, "jobs", id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "job.yaml"), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return taskPath
}

// verify.go:21, :25 and :29: argument and lookup refusals print "job: <error>"
// on stderr and leave stdout empty.
func TestVerifyArgumentRefusals(t *testing.T) {
	const usage = "job: verify requires J-N [--task T-NN] [--dir path] [--json]\n"
	for _, tc := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"no id", []string{"verify"}, 2, usage},
		{"two ids", []string{"verify", "J-1", "J-2", "--task", "T-01"}, 2, usage},
		{"id is not J-N", []string{"verify", "nope", "--task", "T-01"}, 2, usage},
		{"budget flag", []string{"verify", "J-1", "--budget", "3"}, 2, usage},
		{"task that does not resolve", []string{"verify", "J-1", "--task", "nosuch"}, 1, ""},
		{"no saved Job and no plan step", []string{"verify", "J-9", "--task", "T-01"}, 1, `plan.md: expected one item "J-9", found 0`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, out, errs := fixture(t)
			if code := Run(tc.args, e); code != tc.code {
				t.Fatalf("exit %d, want %d; stdout %s stderr %s", code, tc.code, out, errs)
			}
			if out.Len() != 0 {
				t.Fatalf("stdout not empty: %s", out)
			}
			got := errs.String()
			if !strings.HasPrefix(got, "job: ") || !strings.HasSuffix(got, "\n") {
				t.Fatalf("stderr %q", got)
			}
			switch {
			case tc.want == usage:
				if got != usage {
					t.Fatalf("stderr %q, want %q", got, usage)
				}
			case tc.want != "":
				if !strings.Contains(got, tc.want) {
					t.Fatalf("stderr %q lacks %q", got, tc.want)
				}
			case !strings.Contains(got, "nosuch"):
				t.Fatalf("stderr %q does not name the task", got)
			}
		})
	}
}

// verify.go:67-84: a saved Job that is damaged is an error, never the plan.
func TestLoadVerifyJobRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, body, want string
	}{
		{"not YAML", "id: [broken", ""},
		{"does not decode into a Job", "id: J-1\nverify: not-a-map\n", ""},
		{"id of another Job", "id: J-2\nverify:\n  kind: file-exists\n  path: proof.txt\n", "saved Job id does not match requested Job"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, out, errs := fixture(t)
			taskPath := saveVerifyJob(t, e, "J-1", tc.body)
			j, err := loadVerifyJob(taskPath, "J-1")
			if err == nil {
				t.Fatalf("accepted: %+v", j)
			}
			if tc.want != "" && err.Error() != tc.want {
				t.Fatalf("error %q, want %q", err, tc.want)
			}
			if code := Run([]string{"verify", "J-1", "--task", "T-01"}, e); code != 1 {
				t.Fatalf("exit %d, want 1; stdout %s stderr %s", code, out, errs)
			}
			if want := "job: " + err.Error() + "\n"; errs.String() != want || out.Len() != 0 {
				t.Fatalf("stderr %q stdout %q, want stderr %q", errs, out, want)
			}
		})
	}
}

// verify.go:46-56: the total is "unknown" unless the result has one, and the
// reason line appears only when a reason is set.
func TestVerifyTextOutput(t *testing.T) {
	run := func(t *testing.T, body string, dir string) (int, string) {
		t.Helper()
		e, out, errs := fixture(t)
		saveVerifyJob(t, e, "J-1", body)
		code := Run([]string{"verify", "J-1", "--task", "T-01", "--dir", dir}, e)
		if errs.Len() != 0 {
			t.Fatalf("stderr: %s", errs)
		}
		return code, out.String()
	}
	t.Run("file-exists passes: unknown total and no reason line", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "proof.txt"), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
		code, got := run(t, "id: J-1\nverify:\n  kind: file-exists\n  path: proof.txt\n", dir)
		if code != 0 || got != "exit: 0\ntests: unknown\ncommit: \npass: true\n" {
			t.Fatalf("exit %d, stdout %q", code, got)
		}
	})
	t.Run("file-exists fails: unknown total and a reason line", func(t *testing.T) {
		code, got := run(t, "id: J-1\nverify:\n  kind: file-exists\n  path: proof.txt\n", t.TempDir())
		want := "exit: 1\ntests: unknown\ncommit: \npass: false\nreason: proof files missing or not regular files: proof.txt\n"
		if code != 1 || got != want {
			t.Fatalf("exit %d, stdout %q, want %q", code, got, want)
		}
	})
	// The two rows below need a shell command: a file-exists verify never has a total.
	t.Run("command with a total", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX shell command fixture")
		}
		code, got := run(t, `{"id":"J-1","verify":{"command":"printf '# tests 2\\n'","min_tests":2}}`, t.TempDir())
		if code != 0 || got != "exit: 0\ntests: 2\ncommit: \npass: true\n" {
			t.Fatalf("exit %d, stdout %q", code, got)
		}
	})
	t.Run("command with unknown total against min_tests", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("POSIX shell command fixture")
		}
		code, got := run(t, `{"id":"J-1","verify":{"command":"true","min_tests":3}}`, t.TempDir())
		want := "exit: 0\ntests: unknown\ncommit: \npass: false\nreason: test totals unknown; at least 3 tests required\n"
		if code != 1 || got != want {
			t.Fatalf("exit %d, stdout %q, want %q", code, got, want)
		}
	})
}
