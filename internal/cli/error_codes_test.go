package cli

import (
	"encoding/json"
	"path/filepath"

	"github.com/gridconsole/grid-plugins/internal/project"
	"strings"
	"testing"
)

func TestStableErrorCodes(t *testing.T) {
	for _, tc := range []struct {
		name, code, stderr string
		args               []string
		exit               int
		person             bool
	}{
		{"move-source", "no_such_project", "grid: no such project\n", []string{"signal", "move", actID, "--project", "absent", "--to", "example"}, 3, false},
		{"move-target", "no_such_project", "grid: no such project\n", []string{"signal", "move", actID, "--project", "example", "--to", "absent"}, 3, false},
		{"move-note", "no_such_note", "grid: no such note\n", []string{"signal", "move", "S-01ARZ3NDEKTSV4RRFFQ69G5FAW", "--project", "example", "--to", "example"}, 3, false},
		{"act-note", "no_such_note", "grid: no such note\n", []string{"signal", "act", "S-01ARZ3NDEKTSV4RRFFQ69G5FAW", "--project", "example"}, 3, false},
		{"task", "no_such_task", "grid task show: task T-99 not found in the Grid home\n", []string{"task", "show", "T-99"}, 1, false},
		{"project", "no_such_project", "grid: project \"absent\" is not registered\n", []string{"where", "--project", "absent"}, 1, false},
		{"command-error", "no_such_task", "artifact: task T-99 not found in project example\n", []string{"artifact", "list", "--task", "T-99"}, 1, false},
		{"invalid-signal", "invalid_argument", "grid: move requires --to <project|->\n", []string{"signal", "move", actID}, 2, false},
		{"invalid-task", "invalid_argument", "grid task new: invalid size \"huge\": use auto, small, normal or large\n", []string{"task", "new", "Example", "--size", "huge"}, 2, false},
		{"invalid-flag", "invalid_argument", "", []string{"task", "show", "--unknown"}, 2, false},
		{"person-gate", "person_required", "grid task close: " + personHint + " (reason: agent_ancestor).\n", []string{"task", "close", "T-99"}, 3, true},
		{"person-check", "person_required", "grid check: " + personHint + " (reason: agent_ancestor).\n", []string{"person", "check"}, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			h.env.Getenv = func(string) string { return "" }
			if tc.person {
				h.env.Procs = testProcs("codex")
			}
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
			if !tc.person && h.out.Len() != 0 {
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
			if tc.person && (string(got["reason"]) != `"agent_ancestor"` || string(got["error"]) != `"person_required"`) {
				t.Fatalf("person refusal fields changed: %s", h.out)
			}
		})
	}
}
