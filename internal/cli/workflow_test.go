package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/workflow"
)

const cliWorkflow = "# Workflows\n\n## Default\napplies when: nothing else matches\nreview rounds: 2\n\n| # | step | kind | who | on reject, back to |\n|---|---|---|---|---|\n| 1 | Build | run | grid-worker | |\n"

func TestWorkflowShowLinkedProject(t *testing.T) {
	h := newHarness(t)
	outside := filepath.Join(h.root, "outside")
	h.write(filepath.Join(outside, "project.md"), "---\nid: id-alpha\n---\n")
	h.write(filepath.Join(outside, "workflows.md"), cliWorkflow)
	if err := os.MkdirAll(filepath.Join(h.gridHome, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(h.gridHome, "projects", "alpha")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	for _, selector := range []string{"", "alpha", "id-alpha"} {
		for _, mode := range []string{"plain", "json"} {
			t.Run(selector+"/"+mode, func(t *testing.T) {
				args := []string{"workflow", "show"}
				if selector != "" {
					args = append(args, "--project", selector)
				}
				if mode == "json" {
					args = append(args, "--json")
				}
				code := h.run(h.root, args...)
				if code != 2 || h.out.Len() != 0 || h.errb.String() != "grid workflow show: workflow path is unsafe or inaccessible\n" {
					t.Fatalf("exit %d: out=%q err=%q", code, h.out, h.errb)
				}
			})
		}
	}
}

func TestWorkflowShowJSON(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "workflows.md"), cliWorkflow)
	for _, name := range []string{"alpha", "beta"} {
		h.write(filepath.Join(h.gridHome, "projects", name, "project.md"), "---\nid: id-"+name+"\n---\n")
	}
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"), strings.ReplaceAll(cliWorkflow, "nothing else matches", "label alpha"))
	for _, selector := range []string{"", "alpha", "id-alpha"} {
		t.Run(selector, func(t *testing.T) {
			args := []string{"workflow", "show", "--json"}
			if selector != "" {
				args = append(args, "--project", selector)
			}
			if code := h.run(h.root, args...); code != 0 || h.errb.Len() != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			var got workflow.Answer
			if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			count := 3
			if selector != "" {
				count = 2
			}
			if len(got.Files) != count || len(got.Templates) != 2 || got.Errors == nil || len(got.Errors) != 0 {
				t.Fatalf("%s", h.out)
			}
			if got.Files[0].Scope != "workspace" || got.Files[0].Project != nil || got.Files[0].Rev == nil || len(*got.Files[0].Rev) != 12 || got.Files[0].Text != cliWorkflow {
				t.Fatalf("%+v", got.Files[0])
			}
			if got.Files[1].Scope != "project" || *got.Files[1].Project != "alpha" {
				t.Fatalf("%+v", got.Files[1])
			}
			if count == 3 && (got.Files[2].Rev != nil || got.Files[2].Text != "" || *got.Files[2].Project != "beta") {
				t.Fatalf("%+v", got.Files[2])
			}
			if got.Templates[0].Steps[0].Agent == nil || *got.Templates[0].Steps[0].Agent != "grid-builder" {
				t.Fatalf("%+v", got.Templates)
			}
			var shape map[string]json.RawMessage
			if err := json.Unmarshal(h.out.Bytes(), &shape); err != nil {
				t.Fatal(err)
			}
			if len(shape) != 3 || shape["files"] == nil || shape["templates"] == nil || string(shape["errors"]) != "[]" {
				t.Fatalf("shape: %s", h.out)
			}
			var files []map[string]any
			if err := json.Unmarshal(shape["files"], &files); err != nil {
				t.Fatal(err)
			}
			if len(files[0]) != 4 || files[0]["project"] != nil {
				t.Fatalf("files: %+v", files)
			}
		})
	}
	if code := h.run(h.root, "workflow", "show", "--json", "--project", "unknown"); code != exitCode(os.ErrNotExist) || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "missing or ambiguous") {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
}

func TestWorkflowShowPlainAndProblems(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.gridHome, "workflows.md"), cliWorkflow+"\n## Broken\napplies when: label a\nreview rounds: 4\n")
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: id-alpha\n---\n")
	if code := h.run(h.root, "workflow", "show"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	want := "file: workflows.md\ntemplates: Default\nerrors: 1\nfile: alpha/workflows.md\ntemplates: (none)\nerrors: 0\n"
	if h.out.String() != want {
		t.Fatalf("%q want %q", h.out.String(), want)
	}
	if code := h.run(h.root, "workflow", "show", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var got workflow.Answer
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Templates) != 1 || len(got.Errors) != 1 {
		t.Fatalf("%+v", got)
	}
	var raw map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	p := raw["errors"].([]any)[0]
	if !reflect.DeepEqual(p, map[string]any{"file": "workflows.md", "section": "Broken", "line": float64(13), "message": "review rounds must be 1, 2, 3 or 5"}) {
		t.Fatalf("%+v", p)
	}
}

func TestWorkflowShowUsageAndReadOnly(t *testing.T) {
	for _, args := range [][]string{{"workflow"}, {"workflow", "set"}, {"workflow", "show", "extra"}, {"workflow", "show", "--bad"}, {"workflow", "show", "--project"}, {"workflow", "show", "--project="}} {
		h := newHarness(t)
		if code := h.run(h.root, args...); code != 2 || h.out.Len() != 0 {
			t.Fatalf("%v: exit %d %s %s", args, code, h.out, h.errb)
		}
	}
	h := newHarness(t)
	for _, args := range [][]string{{"workflow", "--help"}, {"workflow", "show", "--help"}} {
		if code := h.run(h.root, args...); code != 0 {
			t.Fatalf("%v: %d", args, code)
		}
	}
	h.env.GridHome = filepath.Join(h.home, "absent")
	if code := h.run(h.root, "workflow", "show", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if h.out.String() != "{\"files\":[{\"scope\":\"workspace\",\"project\":null,\"rev\":null,\"text\":\"\"}],\"templates\":[],\"errors\":[]}\n" {
		t.Fatalf("%s", h.out)
	}
	if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
		t.Fatalf("read wrote home: %v", err)
	}
	h.env.GridHome = ""
	if code := h.run(h.root, "workflow", "show", "--json"); code != exitCode(errors.New("missing home")) {
		t.Fatalf("exit %d", code)
	}
}
