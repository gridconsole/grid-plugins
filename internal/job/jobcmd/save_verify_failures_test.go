package jobcmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// jsvClosedWriter lets the first ok writes through, then fails every write.
type jsvClosedWriter struct {
	ok  int
	buf bytes.Buffer
}

func (w *jsvClosedWriter) Write(p []byte) (int, error) {
	if w.ok > 0 {
		w.ok--
		return w.buf.Write(p)
	}
	return 0, errors.New("closed")
}

// jsvTask returns the fixture env and the T-01 task folder.
func jsvTask(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	e, out, errs := fixture(t)
	return e, out, errs, filepath.Join(e.Cwd, "T-01")
}

// jsvSaveReviewerJob saves J-1 of T-01 with a reviewer-check verify, so no shell command runs.
func jsvSaveReviewerJob(t *testing.T, task string) {
	t.Helper()
	j, err := job.LoadPlan(task, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	j.Verify = job.Verify{Kind: "reviewer-check"}
	if _, err = save(task, j, job.Rendered{Brief: "b"}); err != nil {
		t.Fatal(err)
	}
}

// jsvSymlink makes link point at target, skipping the test where links are not allowed.
func jsvSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not available: " + err.Error())
	}
}

func jsvRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Row 2: the jobs folder cannot be made because jobs is a plain file.
func TestJsvSaveJobsFolderCannotBeMade(t *testing.T) {
	_, _, _, task := jsvTask(t)
	jobs := filepath.Join(task, "jobs")
	if err := os.WriteFile(jobs, []byte("not a folder"), 0600); err != nil {
		t.Fatal(err)
	}
	path, err := save(task, job.Job{ID: "J-1"}, job.Rendered{Brief: "b"})
	if err == nil || path != "" {
		t.Fatalf("path %q err %v", path, err)
	}
	if !strings.HasPrefix(err.Error(), "mkdir ") || !strings.Contains(err.Error(), filepath.Join("T-01", "jobs")) {
		t.Fatalf("error %q", err)
	}
	info, statErr := os.Lstat(jobs)
	if statErr != nil || !info.Mode().IsRegular() || jsvRead(t, jobs) != "not a folder" {
		t.Fatalf("jobs changed: %v %v", info, statErr)
	}
}

// Row 3: jobs is a link to a folder outside the task.
func TestJsvSaveJobsFolderLeavesTask(t *testing.T) {
	_, _, _, task := jsvTask(t)
	outside := t.TempDir()
	jsvSymlink(t, outside, filepath.Join(task, "jobs"))
	path, err := save(task, job.Job{ID: "J-1"}, job.Rendered{Brief: "b"})
	if err == nil || path != "" || !strings.Contains(err.Error(), "source escapes its root") {
		t.Fatalf("path %q err %v", path, err)
	}
	entries, readErr := os.ReadDir(outside)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("outside folder written: %v %v", entries, readErr)
	}
}

// Row 4: a saved file that became a link to outside the task counts as different inputs.
func TestJsvSaveLinkedFileIsDifferentInputs(t *testing.T) {
	_, _, _, task := jsvTask(t)
	j := job.Job{ID: "J-1"}
	r := job.Rendered{Brief: "b"}
	if _, err := save(task, j, r); err != nil {
		t.Fatal(err)
	}
	saved := filepath.Join(task, "jobs", "J-1")
	inputsBefore := jsvRead(t, filepath.Join(saved, "inputs.json"))
	metaBefore := jsvRead(t, filepath.Join(saved, "job.yaml"))
	outside := filepath.Join(t.TempDir(), "brief.md")
	if err := os.WriteFile(outside, []byte("b"), 0600); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(saved, "brief.md")
	if err := os.Remove(brief); err != nil {
		t.Fatal(err)
	}
	jsvSymlink(t, outside, brief)
	path, err := save(task, j, r)
	want := "Job J-1 already has different saved inputs; create a new Job for revised work"
	if err == nil || path != "" || err.Error() != want {
		t.Fatalf("path %q err %v", path, err)
	}
	if jsvRead(t, filepath.Join(saved, "inputs.json")) != inputsBefore || jsvRead(t, filepath.Join(saved, "job.yaml")) != metaBefore {
		t.Fatal("saved files changed")
	}
}

// Rows 5-7: verify cannot write its report.
func TestJsvVerifyReportWriteFails(t *testing.T) {
	e, out, errs, task := jsvTask(t)
	jsvSaveReviewerJob(t, task)
	args := []string{"verify", "J-1", "--task", "T-01"}

	// Control: a healthy buffer exits 3 with the plain report.
	if code := Run(args, e); code != 3 {
		t.Fatalf("control: exit %d stdout %s stderr %s", code, out, errs)
	}
	want := "exit: 3\ntests: unknown\ncommit: \npass: false\nreason: a reviewer must check this Job\n"
	if out.String() != want {
		t.Fatalf("control stdout %q want %q", out.String(), want)
	}

	cases := []struct {
		name string
		args []string
		ok   int
	}{
		{"json closed stdout", append(append([]string{}, args...), "--json"), 0},
		{"plain closed stdout", args, 0},
		{"plain reason line fails", args, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &jsvClosedWriter{ok: c.ok}
			broken := e
			broken.Stdout = w
			if code := Run(c.args, broken); code != 1 {
				t.Fatalf("exit %d want 1; stdout %q", code, w.buf.String())
			}
			if c.ok == 1 && strings.Contains(w.buf.String(), "reason:") {
				t.Fatalf("reason line written: %q", w.buf.String())
			}
		})
	}
}

// Row 8: jobs/J-1 is a plain file, so reading its job.yaml fails with something other than not-exist.
func TestJsvVerifyJobFolderIsFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a path through a file can read as not-exist and fall back to the plan")
	}
	e, out, errs, task := jsvTask(t)
	if err := os.MkdirAll(filepath.Join(task, "jobs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "jobs", "J-1"), []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"verify", "J-1", "--task", "T-01"}, e); code != 1 {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errs)
	}
	if !strings.HasPrefix(errs.String(), "job: ") {
		t.Fatalf("stderr %q", errs)
	}
}

// Row 9: job.yaml is a folder, so reading it fails.
func TestJsvVerifyJobYamlIsFolder(t *testing.T) {
	e, out, errs, task := jsvTask(t)
	yaml := filepath.Join(task, "jobs", "J-1", "job.yaml")
	if err := os.MkdirAll(yaml, 0755); err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"verify", "J-1", "--task", "T-01"}, e); code != 1 {
		t.Fatalf("exit %d stdout %s stderr %s", code, out, errs)
	}
	got := errs.String()
	if !strings.HasPrefix(got, "job: read ") || !strings.Contains(got, filepath.Join("jobs", "J-1", "job.yaml")) {
		t.Fatalf("stderr %q", got)
	}
}
