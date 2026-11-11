package jobcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job"
)

const tpPointer = "brain:project/x"

// tpLink makes a symbolic link, or skips the test where the platform refuses
// (Windows without the privilege).
func tpLink(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(from, to); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
}

func tpTrace(t *testing.T, home string) TraceResult {
	t.Helper()
	result, err := Trace(ownerResolver(home), tpPointer, "", "")
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// tpOneRow checks that the good task T-01 / J-1 is still traced, once.
func tpOneRow(t *testing.T, result TraceResult) {
	t.Helper()
	if len(result.Rows) != 1 || result.Rows[0].Task != "T-01" || result.Rows[0].Job != "J-1" || result.Rows[0].Project != "alpha" {
		t.Fatalf("rows: %+v", result.Rows)
	}
}

func tpProblems(t *testing.T, result TraceResult, want ...TraceProblem) {
	t.Helper()
	if len(result.Problems) != len(want) {
		t.Fatalf("problems: %+v, want %+v", result.Problems, want)
	}
	for i := range want {
		if result.Problems[i] != want[i] {
			t.Fatalf("problem %d: got %+v, want %+v", i, result.Problems[i], want[i])
		}
	}
}

func tpGoodTask(t *testing.T, home string) string {
	t.Helper()
	dir := traceTask(t, home, "alpha", "T-01")
	traceTestBundle(t, dir, "J-1", job.Source{Pointer: tpPointer})
	return dir
}

func TestTracePathRefusals(t *testing.T) {
	root := t.TempDir()
	traceWrite(t, filepath.Join(root, "real", "f.txt"), []byte("x"))
	traceWrite(t, filepath.Join(root, "b"), []byte("x"))

	t.Run("regular path", func(t *testing.T) {
		got, err := tracePath(root, "real/f.txt")
		if err != nil || got == "" {
			t.Fatalf("%q, %v", got, err)
		}
	})
	t.Run("symlinked part", func(t *testing.T) {
		tpLink(t, filepath.Join(root, "real"), filepath.Join(root, "link"))
		// The link stays inside the root, so Within accepts it and only the
		// no-symlink rule of tracePath refuses it.
		_, err := tracePath(root, "link/f.txt")
		if err == nil || err.Error() != "symbolic link refused" {
			t.Fatalf("got %v, want symbolic link refused", err)
		}
	})
	t.Run("missing part", func(t *testing.T) {
		_, err := tracePath(root, "absent/f.txt")
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("got %v, want a not-exist error", err)
		}
	})
	t.Run("missing part that Within does not see", func(t *testing.T) {
		// Within cleans "absent/.." away before it looks at the disk, so the
		// per-part Lstat is what reports the missing folder.
		_, err := tracePath(root, "absent/../b")
		var pathErr *fs.PathError
		if !errors.Is(err, fs.ErrNotExist) || !errors.As(err, &pathErr) || pathErr.Op != "lstat" {
			t.Fatalf("got %v, want an lstat not-exist error", err)
		}
	})
}

