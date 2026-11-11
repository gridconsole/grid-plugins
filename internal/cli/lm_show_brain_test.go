package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const lmSbCliTable = "| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n"

const lmSbCliWorkspace = "# Workflows\n\n## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" +
	lmSbCliTable + "| 1 | build | run | grid-builder |  |\n"

const lmSbCliNote = "---\ntype: workflow\nname: flow\nstatus: accepted\n---\n" +
	"# Workflows\n\n## Quick\napplies when: label quick\nreview rounds: 2\n\n" +
	lmSbCliTable + "| 1 | do-the-work | run | grid-builder |  |\n"

// lmSbCliHome lays out projects/alpha (ID id-alpha) and the workspace file,
// with the project brain workflow note when withNote.
func lmSbCliHome(t *testing.T, withNote bool) *harness {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	h.write(filepath.Join(h.gridHome, "workflows.md"), lmSbCliWorkspace)
	if withNote {
		h.write(filepath.Join(h.gridHome, "projects", "alpha", "knowledge", "decisions", "flow.md"), lmSbCliNote)
	}
	return h
}

// lmSbCliShow runs workflow show --json and returns the top-level keys and the
// templates as "file:name".
func lmSbCliShow(t *testing.T, h *harness, args ...string) ([]string, []string) {
	t.Helper()
	args = append([]string{"workflow", "show"}, append(args, "--json")...)
	if code := h.run(h.root, args...); code != 0 {
		t.Fatalf("%v: exit %d, stderr %q", args, code, h.errb)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(h.out.Bytes(), &top); err != nil {
		t.Fatalf("%v: %v in %s", args, err, h.out)
	}
	var keys []string
	for k := range top {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	var templates []struct{ Name, File string }
	if err := json.Unmarshal(top["templates"], &templates); err != nil {
		t.Fatalf("%v: templates %v", args, err)
	}
	var listed []string
	for _, tpl := range templates {
		listed = append(listed, tpl.File+":"+tpl.Name)
	}
	return keys, listed
}

// Row 4: grid workflow show --json lists the project note's template, by the
// project's name or ID, with no top-level key beyond files, templates, errors.
func TestLmSbCliShowListsNote(t *testing.T) {
	h := lmSbCliHome(t, true)
	for _, selector := range []string{"alpha", "id-alpha"} {
		keys, listed := lmSbCliShow(t, h, "--project", selector)
		if got := strings.Join(keys, ","); got != "errors,files,templates" {
			t.Fatalf("--project %s: top-level keys %s, want errors,files,templates", selector, got)
		}
		if got := strings.Join(listed, ","); got != "brain:project/flow:Quick,workflows.md:Standard" {
			t.Fatalf("--project %s: templates %s, want the note's Quick then the workspace Standard", selector, got)
		}
		if !strings.Contains(h.out.String(), `"file":"brain:project/flow"`) {
			t.Fatalf("--project %s: output lacks the note's file label: %s", selector, h.out)
		}
	}
	// The text output stays one block per workflows.md file.
	if code := h.run(h.root, "workflow", "show", "--project", "alpha"); code != 0 || strings.Contains(h.out.String(), "Quick") {
		t.Fatalf("plain show: exit %d, %q; want no brain template", code, h.out)
	}

	// With no workflow note in the home, templates are the workflows.md ones.
	h = lmSbCliHome(t, false)
	for _, args := range [][]string{nil, {"--project", "alpha"}} {
		keys, listed := lmSbCliShow(t, h, args...)
		if got := strings.Join(keys, ","); got != "errors,files,templates" {
			t.Fatalf("%v: top-level keys %s", args, got)
		}
		if got := strings.Join(listed, ","); got != "workflows.md:Standard" {
			t.Fatalf("%v: templates %s, want only workflows.md:Standard", args, got)
		}
	}
}
