package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// linkedFileRefusal checks a linked_file refusal: exit 1, the coded JSON
// envelope, a message that names the file without a machine path, and no file
// under root changed.
func linkedFileRefusal(t *testing.T, h *harness, code int, file string, root string, before map[string][32]byte) {
	t.Helper()
	if code != 1 {
		t.Fatalf("exit %d; want 1 (linked_file): %s %s", code, h.out, h.errb)
	}
	var got struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Error.Code != CodeLinkedFile {
		t.Fatalf("refusal JSON = %s (%v); want error.code linked_file", h.out, err)
	}
	if !strings.Contains(h.errb.String(), file) || !strings.Contains(h.errb.String(), "linked_file") || strings.Contains(h.errb.String(), h.root) {
		t.Errorf("message must name %s and linked_file without a machine path: %s", file, h.errb)
	}
	if !reflect.DeepEqual(before, fileBytes(t, root)) {
		t.Error("the refusal changed files")
	}
}

// The FP1R review probe as rows. The real workflows.md adds signal act note to
// the person verbs. A second link that nobody wrote through used to drop that
// verb, so an agent ran it (exit 0, a task made from the signal). The gate now
// refuses every verb whose policy file is linked, before Required, for an agent
// and a person alike. Unlinked, nothing changes: the agent needs a person.
func TestLinkedPolicyRefusesTheVerbItAdds(t *testing.T) {
	for _, scope := range []string{"workspace", "project"} {
		for _, linked := range []bool{false, true} {
			for _, caller := range []string{"agent", "person"} {
				name := scope + "/unlinked/" + caller
				if linked {
					name = scope + "/linked/" + caller
				}
				t.Run(name, func(t *testing.T) {
					h, _, _ := actFixture(t, "new")
					h.env.Getenv = func(string) string { return "" }
					if caller == "agent" {
						h.env.Procs = testProcs("codex")
					}
					target := filepath.Join(h.gridHome, "workflows.md")
					if scope == "project" {
						target = filepath.Join(h.gridHome, "projects", "example", "workflows.md")
					}
					h.write(target, "---\nperson_verbs: [\"signal act turn\", \"signal act dismiss\", \"signal act snooze\", \"signal act note\"]\n---\n")
					cwd := filepath.Join(h.root, "work")
					h.mkdir(cwd)
					h.record(project.Record{Name: "example", ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
					if linked {
						if err := os.Link(target, filepath.Join(t.TempDir(), "t")); err != nil {
							t.Fatal(err)
						}
					}
					before := fileBytes(t, h.gridHome)
					code := h.run(cwd, "signal", "act", actID, "--act", "note", "--json")
					switch {
					case linked:
						linkedFileRefusal(t, h, code, "workflows.md", h.gridHome, before)
					case caller == "agent":
						var got map[string]any
						if code != 3 || json.Unmarshal(h.out.Bytes(), &got) != nil || got["error"] != "person_required" || got["code"] != CodePersonRequired {
							t.Fatalf("unlinked agent: exit %d %s %s; want 3 person_required", code, h.out, h.errb)
						}
					default:
						if code != 0 {
							t.Fatalf("unlinked person: exit %d %s %s; want 0", code, h.out, h.errb)
						}
					}
				})
			}
		}
	}
}

// The task-local workflow.md gets the same check as workflows.md: when it is
// linked, every verb run in that task's context refuses with linked_file, the
// verb it adds (task show) and a floor verb (task close) alike, for a person
// and an agent. Unlinked, task show still needs a person and a person runs it.
func TestLinkedTaskWorkflowFileRefusesVerbs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller string
		linked bool
		args   []string
		want   int
	}{
		{"unlinked/agent/task show", "agent", false, []string{"task", "show", "T-01", "--json"}, 3},
		{"unlinked/person/task show", "person", false, []string{"task", "show", "T-01", "--json"}, 0},
		{"linked/agent/task show", "agent", true, []string{"task", "show", "T-01", "--json"}, 1},
		{"linked/person/task show", "person", true, []string{"task", "show", "T-01", "--json"}, 1},
		{"linked/person/task close", "person", true, []string{"task", "close", "T-01", "--json"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(string) string { return "" }
			fixture := taskFixture(t, h)
			if tc.caller == "agent" {
				h.env.Procs = testProcs("codex")
			}
			policy := filepath.Join(fixture.Path, "workflow.md")
			h.write(policy, "---\nperson_verbs:\n  - task show\n---\n")
			if tc.linked {
				if err := os.Link(policy, filepath.Join(t.TempDir(), "t")); err != nil {
					t.Fatal(err)
				}
			}
			before := fileBytes(t, h.gridHome)
			code := h.run(fixture.Path, tc.args...)
			if tc.linked {
				linkedFileRefusal(t, h, code, "workflow.md", h.gridHome, before)
				return
			}
			if code != tc.want {
				t.Fatalf("exit %d; want %d: %s %s", code, tc.want, h.out, h.errb)
			}
			if tc.want == 3 && !strings.Contains(h.out.String(), "person_required") {
				t.Fatalf("want person_required: %s", h.out)
			}
		})
	}
}

// signal move --inbox reads the workspace workflows.md first. A linked one is
// refused as linked_file, not as an invalid inbox_workspace value, and nothing moves.
func TestSignalMoveInboxLinkedPolicy(t *testing.T) {
	h, _, _ := actFixture(t, "placed")
	h.record(project.Record{Name: "personal"})
	h.env.Getenv = func(string) string { return "" }
	target := filepath.Join(h.gridHome, "workflows.md")
	h.write(target, "---\ninbox_workspace: personal\n---\n# Workflows\n")
	if err := os.Link(target, filepath.Join(t.TempDir(), "t")); err != nil {
		t.Fatal(err)
	}
	before := fileBytes(t, h.gridHome)
	code := h.run(h.home, "signal", "move", actID, "--project", "example", "--to", "-", "--json")
	linkedFileRefusal(t, h, code, "workflows.md", h.gridHome, before)
	if !strings.HasPrefix(h.errb.String(), "grid signal move: linked_file: ") || strings.Contains(h.out.String(), "invalid_policy") {
		t.Errorf("want one linked_file diagnostic and no invalid_policy object: %s %s", h.out, h.errb)
	}
}