func TestTraceHomeAndProjectErrors(t *testing.T) {
	t.Run("no home", func(t *testing.T) {
		_, err := Trace(ownerResolver(""), tpPointer, "", "")
		if err == nil || err.Error() != "cannot find the Grid home; set GRID_HOME" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("projects is a regular file", func(t *testing.T) {
		// TasksDirs reads <home>/projects; a file there is an error that is not
		// "does not exist", so Trace fails as a whole.
		home := t.TempDir()
		traceWrite(t, filepath.Join(home, "projects"), []byte("not a folder"))
		_, err := Trace(ownerResolver(home), tpPointer, "", "")
		if err == nil || err.Error() != "cannot list project task folders" {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("unreadable project records", func(t *testing.T) {
		home := t.TempDir()
		traceTask(t, home, "alpha", "T-01")
		traceWrite(t, filepath.Join(home, "projects", "alpha", "project.md"), []byte("no front matter here\n"))
		// The filter names no folder, so the records are read, and one does not parse.
		_, err := Trace(ownerResolver(home), tpPointer, "", "nomatch")
		if err == nil || err.Error() != "cannot read project records" {
			t.Fatalf("got %v", err)
		}
		// A filter that is a folder name never reads the records.
		if _, err := Trace(ownerResolver(home), tpPointer, "", "alpha"); err != nil {
			t.Fatalf("folder-name filter: %v", err)
		}
	})
	t.Run("ambiguous project id", func(t *testing.T) {
		home := t.TempDir()
		for _, name := range []string{"alpha", "beta"} {
			traceTask(t, home, name, "T-01")
			traceWrite(t, filepath.Join(home, "projects", name, "project.md"), []byte("---\nid: SAMEID\n---\n"))
		}
		_, err := Trace(ownerResolver(home), tpPointer, "", "SAMEID")
		if err == nil || err.Error() != "project id is ambiguous" {
			t.Fatalf("got %v", err)
		}
		// The command turns it into a failure, not a not-found.
		_, _, errs := traceCall(t, home, 1, tpPointer, "--project", "SAMEID")
		if errs != "job trace: project id is ambiguous\n" {
			t.Fatalf("stderr: %q", errs)
		}
	})
	t.Run("a prefix of an id is not a match", func(t *testing.T) {
		home := t.TempDir()
		traceTask(t, home, "alpha", "T-01")
		traceWrite(t, filepath.Join(home, "projects", "alpha", "project.md"), []byte("---\nid: SAMEID\n---\n"))
		if _, err := Trace(ownerResolver(home), tpPointer, "", "SAME"); err != errTraceProjectNotFound {
			t.Fatalf("got %v, want project not found", err)
		}
	})
}

// A project whose tasks/ is a regular file is not an error of TasksDirs, which
// only lists the folders under projects/. It reaches the problem
// `project "alpha": cannot list tasks`. The error "cannot list project task
// folders" is reached through projects/ being a file (TestTraceHomeAndProjectErrors).
func TestTraceProjectTasksFile(t *testing.T) {
	home := t.TempDir()
	tpGoodTask(t, home)
	traceWrite(t, filepath.Join(home, "projects", "beta", "tasks"), []byte("not a folder"))
	result := tpTrace(t, home)
	tpOneRow(t, result)
	tpProblems(t, result, TraceProblem{Text: `project "beta": cannot list tasks`})
}

func TestTraceProjectTaskFolderUnsafe(t *testing.T) {
	t.Run("parent is a regular file", func(t *testing.T) {
		// <home>/personal is a file, so Lstat of personal/tasks fails with an
		// error that is not "does not exist".
		home := t.TempDir()
		tpGoodTask(t, home)
		traceWrite(t, filepath.Join(home, gridhome.Personal), []byte("not a folder"))
		result := tpTrace(t, home)
		tpOneRow(t, result)
		tpProblems(t, result, TraceProblem{Text: `project "personal": task folder is unreadable or unsafe`})
	})
	t.Run("tasks is a symbolic link", func(t *testing.T) {
		home := t.TempDir()
		tpGoodTask(t, home)
		other := traceTask(t, home, "beta", "T-09")
		tpLink(t, filepath.Dir(other), filepath.Join(home, "projects", "gamma", "tasks"))
		result := tpTrace(t, home)
		tpProblems(t, result, TraceProblem{Text: `project "gamma": task folder is unreadable or unsafe`})
		tpOneRow(t, result)
	})
}

func TestTraceLinkedTaskFolder(t *testing.T) {
	home := t.TempDir()
	good := tpGoodTask(t, home)
	tpLink(t, good, filepath.Join(filepath.Dir(good), "T-03-link"))
	result := tpTrace(t, home)
	// The target's Job is traced through its real folder only, not through the link.
	tpOneRow(t, result)
	tpProblems(t, result, TraceProblem{Task: "T-03-link", Text: "task folder is unreadable or unsafe"})
}

func TestTraceTaskProblems(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, tasks string)
		want  TraceProblem
	}{
		{"task folder is a regular file", func(t *testing.T, tasks string) {
			traceWrite(t, filepath.Join(tasks, "T-02-bad"), []byte("not a folder"))
		}, TraceProblem{Task: "T-02-bad", Text: "task folder is unreadable or unsafe"}},
		{"task.md is a folder", func(t *testing.T, tasks string) {
			if err := os.MkdirAll(filepath.Join(tasks, "T-02-bad", "task.md"), 0755); err != nil {
				t.Fatal(err)
			}
			traceWrite(t, filepath.Join(tasks, "T-02-bad", "intent.md"), []byte("intent\n"))
		}, TraceProblem{Task: "T-02-bad", Text: "task metadata is unreadable or unsafe"}},
		{"intent.md is missing", func(t *testing.T, tasks string) {
			traceWrite(t, filepath.Join(tasks, "T-02-bad", "task.md"), []byte("---\nid: T-02\ntitle: Bad\nproject: alpha\nstate: open\ncreated: 2026-01-01T00:00:00Z\n---\n"))
		}, TraceProblem{Task: "T-02-bad", Text: "task metadata is unreadable or unsafe"}},
		{"task.md names another project", func(t *testing.T, tasks string) {
			traceWrite(t, filepath.Join(tasks, "T-02-bad", "task.md"), []byte("---\nid: T-02\ntitle: Bad\nproject: beta\nstate: open\ncreated: 2026-01-01T00:00:00Z\n---\n"))
			traceWrite(t, filepath.Join(tasks, "T-02-bad", "intent.md"), []byte("intent\n"))
		}, TraceProblem{Task: "T-02-bad", Text: "invalid task metadata or mismatched project"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			good := tpGoodTask(t, home)
			tc.build(t, filepath.Dir(good))
			result := tpTrace(t, home)
			tpOneRow(t, result)
			tpProblems(t, result, tc.want)
		})
	}
}

func TestTraceBundleProblems(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, dir string)
		want  TraceProblem
	}{
		{"jobs is a regular file", func(t *testing.T, dir string) {
			// A second task, so T-01 keeps its Job.
			other := traceTask(t, filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(dir)))), "alpha", "T-02")
			traceWrite(t, filepath.Join(other, "jobs"), []byte("not a folder"))
		}, TraceProblem{Task: "T-02", Text: "cannot list Job bundles"}},
		{"bundle is a regular file", func(t *testing.T, dir string) {
			traceWrite(t, filepath.Join(dir, "jobs", "J-2"), []byte("not a folder"))
		}, TraceProblem{Task: "T-01", Job: "J-2", Text: "Job bundle is unreadable or unsafe"}},
		{"inputs.json is a folder", func(t *testing.T, dir string) {
			if err := os.MkdirAll(filepath.Join(dir, "jobs", "J-2", "inputs.json"), 0755); err != nil {
				t.Fatal(err)
			}
		}, TraceProblem{Task: "T-01", Job: "J-2", Text: "inputs.json must be a regular file"}},
		{"inputs.json is not json", func(t *testing.T, dir string) {
			traceWrite(t, filepath.Join(dir, "jobs", "J-2", "inputs.json"), []byte("not json"))
		}, TraceProblem{Task: "T-01", Job: "J-2", Text: "invalid inputs.json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			dir := tpGoodTask(t, home)
			tc.build(t, dir)
			result := tpTrace(t, home)
			tpOneRow(t, result)
			tpProblems(t, result, tc.want)
		})
	}
}

