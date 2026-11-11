package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func personFixture(t *testing.T, h *harness) {
	t.Helper()
	h.write(filepath.Join(h.gridHome, "knowledge", "people", "reviewer.md"), "---\nhandle: reviewer\nname: Example Reviewer\nrole: member\nlinks:\n  - to: release\n    rel: approves\n    summary: Release review\n    why: Reviews release changes\n---\nReview notes.\n")
	h.write(filepath.Join(h.gridHome, "knowledge", "people", "decider.md"), "---\nhandle: decider\nname: Example Decider\nrole: owner\nlinks:\n  - to: release\n    rel: decides\n    summary: Release decision\n    why: Decides release readiness\n---\n")
	legacy, err := os.ReadFile(filepath.Join("..", "people", "testdata", "legacy.md"))
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(h.gridHome, "knowledge", "people", "legacy.md"), string(legacy))
}

// Snapshot all paths and file bytes, including directories, so a read cannot
// quietly initialize local state, indexes, locks, or task records either.
func personSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			snapshot[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			snapshot[path] = "file:" + string(data)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func TestPersonGoldenAndReadOnly(t *testing.T) {
	h := newHarness(t)
	personFixture(t, h)
	task := taskFixture(t, h)
	before := personSnapshot(t, h.gridHome)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"person-list-json", []string{"list", "--json"}},
		{"person-show-json", []string{"show", "reviewer", "--json"}},
		{"person-approvers-json", []string{"approvers", "--area", "release", "--json"}},
		{"person-approvers-default-json", []string{"approvers", "--area", "unmatched", "--task", task.Path, "--json"}},
		{"person-approvers-default-json", []string{"approvers", "--area", "Release", "--task", task.ID, "--project", "personal", "--json"}},
	} {
		if code := h.run(h.home, append([]string{"person"}, tc.args...)...); code != 0 {
			t.Fatalf("%v: exit %d: %s", tc.args, code, h.errb)
		}
		golden.Assert(t, tc.name, h.out.String())
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"list"}, 0},
		{[]string{"show", "reviewer"}, 0},
		{[]string{"show", "absent", "--json"}, 1},
		{[]string{"show", "legacy", "--json"}, 1},
		{[]string{"approvers", "--area", "release"}, 0},
		{[]string{"approvers", "--area", "release/child", "--json"}, 2},
		{[]string{"approvers", "--area", "release", "--task", "T-99", "--project", "personal"}, 1},
		{[]string{"approvers"}, 2},
		{[]string{"approvers", "--area", " "}, 2},
		{[]string{"show"}, 2},
		{[]string{"list", "extra"}, 2},
		{[]string{"list", "--task", task.Path}, 2},
		{[]string{"list", "--unknown"}, 2},
		{[]string{"list", "--project", "unknown"}, 1},
		{[]string{"add", "someone"}, 2},
		{[]string{"--help"}, 0},
		{[]string{"show", "--help"}, 0},
	} {
		if code := h.run(h.home, append([]string{"person"}, tc.args...)...); code != tc.code {
			t.Fatalf("%v: exit %d, want %d: %s", tc.args, code, tc.code, h.errb)
		}
		if tc.code != 0 {
			code := ""
			if tc.code == 2 && jsonRequested(tc.args) {
				code = CodeInvalidArgument
			}
			assertErrorOutput(t, h.out.String(), h.errb.String(), code)
		}
	}
	if after := personSnapshot(t, h.gridHome); !reflect.DeepEqual(before, after) {
		t.Fatal("person commands changed the Grid home")
	}
}

