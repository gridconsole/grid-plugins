package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func bypassFixture(t *testing.T) (*harness, string, string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	personal := taskFixture(t, h)
	alpha := filepath.Join(h.root, "alpha")
	h.mkdir(alpha)
	h.record(project.Record{Name: "alpha", ID: "01ALPHA", Folders: []project.Folder{{Machine: h.env.Machine, Path: alpha}}})
	if _, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: alpha, Words: "Public intent", Project: "alpha"}); err != nil {
		t.Fatal(err)
	}
	h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "gus"}[k] }
	return h, alpha, personal.Path
}

func TestPersonalBypassTaskCommands(t *testing.T) {
	for _, verb := range []string{"show", "stop"} {
		t.Run(verb, func(t *testing.T) {
			h, cwd, personal := bypassFixture(t)
			before := bypassTree(t, personal)
			run := func(id string) (int, string) {
				args := []string{"task", verb, id}
				if verb == "stop" {
					args = append(args, "--reason", "x")
				}
				code := h.run(cwd, args...)
				if h.out.Len() != 0 {
					t.Errorf("stdout = %q", h.out)
				}
				return code, h.errb.String()
			}
			code, stderr := run("T-01")
			missingCode, missingErr := run("T-99")
			if code != 1 || code != missingCode || stderr != missingErr {
				t.Errorf("hidden = (%d, %q), missing = (%d, %q)", code, stderr, missingCode, missingErr)
			}
			if !reflect.DeepEqual(before, bypassTree(t, personal)) {
				t.Error("Personal task changed")
			}
		})
	}
}

func TestPersonalBypassProjectCommands(t *testing.T) {
	h, cwd, personal := bypassFixture(t)
	code := h.run(cwd, "task", "list", "--project", "personal")
	stderr := h.errb.String()
	if code != 1 || h.out.Len() != 0 {
		t.Errorf("list: %d %s %s", code, h.out, h.errb)
	}
	missing := h.run(cwd, "task", "list", "--project", "nosuch")
	if code != missing || stderr != h.errb.String() {
		t.Errorf("list stderr %q != %q", stderr, h.errb)
	}
	before := bypassTree(t, filepath.Dir(personal))
	if code := h.run(cwd, "task", "new", "x", "--project", "PERSONAL"); code != 1 {
		t.Errorf("new: %d %s", code, h.errb)
	}
	if !reflect.DeepEqual(before, bypassTree(t, filepath.Dir(personal))) {
		t.Error("Personal tasks changed")
	}
	if code := h.run(cwd, "signal", "add", "--project", "personal", "--source", "test", "--title", "x"); code != 1 {
		t.Errorf("signal: %d %s", code, h.errb)
	}
	// A stored Personal entry must also be hidden from registry consumers.
	h.record(project.Record{Name: "personal", ID: "01PERSONAL"})
	if code := h.run(cwd, "project", "list", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	var entries []struct{ Name string }
	if err := json.Unmarshal(h.out.Bytes(), &entries); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name == "personal" {
			t.Error("Personal listed")
		}
	}
}

func TestPersonalBypassBrain(t *testing.T) {
	for _, verb := range []string{"show", "search", "keep", "decide"} {
		t.Run(verb, func(t *testing.T) {
			h, cwd, personal := bypassFixture(t)
			before := bypassTree(t, personal)
			run := func(id string) (int, string) {
				args := []string{"brain", verb, "--task", id}
				if verb == "decide" {
					args = []string{"brain", "decide", id, "P-1", "accept"}
				} else if verb != "keep" {
					args = append(args, "lesson")
				}
				code := h.run(cwd, args...)
				if h.out.Len() != 0 {
					t.Errorf("stdout = %q", h.out)
				}
				return code, h.errb.String()
			}
			code, stderr := run("T-01")
			missingCode, missingErr := run("T-99")
			if code != 1 || code != missingCode || stderr != missingErr {
				t.Errorf("hidden = (%d, %q), missing = (%d, %q)", code, stderr, missingCode, missingErr)
			}
			if !reflect.DeepEqual(before, bypassTree(t, personal)) {
				t.Error("Personal history or proposal changed")
			}
		})
	}
}

