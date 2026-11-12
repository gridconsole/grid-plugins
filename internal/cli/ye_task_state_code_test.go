package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// yeTsMove is one task write the state table refuses. The expected stderr is
// the text of task.NextState (write.go) behind the `grid: ` prefix of
// taskWriteResult, written out by hand so a change to either shows here.
type yeTsMove struct {
	name   string
	closed bool // the fixture is closed before the run (it starts queued)
	args   []string
	stderr string
}

var yeTsMoves = []yeTsMove{
	{"stop of a closed task", true, []string{"task", "stop", "T-01"},
		"grid: cannot stop task in current state \"closed\" to \"\"\n"},
	{"close of a closed task", true, []string{"task", "close", "T-01"},
		"grid: cannot close task in current state \"closed\" to \"\"\n"},
	{"reopen of a queued task", false, []string{"task", "reopen", "T-01"},
		"grid: cannot reopen task in current state \"queued\" to \"\"\n"},
	{"status of a closed task to running", true, []string{"task", "status", "T-01", "running"},
		"grid: cannot status task in current state \"closed\" to \"running\"\n"},
}

// yeTsHarness is a harness with one queued task T-01, closed when the move needs it.
func yeTsHarness(t *testing.T, closed bool) (*harness, task.Task) {
	t.Helper()
	h := newHarness(t)
	h.person()
	fixture := taskFixture(t, h)
	if closed {
		if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "close"}); err != nil {
			t.Fatal(err)
		}
	}
	return h, fixture
}

// yeTsTaskBytes is task.md of the fixture, to prove a refusal writes nothing.
func yeTsTaskBytes(t *testing.T, fixture task.Task) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixture.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// yeTsEnvelope is the stdout of commandFailure.finish (error_codes.go) for
// code and stderr: one object, a final newline, and no escaped HTML.
func yeTsEnvelope(code, stderr string) string {
	message := strings.ReplaceAll(strings.TrimSuffix(stderr, "\n"), `"`, `\"`)
	return `{"error":{"code":"` + code + `","message":"` + message + `"}}` + "\n"
}

// Row 1: with --json a task stop, close, reopen or status the state table
// refuses exits 1, keeps its stderr, and prints the task_state envelope.
func TestYeTsJSONEnvelope(t *testing.T) {
	for _, mv := range yeTsMoves {
		t.Run(mv.name, func(t *testing.T) {
			h, fixture := yeTsHarness(t, mv.closed)
			before := yeTsTaskBytes(t, fixture)
			code := h.run(h.home, append(mv.args[:len(mv.args):len(mv.args)], "--json")...)
			if code != 1 {
				t.Fatalf("exit %d, want 1; stdout %q stderr %q", code, h.out.String(), h.errb.String())
			}
			if got := h.errb.String(); got != mv.stderr {
				t.Fatalf("stderr %q, want %q", got, mv.stderr)
			}
			want := yeTsEnvelope("task_state", mv.stderr)
			if got := h.out.String(); got != want {
				t.Fatalf("stdout %q, want %q", got, want)
			}
			if !bytes.Equal(before, yeTsTaskBytes(t, fixture)) {
				t.Fatal("a refused move changed task.md")
			}
		})
	}
}

// Row 2: without --json the same refusals print nothing on stdout.
func TestYeTsPlainUnchanged(t *testing.T) {
	for _, mv := range yeTsMoves {
		t.Run(mv.name, func(t *testing.T) {
			h, _ := yeTsHarness(t, mv.closed)
			if code := h.run(h.home, mv.args...); code != 1 {
				t.Fatalf("exit %d, want 1; stdout %q stderr %q", code, h.out.String(), h.errb.String())
			}
			if h.out.Len() != 0 {
				t.Fatalf("stdout %q, want empty", h.out.String())
			}
			if got := h.errb.String(); got != mv.stderr {
				t.Fatalf("stderr %q, want %q", got, mv.stderr)
			}
		})
	}
}

// Row 1 (deliver): grid deliver --json refused by the state table prints the
// same envelope; its stderr keeps the `deliver: ` prefix and stays on stderr.
func TestYeTsDeliverEnvelope(t *testing.T) {
	h, path, _ := jwDaHarness(t)
	code := h.run(path, "deliver", "--json", "Result again")
	wantErr := "deliver: " + jwDaRefusal + "\n"
	if code != 1 || h.errb.String() != wantErr {
		t.Fatalf("exit %d stderr %q, want exit 1 stderr %q", code, h.errb.String(), wantErr)
	}
	if want, got := yeTsEnvelope("task_state", wantErr), h.out.String(); got != want {
		t.Fatalf("stdout %q, want %q", got, want)
	}
	if code := h.run(path, "deliver", "Result again"); code != 1 || h.out.Len() != 0 || h.errb.String() != wantErr {
		t.Fatalf("plain: exit %d stdout %q stderr %q", code, h.out.String(), h.errb.String())
	}
}

// Row 3: the other exits of taskWriteResult and the decline path print what
// they printed before the code existed: nothing on stdout, with or without --json.
func TestYeTsOtherPathsUnchanged(t *testing.T) {
	t.Run("approve without a plan Job", func(t *testing.T) {
		h, _ := yeTsHarness(t, false)
		for _, args := range [][]string{{"approve", "T-01"}, {"approve", "T-01", "--json"}} {
			if code := h.run(h.home, args...); code != 1 {
				t.Fatalf("%v: exit %d, want 1; stdout %q stderr %q", args, code, h.out.String(), h.errb.String())
			}
			if h.out.Len() != 0 || !strings.HasPrefix(h.errb.String(), "grid: ") {
				t.Fatalf("%v: stdout %q stderr %q, want empty stdout and a grid: line", args, h.out.String(), h.errb.String())
			}
		}
	})
	t.Run("decline of a closed task", func(t *testing.T) {
		h, _ := yeTsHarness(t, true)
		args := []string{"task", "decline", "T-01", "--reason", "Not now", "--json"}
		// A decline refused as ErrEditRejected exits 3 and records no code.
		if code := h.run(h.home, args...); code != 3 {
			t.Fatalf("exit %d, want 3; stdout %q stderr %q", code, h.out.String(), h.errb.String())
		}
		if h.out.Len() != 0 || !strings.Contains(h.errb.String(), "is already closed") {
			t.Fatalf("stdout %q stderr %q, want empty stdout and the closed refusal", h.out.String(), h.errb.String())
		}
	})
}

// The sentinel NextState wraps keeps the refusal text exactly as it was, and an
// allowed move is not wrapped.
func TestYeTsNextStateSentinel(t *testing.T) {
	_, err := task.NextState("closed", "close", "")
	if !errors.Is(err, task.ErrStateMove) {
		t.Fatalf("NextState error %v is not task.ErrStateMove", err)
	}
	if want := `cannot close task in current state "closed" to ""`; err.Error() != want {
		t.Fatalf("text %q, want %q", err.Error(), want)
	}
	if _, err := task.NextState("queued", "status", "running"); err != nil {
		t.Fatalf("an allowed move failed: %v", err)
	}
}
