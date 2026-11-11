package jobcmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// owClosedWriter stands in for a closed stdout or stderr: every write fails.
type owClosedWriter struct{}

func (owClosedWriter) Write([]byte) (int, error) { return 0, errors.New("write on closed pipe") }

// owSavedBrief renders T-01 item 1 on the fixture (exit 0) and returns the saved
// brief.md path, which lints clean.
func owSavedBrief(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer, string) {
	t.Helper()
	e, out, errs := fixture(t)
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if !r.OK || r.Path == "" {
		t.Fatalf("render: %+v", r)
	}
	brief := filepath.Join(e.Cwd, "T-01", "jobs", "J-1", "brief.md")
	if _, err := os.Stat(brief); err != nil {
		t.Fatal(err)
	}
	return e, out, errs, brief
}

func owRun(e Env, args ...string) int { return Run(args, e) }

// owTraceHome builds a home with one bundle filing brain:home/x under alpha/T-01.
func owTraceHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	traceTestBundle(t, traceTask(t, home, "alpha", "T-01"), "J-1", job.Source{Pointer: "brain:home/x"})
	return home
}

// Row 2: the usage write.
func TestOWUsageWriteFails(t *testing.T) {
	e, _, errs := fixture(t)
	e.Stdout = owClosedWriter{}
	if got := owRun(e, "--help"); got != 1 {
		t.Fatalf("--help to a closed stdout: exit %d want 1; stderr %s", got, errs)
	}
}

// Rows 3 and 4: the report writer, JSON and plain, with a healthy buffer as control.
func TestOWOutputWriteFails(t *testing.T) {
	e, out, errs, brief := owSavedBrief(t)
	call(t, e, out, errs, 0, "lint", brief, "--json")
	e.Stdout = owClosedWriter{}
	if got := owRun(e, "lint", brief, "--json"); got != 1 {
		t.Fatalf("lint --json to a closed stdout: exit %d want 1", got)
	}

	e, out, errs, brief = owSavedBrief(t)
	call(t, e, out, errs, 0, "lint", brief)
	if want := "path: " + display(brief, e) + "\nok: true\n"; out.String() != want {
		t.Fatalf("plain lint output %q want %q", out.String(), want)
	}
	e.Stdout = owClosedWriter{}
	if got := owRun(e, "lint", brief); got != 1 {
		t.Fatalf("lint to a closed stdout: exit %d want 1", got)
	}
}

// Row 5: the error line to a closed stderr is exit 1, not the usage 2.
func TestOWErrorLineWriteFails(t *testing.T) {
	e, out, errs := fixture(t)
	call(t, e, out, errs, 2, "lint", "a", "b")
	e.Stderr = owClosedWriter{}
	if got := owRun(e, "lint", "a", "b"); got != 1 {
		t.Fatalf("lint a b to a closed stderr: exit %d want 1", got)
	}
}

// Row 6: --task that does not resolve.
func TestOWLintTaskNotResolved(t *testing.T) {
	e, out, errs, brief := owSavedBrief(t)
	call(t, e, out, errs, 1, "lint", brief, "--task", "T-99")
	if want := "job: task T-99 needs --project: this folder is in no project with a record\n"; errs.String() != want {
		t.Fatalf("stderr %q want %q", errs.String(), want)
	}
}

// Row 7: a brief path that is a directory.
func TestOWShowBriefIsDirectory(t *testing.T) {
	e, out, errs, brief := owSavedBrief(t)
	if err := os.Remove(brief); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(brief, 0755); err != nil {
		t.Fatal(err)
	}
	call(t, e, out, errs, 1, "show", "T-01", "J-1")
	tail := filepath.Join("T-01", "jobs", "J-1", "brief.md")
	if !strings.HasPrefix(errs.String(), "job: read ") || !strings.Contains(errs.String(), tail) {
		t.Fatalf("stderr %q: want prefix \"job: read \" naming ...%s", errs.String(), tail)
	}
}

// Row 8: Trace's own query check.
func TestOWTraceQueryCheck(t *testing.T) {
	home := t.TempDir()
	_, err := Trace(ownerResolver(home), "brain:home/x", "zz", "")
	if want := "--sha256 requires a 64-character hexadecimal digest"; err == nil || err.Error() != want {
		t.Fatalf("err %v want %q", err, want)
	}
}

// Row 9: a folder that is not a task, beside the task folders, is skipped.
func TestOWTraceSkipsNonTaskFolder(t *testing.T) {
	home := owTraceHome(t)
	notes := filepath.Join(filepath.Dir(traceTask(t, home, "alpha", "T-01")), "notes")
	if err := os.MkdirAll(notes, 0755); err != nil {
		t.Fatal(err)
	}
	rows := traceTestRows(t, home, "brain:home/x")
	if len(rows) != 1 || rows[0]["task"] != "T-01" || rows[0]["job"] != "J-1" {
		t.Fatalf("rows: %+v", rows)
	}
}

// Row 10: the trace verb's own writers.
func TestOWTraceWritersFail(t *testing.T) {
	home := owTraceHome(t)
	healthy := func(args ...string) (int, string) {
		out, errs := &bytes.Buffer{}, &bytes.Buffer{}
		code := Run(append([]string{"trace"}, args...), Env{Resolver: ownerResolver(home), GridHome: home, Stdout: out, Stderr: errs})
		return code, out.String()
	}
	if code, out := healthy("brain:home/x", "--json"); code != 0 || !strings.Contains(out, `"job":"J-1"`) {
		t.Fatalf("json control: exit %d out %s", code, out)
	}
	if code, out := healthy("brain:home/x"); code != 0 || !strings.Contains(out, `"J-1"`) {
		t.Fatalf("plain control: exit %d out %s", code, out)
	}
	if code, out := healthy("-h"); code != 0 || out != traceUsage {
		t.Fatalf("help control: exit %d out %s", code, out)
	}
	for _, tc := range []struct {
		name   string
		args   []string
		stderr bool
	}{
		{"json", []string{"trace", "brain:home/x", "--json"}, false},
		{"plain", []string{"trace", "brain:home/x"}, false},
		{"help", []string{"trace", "-h"}, false},
		{"error line", []string{"trace", "--bogus"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := Env{Resolver: ownerResolver(home), GridHome: home, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
			if tc.stderr {
				e.Stderr = owClosedWriter{}
			} else {
				e.Stdout = owClosedWriter{}
			}
			if got := Run(tc.args, e); got != traceExitFailed {
				t.Fatalf("%v: exit %d want %d", tc.args, got, traceExitFailed)
			}
		})
	}
}
