package jobcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// refusalRun runs one command and returns its exit code, stdout and stderr.
func refusalRun(e Env, args ...string) (int, string, string) {
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	e.Stdout, e.Stderr = out, errs
	code := Run(args, e)
	return code, out.String(), errs.String()
}

// refusalWant asserts the exit code, the exact stderr line and an empty stdout.
func refusalWant(t *testing.T, e Env, code int, stderr string, args ...string) {
	t.Helper()
	gotCode, out, errs := refusalRun(e, args...)
	if gotCode != code || errs != stderr || out != "" {
		t.Fatalf("%v: exit %d stdout %q stderr %q; want exit %d, empty stdout, stderr %q", args, gotCode, out, errs, code, stderr)
	}
}

// Flag and command refusals are all exit 2 with the one "job: <message>" line on stderr.
func TestJobFlagAndCommandRefusals(t *testing.T) {
	const dirMsg = "job: --dir is only supported by verify\n"
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"dir on render", []string{"render", "T-01", "1", "--dir", "x"}, dirMsg},
		{"dir on lint", []string{"lint", "brief.md", "--dir", "x"}, dirMsg},
		{"dir on show", []string{"show", "T-01", "J-1", "--dir", "x"}, dirMsg},
		{"task with no value", []string{"render", "T-01", "1", "--task"}, "job: --task needs a value\n"},
		{"budget with no value", []string{"render", "T-01", "1", "--budget"}, "job: --budget needs a value\n"},
		{"dir with no value", []string{"show", "--dir"}, "job: --dir needs a value\n"},
		{"budget 0", []string{"render", "T-01", "1", "--budget", "0"}, "job: --budget must be between 1 and 2000\n"},
		{"budget 2001", []string{"render", "T-01", "1", "--budget", "2001"}, "job: --budget must be between 1 and 2000\n"},
		{"budget x", []string{"render", "T-01", "1", "--budget", "x"}, "job: --budget must be between 1 and 2000\n"},
		{"unknown flag", []string{"render", "T-01", "1", "--bogus"}, "job: unknown flag --bogus\n"},
		{"unknown command", []string{"frobnicate", "T-01"}, "job: unknown job command \"frobnicate\"; use render, lint or show\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, _, _ := fixture(t)
			refusalWant(t, e, 2, c.want, c.args...)
			if _, err := os.Stat(filepath.Join(e.Cwd, "T-01", "jobs")); !os.IsNotExist(err) {
				t.Fatalf("a refused call created Job files: %v", err)
			}
		})
	}
	// The parse refusal under --json is the same error in the report, with exit 2.
	t.Run("parse refusal as json", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 2, "render", "T-01", "1", "--budget", "0", "--json")
		if r.OK || r.Error != "--budget must be between 1 and 2000" || errs.Len() != 0 {
			t.Fatalf("%+v stderr %q", r, errs)
		}
	})
}

// --budget 1 and 2000 are the ends of the range parse accepts. 2000 renders. 1 is
// past parse (no exit 2, no range message) but job.Render then refuses it on this
// fixture, which carries brain notes: the code wins over the brief's "accepted".
func TestRenderAcceptsTheBudgetEnds(t *testing.T) {
	t.Run("2000 renders", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 0, "render", "T-01", "1", "--budget", "2000", "--json")
		if !r.OK || r.ID != "J-1" || errs.Len() != 0 {
			t.Fatalf("%+v stderr %q", r, errs)
		}
		if _, err := os.Stat(r.Path); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("1 passes parse and is refused by the render", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 1, "render", "T-01", "1", "--budget", "1", "--json")
		if r.OK || r.Error != "brain budget too small for the elision banner" {
			t.Fatalf("%+v", r)
		}
		if f, err := parse([]string{"render", "T-01", "1", "--budget", "1"}); err != nil || f.budget != 1 {
			t.Fatalf("parse: %+v %v", f, err)
		}
		if _, err := os.Stat(filepath.Join(e.Cwd, "T-01", "jobs")); !os.IsNotExist(err) {
			t.Fatalf("a refused render created Job files: %v", err)
		}
	})
}

