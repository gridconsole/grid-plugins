package cli

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestSignalMoveInboxPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, workflow string
		invalid        bool
	}{
		{"missing", "", false},
		{"legacy", "# Workflows\n", false},
		{"unset", "---\nperson_verbs: []\n---\n# Workflows\n", false},
		{"personal", "---\ninbox_workspace: personal\n---\n# Workflows\n", false},
		{"shared", "---\ninbox_workspace: shared\n---\n# Workflows\n", true},
		{"unparseable", "---\ninbox_workspace: [\n---\n", true},
		{"unclosed", "---\ninbox_workspace: personal\n", true},
		{"null", "---\ninbox_workspace: null\n---\n", true},
		{"list", "---\ninbox_workspace: [personal]\n---\n", true},
		{"duplicate", "---\ninbox_workspace: personal\ninbox_workspace: shared\n---\n", true},
	} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", tc.name, asJSON), func(t *testing.T) {
				h, _, _ := actFixture(t, "placed")
				h.record(project.Record{Name: "personal"})
				h.env.Getenv = func(string) string { return "" }
				if tc.workflow != "" {
					h.write(filepath.Join(h.gridHome, "workflows.md"), tc.workflow)
				}
				// Inbox policy is workspace-only, even when the project sets a different value.
				h.write(filepath.Join(h.gridHome, "projects", "example", "workflows.md"), "---\ninbox_workspace: shared\n---\n")
				before := moveTree(t, h.gridHome)
				args := []string{"signal", "move", actID, "--project", "example", "--to", "-"}
				if asJSON {
					args = append(args, "--json")
				}
				code := h.run(h.home, args...)
				if tc.invalid {
					if code != 1 || h.errb.String() != "grid signal move: invalid inbox_workspace in workspace workflow (only personal is supported)\n" {
						t.Errorf("exit %d, stderr %q", code, h.errb.String())
					}
					want := ""
					if asJSON {
						want = "{\"error\":\"invalid_policy\",\"key\":\"inbox_workspace\",\"message\":\"invalid inbox_workspace in workspace workflow (only personal is supported)\",\"code\":\"invalid_policy\"}\n"
					}
					if h.out.String() != want {
						t.Errorf("stdout %q; want %q", h.out.String(), want)
					}
					if before != moveTree(t, h.gridHome) {
						t.Error("invalid policy changed tree")
					}
				} else if code != 0 || h.errb.Len() != 0 {
					t.Fatalf("exit %d: %s", code, h.errb)
				} else {
					_, n := actRead(t, filepath.Join(h.gridHome, "projects", "personal", "notes", actID+".md"))
					if n["status"] != "new" {
						t.Fatalf("status %q", n["status"])
					}
				}
			})
		}
	}
}

func TestSignalMoveNamedIgnoresInboxPolicy(t *testing.T) {
	for _, workflow := range []string{"---\ninbox_workspace: shared\n---\n", "---\ninbox_workspace: [\n---\n"} {
		h, _, _ := actFixture(t, "new")
		h.record(project.Record{Name: "second"})
		h.env.Getenv = func(string) string { return "" }
		h.write(filepath.Join(h.gridHome, "workflows.md"), workflow)
		// This policy prevents the separate person gate from needing workspace inheritance.
		h.write(filepath.Join(h.gridHome, "projects", "example", "workflows.md"), "---\nperson_verbs: []\n---\n")
		if code := h.run(h.home, "signal", "move", actID, "--project", "example", "--to", "second"); code != 0 || h.errb.Len() != 0 {
			t.Fatalf("named move exit %d: %s", code, h.errb)
		}
	}
}

func TestSignalMoveInboxRetainsPersonGate(t *testing.T) {
	for _, target := range []string{"-", "personal"} {
		h, _, _ := actFixture(t, "new")
		h.record(project.Record{Name: "personal"})
		h.env.Getenv = func(string) string { return "" }
		h.env.Procs = testProcs("codex")
		h.write(filepath.Join(h.gridHome, "workflows.md"), "---\ninbox_workspace: personal\nperson_verbs: [signal move]\n---\n")
		before := moveTree(t, h.gridHome)
		if code := h.run(h.home, "signal", "move", actID, "--project", "example", "--to", target, "--json"); code != 3 || !strings.Contains(h.out.String(), `"error":"person_required"`) {
			t.Fatalf("gate exit %d: %s %s", code, h.out, h.errb)
		}
		if before != moveTree(t, h.gridHome) {
			t.Fatal("person refusal changed tree")
		}
	}
}
