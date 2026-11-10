package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/signal"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestStableErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name, code, stderr string
		args               []string
		exit               int
	}{
		{"move-source", "no_such_project", "grid: no such project\n", []string{"signal", "move", actID, "--project", "absent", "--to", "example"}, 3},
		{"move-target", "no_such_project", "grid: no such project\n", []string{"signal", "move", actID, "--project", "example", "--to", "absent"}, 3},
		{"move-note", "no_such_note", "grid: no such note\n", []string{"signal", "move", "S-01ARZ3NDEKTSV4RRFFQ69G5FAW", "--project", "example", "--to", "example"}, 3},
		{"act-note", "no_such_note", "grid: no such note\n", []string{"signal", "act", "S-01ARZ3NDEKTSV4RRFFQ69G5FAW", "--project", "example"}, 3},
		{"task", "no_such_task", "grid task show: task T-99 not found in the Grid home\n", []string{"task", "show", "T-99"}, 1},
		{"project", "no_such_project", "grid: project \"absent\" is not registered\n", []string{"where", "--project", "absent"}, 1},
		{"command-error", "no_such_task", "artifact: task T-99 not found in project example\n", []string{"artifact", "list", "--task", "T-99"}, 1},
		{"invalid-signal", "invalid_argument", "grid: move requires --to <project|->\n", []string{"signal", "move", actID}, 2},
		{"invalid-task", "invalid_argument", "grid task new: invalid size \"huge\": use auto, small, normal or large\n", []string{"task", "new", "Example", "--size", "huge"}, 2},
		{"invalid-flag", "invalid_argument", "", []string{"task", "show", "--unknown"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			h.env.Getenv = func(string) string { return "" }
			cwd := filepath.Join(h.root, "checkout")
			h.mkdir(cwd)
			h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
			if exit := h.run(cwd, tc.args...); exit != tc.exit {
				t.Fatalf("plain exit = %d, want %d: %s", exit, tc.exit, h.errb)
			}
			stderr := h.errb.String()
			if tc.stderr != "" && stderr != tc.stderr {
				t.Fatalf("stderr = %q, want %q", stderr, tc.stderr)
			}
			if h.out.Len() != 0 {
				t.Fatalf("plain stdout = %q", h.out)
			}
			args := append(append([]string{}, tc.args...), "--json")
			if exit := h.run(cwd, args...); exit != tc.exit || h.errb.String() != stderr {
				t.Fatalf("JSON changed exit/stderr: %d, %q; want %d, %q", exit, h.errb, tc.exit, stderr)
			}
			var got map[string]json.RawMessage
			if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
				t.Fatalf("failure JSON: %v: %s", err, h.out)
			}
			var code, message string
			_ = json.Unmarshal(got["code"], &code)
			if code == "" {
				var failure struct{ Code, Message string }
				if err := json.Unmarshal(got["error"], &failure); err != nil {
					t.Fatalf("error object: %v: %s", err, h.out)
				}
				code, message = failure.Code, failure.Message
				if message != strings.TrimSuffix(stderr, "\n") {
					t.Fatalf("message = %q, stderr = %q", message, stderr)
				}
			}
			if code != tc.code {
				t.Fatalf("code = %q, want %q: %s", code, tc.code, h.out)
			}
		})
	}
}

// assertErrorOutput checks new envelopes; an empty code retains empty stdout.
func assertErrorOutput(t *testing.T, output, stderr, code string) {
	t.Helper()
	if code == "" {
		if output != "" {
			t.Fatalf("unexpected stdout: %s", output)
		}
		return
	}
	var got map[string]map[string]string
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("error JSON: %v: %s", err, output)
	}
	if len(got) != 1 || len(got["error"]) != 2 || got["error"]["code"] != code || got["error"]["message"] != strings.TrimSuffix(stderr, "\n") {
		t.Fatalf("error JSON = %s, want code %q and stderr message %q", output, code, stderr)
	}
}

func TestErrorCodeIgnoresWording(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
	}{
		{&where.LookupError{Kind: where.ErrProjectUnregistered, Message: "changed project wording"}, CodeNoSuchProject},
		{&where.LookupError{Kind: where.ErrTaskNotFound, Message: "changed task wording"}, CodeNoSuchTask},
		{&signal.RefusedError{Cause: signal.ErrNoNote, Message: "changed note wording"}, CodeNoSuchNote},
		{&signal.BadInputError{Message: "changed argument wording"}, CodeInvalidArgument},
		{errors.New("no such project"), ""},
		{errors.New("no such note"), ""},
	} {
		if got := errorCode(fmt.Errorf("context: %w", tc.err)); got != tc.code {
			t.Fatalf("code = %q, want %q", got, tc.code)
		}
	}
}

func TestErrorJSONFlagSemantics(t *testing.T) {
	for _, args := range [][]string{
		{"task", "show", "T-99", "--json=false"},
		{"task", "show", "T-99", "--json", "--json=false"},
		{"task", "show", "--", "--json"},
		{"task", "new", "Example", "--workflow", "--json", "--size", "huge"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, args...); code == 0 || h.errb.Len() == 0 {
				t.Fatalf("expected failure: %d %s", code, h.errb)
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), "")
		})
	}
	for _, args := range [][]string{
		{"task", "show", "T-99", "-json"},
		{"task", "show", "T-99", "--json=true"},
		{"task", "show", "T-99", "--json=false", "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, args...); code != 1 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), CodeNoSuchTask)
		})
	}
}