// A history append failure is printed to stderr; the render still succeeds and
// the Job is written (command.go: the brain given loop after saveReport).
func TestRenderHistoryFailureStillRenders(t *testing.T) {
	e, taskDir, hist := givenFixture(t, []string{"brain:project/deploy"}, func(h jobtest.Home) {
		h.Note("project", "deploy", "conventions", "How a release is deployed.", "Run the deploy script from the release branch.\n")
	})
	if err := os.Remove(hist); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(hist, 0o755); err != nil {
		t.Fatal(err)
	}
	out, errs := newBuffers()
	e.Stdout, e.Stderr = out, errs
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if !r.OK || r.ID != "J-1" {
		t.Fatalf("%+v", r)
	}
	// One line per note the brief carries (deploy, and the recalled export-format).
	const line = "grid job render: history: history must be a regular file\n"
	if errs.String() != line+line {
		t.Fatalf("stderr %q; want the history error twice, once per note given", errs)
	}
	for _, f := range []string{filepath.Join("jobs", "J-1", "brief.md"), filepath.Join("jobs", "J-1", "inputs.json")} {
		if s, err := os.Stat(filepath.Join(taskDir, f)); err != nil || !s.Mode().IsRegular() {
			t.Fatalf("the render did not write %s: %v", f, err)
		}
	}
	if s, err := os.Stat(hist); err != nil || !s.IsDir() {
		t.Fatalf("history.md is no longer the directory: %v", err)
	}
}

// A guest invited to two tasks lints a file of the first with --task naming the
// second: the by-path gate wants --task to be the task the file lies in.
func TestGuestLintTaskNamingAnotherInvitedTask(t *testing.T) {
	for _, role := range []string{"guest", "owner"} {
		t.Run(role, func(t *testing.T) {
			env, out, errs := fixture(t)
			rendered := call(t, env, out, errs, 0, "render", "T-01", "1", "--json")
			second := filepath.Join(env.Cwd, "T-02")
			if err := os.CopyFS(second, os.DirFS(filepath.Join(env.Cwd, "T-01"))); err != nil {
				t.Fatal(err)
			}
			projectPath := filepath.Dir(env.Cwd)
			if err := os.WriteFile(filepath.Join(projectPath, "project-roles.json"), []byte(`{"people":[{"id":"gus"}]}`), 0o644); err != nil {
				t.Fatal(err)
			}
			env.Resolver.Caller = where.Caller{Role: role, Member: "gus"}
			// Both tasks resolve for the caller, so only the same-task rule refuses.
			for _, task := range []string{"T-01", "T-02"} {
				code, _, errText := refusalRun(env, "lint", rendered.Path, "--task", task)
				if role == "guest" && task == "T-01" && code != 0 {
					t.Fatalf("lint with the file's own task: exit %d stderr %q", code, errText)
				}
			}
			code, stdout, errText := refusalRun(env, "lint", rendered.Path, "--task", "T-02")
			if role == "guest" {
				if code != 1 || errText != "job: task not found\n" || stdout != "" {
					t.Fatalf("exit %d stdout %q stderr %q; want exit 1, empty stdout, \"job: task not found\"", code, stdout, errText)
				}
				if strings.Contains(errText, env.GridHome) {
					t.Fatalf("the refusal names a host path: %q", errText)
				}
				return
			}
			if code != 0 || errText != "" || strings.Contains(stdout, "task not found") {
				t.Fatalf("the owner was refused: exit %d stdout %q stderr %q", code, stdout, errText)
			}
		})
	}
}

