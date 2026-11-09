package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowShowPersonalVisibility(t *testing.T) {
	for _, name := range []string{"personal", "Personal"} {
		for _, role := range []string{"member", "guest", "owner"} {
			for _, mode := range []string{"plain", "json"} {
				t.Run(name+"/"+role+"/"+mode, func(t *testing.T) {
					h := newHarness(t)
					h.env.Getenv = func(key string) string {
						return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "sam"}[key]
					}
					h.write(filepath.Join(h.gridHome, "workflows.md"), cliWorkflow)
					args := []string{"workflow", "show"}
					if mode == "json" {
						args = append(args, "--json")
					}
					if code := h.run(h.root, args...); code != 0 || h.errb.Len() != 0 {
						t.Fatalf("initial listing: exit %d: %s", code, h.errb)
					}
					publicOut := h.out.String()
					h.write(filepath.Join(h.gridHome, "projects", name, "project.md"), "---\nid: id-personal\n---\n")
					privateText := strings.ReplaceAll(strings.ReplaceAll(cliWorkflow, "Default", "Private"), "nothing else matches", "label private")
					h.write(filepath.Join(h.gridHome, "projects", name, "workflows.md"), privateText)
					missingCode := h.run(h.root, append(args, "--project", "unknown")...)
					missingOut, missingErr := h.out.String(), h.errb.String()
					if missingCode != 1 || missingOut != "" || missingErr == "" {
						t.Fatalf("unknown: exit %d stdout=%q stderr=%q", missingCode, missingOut, missingErr)
					}
					for _, selector := range []string{name, "id-personal"} {
						code := h.run(h.root, append(args, "--project", selector)...)
						if role == "owner" {
							if code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), "Private") || !strings.Contains(h.out.String(), name) {
								t.Errorf("%s: owner's workflow missing: exit %d stdout=%q stderr=%q", selector, code, h.out, h.errb)
							}
						} else if code != missingCode || h.out.String() != missingOut || h.errb.String() != missingErr {
							t.Errorf("%s: exit %d stdout=%q stderr=%q; unknown: exit %d stdout=%q stderr=%q", selector, code, h.out, h.errb, missingCode, missingOut, missingErr)
						}
					}
					code := h.run(h.root, args...)
					if role == "owner" {
						if code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), "Private") {
							t.Error("owner's listing omitted Personal")
						}
						return
					}
					if code != 0 || h.out.String() != publicOut || h.errb.Len() != 0 {
						t.Error("hidden Personal changed the public listing")
					}
					// Filter before reading even a damaged private project record.
					h.write(filepath.Join(h.gridHome, "projects", name, "project.md"), "---\nbroken: [\n---\n")
					if code := h.run(h.root, args...); code != 0 || h.out.String() != publicOut || h.errb.Len() != 0 {
						t.Error("damaged hidden Personal changed the public listing")
					}
				})
			}
		}
	}
}
