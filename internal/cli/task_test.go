package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

func taskFixture(t *testing.T, h *harness) task.Task {
	t.Helper()
	s := task.Store{Resolver: resolver(h.env), Now: func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}}
	got, err := s.New(task.NewOptions{Cwd: h.home, Words: "Fix the label\n\nKeep these exact words.\n", Size: "small", Workflow: "Check and ship"})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestTaskGolden(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"task-new-json", []string{"task", "new", "Fix the label", "--json"}},
		{"task-new-plain", []string{"task", "new", "Fix the label"}},
		{"task-show-json", []string{"task", "show", "T-01", "--json"}},
		{"task-show-plain", []string{"task", "show", "T-01"}},
		{"task-list-json", []string{"task", "list", "--json"}},
		{"task-list-plain", []string{"task", "list"}},
		{"task-list-filtered-json", []string{"task", "list", "--state", "closed", "--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if tc.args[1] != "new" {
				taskFixture(t, h)
			}
			if code := h.run(h.home, tc.args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, tc.name, h.norm(h.out.String()))
		})
	}
}

func TestTaskFlagsAndIntent(t *testing.T) {
	for _, tc := range []struct {
		name, words, size, workflow string
		args                        []string
	}{
		{"flags after words", "  Keep: my words\n\nexactly. ", "large", "Review and publish", []string{"  Keep: my words\n\nexactly. ", "--size", "large", "--workflow", "Review and publish", "--json"}},
		{"flags before words", "Example task", "small", "", []string{"--size", "small", "--json", "Example task"}},
		{"mixed flags", "Example task", "normal", "docs", []string{"--size=normal", "Example task", "--workflow=docs", "--json"}},
		{"protected words", "--fix this", "auto", "", []string{"--json", "--", "--fix this"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, append([]string{"task", "new"}, tc.args...)...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var output struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			got, err := task.Read(output.Path)
			if err != nil || got.ID != output.ID || got.Intent != tc.words || got.Size != tc.size || got.Workflow != tc.workflow {
				t.Fatalf("got %+v, error %v", got, err)
			}
			if code := h.run(filepath.Join(output.Path, "artifacts"), "task", "show", "--json"); code != 0 {
				t.Fatalf("show inside task: %s", h.errb)
			}
			var shown task.Task
			if err := json.Unmarshal(h.out.Bytes(), &shown); err != nil || shown.Intent != tc.words {
				t.Fatalf("show %+v: %v", shown, err)
			}
		})
	}
}

func TestTaskProjectAndPersonalSelection(t *testing.T) {
	for _, selection := range []string{"inferred", "explicit name", "explicit id", "personal"} {
		t.Run(selection, func(t *testing.T) {
			h := newHarness(t)
			cwd := filepath.Join(h.root, "work", "example")
			h.mkdir(cwd)
			testgit.Init(t, cwd, "https://example.com/org/example.git", true)
			args := []string{"task", "new", "Add search", "--json"}
			wantProject := "example"
			switch selection {
			case "explicit name", "explicit id":
				h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
				arg := "example"
				if selection == "explicit id" {
					arg = "01EXAMPLE"
				}
				args = append(args, "--project", arg)
				cwd = h.home
			case "personal":
				args = append(args, "--project", "Personal")
				wantProject = "personal"
			}
			if code := h.run(cwd, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var output struct{ ID, Path string }
			if err := json.Unmarshal(h.out.Bytes(), &output); err != nil {
				t.Fatal(err)
			}
			got, err := task.Read(output.Path)
			if err != nil || got.Project != wantProject {
				t.Fatalf("task %+v, %v", got, err)
			}
			if code := h.run(h.home, "task", "list", "--project", wantProject, "--state", "queued", "--json"); code != 0 {
				t.Fatalf("list exit %d: %s", code, h.errb)
			}
			var list []task.Task
			if err := json.Unmarshal(h.out.Bytes(), &list); err != nil || len(list) != 1 || list[0].ID != output.ID {
				t.Fatalf("list %+v, %v", list, err)
			}
			if code := h.run(output.Path, "task", "show", "--json"); code != 0 {
				t.Fatalf("show exit %d: %s", code, h.errb)
			}
		})
	}
}

func TestTaskUsageAndFailures(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"task"}, 2},
		{[]string{"task", "unknown"}, 2},
		{[]string{"task", "new"}, 2},
		{[]string{"task", "new", "one", "two"}, 2},
		{[]string{"task", "new", " "}, 2},
		{[]string{"task", "new", "Example", "--size", "tiny"}, 2},
		{[]string{"task", "new", "Example", "--workflow"}, 2},
		{[]string{"task", "new", "Example", "--unknown"}, 2},
		{[]string{"task", "new", "Example", "--project", "missing"}, 1},
		{[]string{"task", "show"}, 1},
		{[]string{"task", "show", "T-99", "--json"}, 1},
		{[]string{"task", "show", "T-01", "T-02"}, 2},
		{[]string{"task", "list", "extra"}, 2},
		{[]string{"task", "list", "--project", "missing"}, 1},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, tc.args...); code != tc.code || h.out.Len() != 0 || h.errb.Len() == 0 {
				t.Fatalf("exit %d; stdout %q; stderr %q", code, h.out, h.errb)
			}
		})
	}
}

func TestTaskReadsDoNotWrite(t *testing.T) {
	h := newHarness(t)
	h.env.GridHome = filepath.Join(h.home, ".absent")
	if code := h.run(h.home, "task", "list", "--json"); code != 0 || h.out.String() != "[]\n" {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.home, "task", "show", "T-01"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
		t.Fatalf("reads created Grid home: %v", err)
	}
	h.env.GridHome = ""
	for _, args := range [][]string{{"task", "new", "Example"}, {"task", "show"}, {"task", "list"}} {
		if code := h.run(h.home, args...); code != 1 || !strings.Contains(h.errb.String(), "GRID_HOME") {
			t.Fatalf("%v: exit %d, %s", args, code, h.errb)
		}
	}
}