func TestPersonalBypassEnvironmentScan(t *testing.T) {
	h, cwd, personal := bypassFixture(t)
	// env up takes a Job id: the same Job is present only in Personal, then nowhere.
	jobs := filepath.Join(personal, "jobs", "J-01")
	h.mkdir(jobs)
	code := h.run(cwd, "env", "up", "J-01")
	stderr := h.errb.String()
	if h.out.Len() != 0 || strings.Contains(stderr, personal) {
		t.Errorf("leaked output: %s %s", h.out, stderr)
	}
	if err := os.Remove(jobs); err != nil {
		t.Fatal(err)
	}
	missingCode := h.run(cwd, "env", "up", "J-01")
	if code != 1 || code != missingCode || stderr != h.errb.String() {
		t.Errorf("hidden = (%d, %q), missing = (%d, %q)", code, stderr, missingCode, h.errb)
	}
}

func TestPersonalBypassWhere(t *testing.T) {
	h, cwd, personal := bypassFixture(t)
	code := h.run(cwd, "where", "--project", "personal", "--task", "T-01")
	stderr := h.errb.String()
	missing := h.run(cwd, "where", "--project", "personal", "--task", "T-09")
	if code != 1 || code != missing || stderr != h.errb.String() {
		t.Errorf("where: %d %q != %d %q", code, stderr, missing, h.errb)
	}
	for _, path := range []string{personal, filepath.Join(personal, "absent"), filepath.Join(filepath.Dir(personal), "T-99-absent")} {
		if code := h.run(cwd, "where", "--task", path); code != 1 || !strings.Contains(h.errb.String(), "task not found") || strings.ContainsAny(h.errb.String(), "/\\") {
			t.Errorf("path: %d %s", code, h.errb)
		}
	}
}

func TestPersonalBypassDuplicateID(t *testing.T) {
	h, cwd, personal := bypassFixture(t)
	for _, proj := range []string{"personal", "alpha"} {
		dir := filepath.Join(h.gridHome, "projects", proj, "tasks", "T-03-copy")
		if proj == "personal" {
			dir = filepath.Join(filepath.Dir(personal), "T-03-copy")
		}
		for _, file := range []string{"task.md", "intent.md"} {
			data, err := os.ReadFile(filepath.Join(personal, file))
			if err != nil {
				t.Fatal(err)
			}
			data = bytes.ReplaceAll(data, []byte("T-01"), []byte("T-03"))
			data = bytes.ReplaceAll(data, []byte("personal"), []byte(proj))
			h.write(filepath.Join(dir, file), string(data))
		}
	}
	if code := h.run(cwd, "task", "show", "T-03", "--json"); code != 0 {
		t.Fatalf("show: %d %s", code, h.errb)
	}
	var got task.Task
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Project != "alpha" {
		t.Errorf("project = %s", got.Project)
	}
}

func TestPersonalBypassCallerMatrix(t *testing.T) {
	for _, tc := range []struct {
		role, member string
		allowed      bool
	}{
		{"", "", true}, {"owner", "sam", true}, {"OWNER", "sam", true}, {"cli", "", true}, {"worker", "sam", true}, {"builder", "", true}, {"unknown", "", true}, {"member", "", true}, {"lead", "", true}, {"member", "sam", false}, {"lead", "sam", false}, {"external", "", false},
	} {
		t.Run(tc.role+"/"+tc.member, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": tc.role, "GRID_MEMBER": tc.member}[k] }
			code := h.run(h.home, "where", "--project", "personal")
			if !tc.allowed {
				if code != 1 {
					t.Errorf("exit = %d", code)
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if code := h.run(h.home, "where"); code != 0 {
				t.Fatal(h.errb)
			}
			golden.Assert(t, "where-personal-plain", h.norm(h.out.String()))
		})
	}
}

func bypassTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			out[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path)
		out[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