// show refuses a task with no folder, the task given twice, a task whose saved
// Jobs are not exactly one when no id is given, and an id that is not a Job id.
func TestShowRefusals(t *testing.T) {
	t.Run("task with no folder", func(t *testing.T) {
		e, _, _ := fixture(t)
		// The real resolver refuses an unknown task with its own message (the where package).
		refusalWant(t, e, 1, "job: task T-99 needs --project: this folder is in no project with a record\n", "show", "T-99", "J-1")
		// The seam the package has: a resolver that finds no folder and no error.
		e.ResolveTask = func(string) (string, error) { return "", nil }
		refusalWant(t, e, 1, "job: cannot find task \"T-99\"; pass a task folder (one holding task.md, intent.md or plan.md)\n", "show", "T-99", "J-1")
	})
	t.Run("task given twice", func(t *testing.T) {
		e, _, _ := fixture(t)
		refusalWant(t, e, 1, "job: give the task once\n", "show", "T-01", "J-1", "--task", "T-01")
	})
	t.Run("zero saved Jobs", func(t *testing.T) {
		e, _, _ := fixture(t)
		refusalWant(t, e, 1, "job: task has 0 saved Jobs; specify a Job id\n", "show", "--task", "T-01")
	})
	t.Run("two saved Jobs", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
		b, err := os.ReadFile(r.Path)
		if err != nil {
			t.Fatal(err)
		}
		second := filepath.Join(filepath.Dir(filepath.Dir(r.Path)), "J-2")
		if err := os.MkdirAll(second, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(second, "brief.md"), b, 0o644); err != nil {
			t.Fatal(err)
		}
		refusalWant(t, e, 1, "job: task has 2 saved Jobs; specify a Job id\n", "show", "--task", "T-01")
	})
	t.Run("one saved Job is shown without an id", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
		saved, err := os.ReadFile(r.Path)
		if err != nil {
			t.Fatal(err)
		}
		shown := call(t, e, out, errs, 0, "show", "--task", "T-01", "--json")
		if !shown.OK || shown.ID != "J-1" || shown.Job == nil || shown.Brief != string(saved) {
			t.Fatalf("%+v", shown)
		}
	})
	t.Run("not a Job id", func(t *testing.T) {
		e, out, errs := fixture(t)
		call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
		refusalWant(t, e, 1, "job: invalid Job id\n", "show", "X-1", "--task", "T-01")
	})
}

// A saved Job whose brief does not decode, and a Feed that cannot be read, fail
// show with exit 1 and leave the files as they were.
func TestShowBrokenFiles(t *testing.T) {
	t.Run("brief does not decode", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
		broken := []byte("not a brief\n")
		if err := os.WriteFile(r.Path, broken, 0o644); err != nil {
			t.Fatal(err)
		}
		_, _, decodeErr := job.DecodeBrief(string(broken))
		if decodeErr == nil {
			t.Fatal("the broken brief decodes")
		}
		refusalWant(t, e, 1, "job: "+decodeErr.Error()+"\n", "show", "T-01", "J-1")
		if got, err := os.ReadFile(r.Path); err != nil || !bytes.Equal(got, broken) {
			t.Fatalf("show changed the brief: %q %v", got, err)
		}
	})
	t.Run("feed file is a directory", func(t *testing.T) {
		e, out, errs := fixture(t)
		r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
		saved, err := os.ReadFile(r.Path)
		if err != nil {
			t.Fatal(err)
		}
		taskDir := filepath.Join(e.Cwd, "T-01")
		// The Feed is read only for a task with a stable ULID in task.md.
		taskMD := "---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAW\nowner: member\n---\n# Export documentation\n"
		if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte(taskMD), 0o644); err != nil {
			t.Fatal(err)
		}
		feedFile := filepath.Join(taskDir, "feed", "01ARZ3NDEKTSV4RRFFQ69G5FAW.jsonl")
		if err := os.MkdirAll(feedFile, 0o755); err != nil {
			t.Fatal(err)
		}
		code, stdout, errText := refusalRun(e, "show", "T-01", "J-1")
		if code != 1 || stdout != "" || !strings.HasPrefix(errText, "job: ") || !strings.Contains(errText, "01ARZ3NDEKTSV4RRFFQ69G5FAW.jsonl") {
			t.Fatalf("exit %d stdout %q stderr %q; want exit 1 and the feed read error", code, stdout, errText)
		}
		if got, err := os.ReadFile(r.Path); err != nil || !bytes.Equal(got, saved) {
			t.Fatalf("show changed the brief: %v", err)
		}
		if s, err := os.Stat(feedFile); err != nil || !s.IsDir() {
			t.Fatalf("the feed file is no longer the directory: %v", err)
		}
	})
}
