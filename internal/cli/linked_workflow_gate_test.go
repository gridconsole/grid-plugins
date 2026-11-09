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

// A linked workflows.md may be forged (here it would drop every person verb),
// so a person verb refuses with linked_file for a person and an agent alike,
// and changes nothing. A verb outside the floor and the default policy runs.
func TestLinkedWorkflowsFileRefusesPersonVerbs(t *testing.T) {
	for _, scope := range []string{"workspace", "project"} {
		for _, caller := range []string{"person", "agent", "agent note"} {
			t.Run(scope+"/"+caller, func(t *testing.T) {
				h, _, _ := actFixture(t, "new")
				h.env.Getenv = func(string) string { return "" }
				if caller != "person" {
					h.env.Procs = testProcs("codex")
				}
				target := filepath.Join(h.gridHome, "workflows.md")
				if scope == "project" {
					target = filepath.Join(h.gridHome, "projects", "example", "workflows.md")
				}
				h.write(target, "---\nperson_verbs: []\n---\n")
				cwd := filepath.Join(h.root, "work")
				h.mkdir(cwd)
				h.record(project.Record{Name: "example", ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV", Folders: []project.Folder{{Machine: h.env.Machine, Path: cwd}}})
				if err := os.Link(target, filepath.Join(t.TempDir(), "t")); err != nil {
					t.Fatal(err)
				}
				act := "turn"
				if caller == "agent note" {
					act = "note"
				}
				before := fileBytes(t, h.gridHome)
				code := h.run(cwd, "signal", "act", actID, "--act", act, "--json")
				if caller == "agent note" {
					if code != 0 {
						t.Fatalf("a verb outside the policy refused: exit %d %s %s", code, h.out, h.errb)
					}
					return
				}
				if code != 1 {
					t.Fatalf("exit %d; want 1 (linked_file): %s %s", code, h.out, h.errb)
				}
				var got struct {
					Error struct {
						Code    string `json:"code"`
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Error.Code != CodeLinkedFile {
					t.Fatalf("refusal JSON = %s (%v); want error.code linked_file", h.out, err)
				}
				if !strings.Contains(h.errb.String(), "workflows.md") || strings.Contains(h.errb.String(), h.root) {
					t.Errorf("message must name workflows.md without a machine path: %s", h.errb)
				}
				if !reflect.DeepEqual(before, fileBytes(t, h.gridHome)) {
					t.Error("the refusal changed files")
				}
			})
		}
	}
}