func TestTraceBundleSymlinkProblems(t *testing.T) {
	t.Run("jobs is a symbolic link", func(t *testing.T) {
		home := t.TempDir()
		dir := tpGoodTask(t, home)
		other := traceTask(t, home, "alpha", "T-02")
		tpLink(t, filepath.Join(dir, "jobs"), filepath.Join(other, "jobs"))
		result := tpTrace(t, home)
		tpOneRow(t, result)
		tpProblems(t, result, TraceProblem{Task: "T-02", Text: "jobs folder is unreadable or unsafe"})
	})
	t.Run("inputs.json is a symbolic link", func(t *testing.T) {
		home := t.TempDir()
		dir := tpGoodTask(t, home)
		tpLink(t, filepath.Join(dir, "jobs", "J-1", "inputs.json"), filepath.Join(dir, "jobs", "J-2", "inputs.json"))
		result := tpTrace(t, home)
		tpOneRow(t, result)
		tpProblems(t, result, TraceProblem{Task: "T-01", Job: "J-2", Text: "inputs.json is unreadable or unsafe"})
	})
}

func TestTraceCommandProblems(t *testing.T) {
	home := t.TempDir()
	dir := tpGoodTask(t, home)
	traceWrite(t, filepath.Join(dir, "jobs", "J-2", "inputs.json"), []byte("not json"))
	if err := os.MkdirAll(filepath.Join(dir, "jobs", "J-3", "inputs.json"), 0755); err != nil {
		t.Fatal(err)
	}
	want := []TraceProblem{
		{Task: "T-01", Job: "J-2", Text: "invalid inputs.json"},
		{Task: "T-01", Job: "J-3", Text: "inputs.json must be a regular file"},
	}

	// Problems do not change the exit status (traceCall requires 0) and are printed.
	_, out, errs := traceCall(t, home, 0, tpPointer)
	if errs != "" {
		t.Fatalf("stderr: %q", errs)
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], `"alpha" "T-01" "J-1" "brain:project/x"`) ||
		lines[1] != `problem: "T-01" "J-2": invalid inputs.json` ||
		lines[2] != `problem: "T-01" "J-3": inputs.json must be a regular file` {
		t.Fatalf("text output:\n%s", out)
	}

	result, _, _ := traceCall(t, home, 0, tpPointer, "--json")
	var problems []TraceProblem
	if err := json.Unmarshal(result["problems"], &problems); err != nil {
		t.Fatal(err)
	}
	if len(problems) != len(want) || problems[0] != want[0] || problems[1] != want[1] {
		t.Fatalf("problems: %+v, want %+v", problems, want)
	}
}

func TestTraceCommandNoHome(t *testing.T) {
	const text = "cannot find the Grid home; set GRID_HOME"
	t.Run("text", func(t *testing.T) {
		out, errs := &bytes.Buffer{}, &bytes.Buffer{}
		code := Run([]string{"trace", tpPointer}, Env{Resolver: ownerResolver(""), Stdout: out, Stderr: errs})
		if code != traceExitFailed || errs.String() != "job trace: "+text+"\n" || out.Len() != 0 {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
		}
	})
	t.Run("json", func(t *testing.T) {
		out, errs := &bytes.Buffer{}, &bytes.Buffer{}
		code := Run([]string{"trace", tpPointer, "--json"}, Env{Resolver: ownerResolver(""), Stdout: out, Stderr: errs})
		if code != traceExitFailed || errs.Len() != 0 {
			t.Fatalf("exit %d, stdout %q, stderr %q", code, out, errs)
		}
		var got TraceResult
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Rows) != 0 || len(got.Problems) != 1 || got.Problems[0] != (TraceProblem{Text: text}) {
			t.Fatalf("stdout: %s", out)
		}
	})
}