func TestPersonProjectAndTaskSelection(t *testing.T) {
	h := newHarness(t)
	personFixture(t, h)
	h.record(project.Record{Name: "example", ID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"})
	projectRoot := filepath.Join(h.gridHome, "projects", "example")
	// Both home grants are overridden by guests in the project.
	for _, handle := range []string{"reviewer", "decider"} {
		h.write(filepath.Join(projectRoot, "knowledge", "people", handle+".md"), "---\nhandle: "+handle+"\nname: Project Guest\nrole: guest\nlinks:\n  - to: release\n    rel: approves\n---\n")
	}
	taskPath := filepath.Join(projectRoot, "tasks", "T-02-review")
	h.write(filepath.Join(taskPath, "task.md"), "---\nid: T-02\ntitle: Example task\nproject: example\nowner: task-owner\nstate: open\ncreated: 2026-01-02T03:04:05Z\n---\n")
	h.write(filepath.Join(taskPath, "intent.md"), "Review the change.\n")
	before := personSnapshot(t, h.gridHome)
	for _, selector := range []string{"example", "01ARZ3NDEKTSV4RRFFQ69G5FAV"} {
		if code := h.run(h.home, "person", "show", "--json", "reviewer", "--project", selector); code != 0 || !strings.Contains(h.out.String(), `"role":"guest"`) {
			t.Fatalf("project override: %d %s %s", code, h.out, h.errb)
		}
		if code := h.run(h.home, "person", "approvers", "--area", "release", "--project", selector, "--json"); code != 2 {
			t.Fatalf("guest grants leaked from home: %d %s %s", code, h.out, h.errb)
		}
		if code := h.run(h.home, "person", "approvers", "--area", "release", "--task", "T-02", "--project", selector, "--json"); code != 0 || h.out.String() != "{\"approvers\":[\"task-owner\"],\"source\":\"default\"}\n" {
			t.Fatalf("task owner: %d %s %s", code, h.out, h.errb)
		}
	}
	if code := h.run(h.home, "person", "approvers", "--area", "release", "--task", taskPath, "--json"); code != 0 || !strings.Contains(h.out.String(), `"source":"default"`) {
		t.Fatalf("task project inference: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.home, "person", "approvers", "--area", "release", "--task", taskPath, "--project", "personal"); code != 1 {
		t.Fatalf("mismatched project accepted: %d %s", code, h.errb)
	}
	if after := personSnapshot(t, h.gridHome); !reflect.DeepEqual(before, after) {
		t.Fatal("project person commands changed the Grid home")
	}
}

func TestPersonLegacyTaskWithoutOwner(t *testing.T) {
	h := newHarness(t)
	personFixture(t, h)
	task := taskFixture(t, h)
	old, err := os.ReadFile("testdata/person-task-legacy.md")
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(task.Path, "task.md"), string(old))
	before := personSnapshot(t, h.gridHome)
	if code := h.run(h.home, "person", "approvers", "--area", "release", "--task", task.Path, "--json"); code != 0 {
		t.Fatalf("legacy task should work with a people match: %d %s", code, h.errb)
	}
	golden.Assert(t, "person-approvers-json", h.out.String())
	if code := h.run(h.home, "person", "approvers", "--area", "missing", "--task", task.Path, "--json"); code != 2 || !strings.Contains(h.errb.String(), "owner in task.md") {
		t.Fatalf("missing legacy owner: %d %s %s", code, h.out, h.errb)
	}
	if after := personSnapshot(t, h.gridHome); !reflect.DeepEqual(before, after) {
		t.Fatal("legacy task or brain changed")
	}
}

func TestPersonEmptyCacheAndOwnerMatch(t *testing.T) {
	h := newHarness(t)
	before := personSnapshot(t, h.gridHome)
	if code := h.run(h.home, "person", "show", "nobody", "--json"); code != 1 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "nobody") {
		t.Fatalf("empty cache: %d %s %s", code, h.out, h.errb)
	}
	if after := personSnapshot(t, h.gridHome); !reflect.DeepEqual(before, after) {
		t.Fatal("empty cache read initialized files")
	}
	task := taskFixture(t, h)
	h.write(filepath.Join(h.gridHome, "knowledge", "people", task.Owner+".md"), "---\nhandle: "+task.Owner+"\nname: Task Owner\nrole: owner\nlinks:\n  - to: release\n    rel: approves\n---\n")
	if code := h.run(h.home, "person", "approvers", "--area", "release", "--task", task.Path, "--json"); code != 0 || !strings.Contains(h.out.String(), `"source":"people"`) {
		t.Fatalf("matching owner mislabeled as default: %d %s %s", code, h.out, h.errb)
	}
}
