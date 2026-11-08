package jobcmd

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func fixture(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join("..", "..", "..", "testdata", "job")
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		target := filepath.Join(root, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	return Env{Cwd: root, Stdout: out, Stderr: errs}, out, errs
}
func call(t *testing.T, e Env, out, errs *bytes.Buffer, want int, args ...string) report {
	t.Helper()
	out.Reset()
	errs.Reset()
	got := Run(args, e)
	if got != want {
		t.Fatalf("%v: exit %d want %d; stdout %s stderr %s", args, got, want, out, errs)
	}
	var r report
	if strings.Contains(strings.Join(args, " "), "--json") {
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
func TestRenderLintShow(t *testing.T) {
	e, out, errs := fixture(t)
	r := call(t, e, out, errs, 0, "render", "task", "1", "--json")
	if !r.OK || r.ID != "J-1" || r.Path == "" || r.Inputs == "" {
		t.Fatalf("%+v", r)
	}
	first, e2 := os.ReadFile(r.Path)
	if e2 != nil {
		t.Fatal(e2)
	}
	call(t, e, out, errs, 0, "render", "task", "J-1", "--json")
	got := call(t, e, out, errs, 0, "lint", r.Path, "--json")
	if !got.OK || len(got.Findings) != 0 {
		t.Fatalf("%+v", got)
	}
	show := call(t, e, out, errs, 0, "show", "task", "J-1", "--json")
	if show.Job == nil || show.Brief != string(first) {
		t.Fatal("show does not match saved brief")
	}
	call(t, e, out, errs, 0, "show", r.Path)
	if out.String() != string(first) {
		t.Fatal("plain show differs")
	}
	e.Cwd = filepath.Join(e.Cwd, "task")
	call(t, e, out, errs, 0, "show", "--json")
	call(t, e, out, errs, 0, "show", "J-1", "--json")
	call(t, e, out, errs, 0, "lint", r.Path)
	if !strings.Contains(out.String(), "ok: true") {
		t.Fatal("missing plain status")
	}
}
func TestRenderRefusesInvalidAndChangedJobs(t *testing.T) {
	e, out, errs := fixture(t)
	plan := filepath.Join(e.Cwd, "task", "plan.md")
	original, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(original), "on_blocked: ask-and-continue", "on_blocked: wait", 1)
	if err = os.WriteFile(plan, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	r := call(t, e, out, errs, 1, "render", "task", "1", "--json")
	if len(r.Findings) == 0 {
		t.Fatal("missing findings")
	}
	if _, err = os.Stat(filepath.Join(e.Cwd, "task", "jobs")); !os.IsNotExist(err) {
		t.Fatal("invalid render created artifacts")
	}
	if err = os.WriteFile(plan, original, 0644); err != nil {
		t.Fatal(err)
	}
	r = call(t, e, out, errs, 0, "render", "task", "1", "--json")
	before, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(e.Cwd, "knowledge", "conventions", "export-format.md")
	f, err := os.OpenFile(note, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("A changed constraint.\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	rejected := call(t, e, out, errs, 1, "render", "task", "1", "--json")
	if !strings.Contains(rejected.Error, "different saved inputs") {
		t.Fatalf("%+v", rejected)
	}
	after, err := os.ReadFile(r.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("changed existing bundle")
	}
}
func TestArgumentAndLintFailures(t *testing.T) {
	e, out, errs := fixture(t)
	for _, args := range [][]string{{"render"}, {"render", "task", "1", "--budget", "0"}, {"lint"}, {"show", "a", "b", "c"}, {"unknown"}, {"lint", "a", "--unknown"}} {
		call(t, e, out, errs, 2, append(args, "--json")...)
	}
	call(t, e, out, errs, 1, "render", "task", "999", "--json")
	call(t, e, out, errs, 1, "show", "task", "J-999", "--json")
	path := filepath.Join(e.Cwd, "broken.md")
	if err := os.WriteFile(path, []byte("broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := call(t, e, out, errs, 1, "lint", path, "--json")
	if len(r.Findings) != 1 || r.Findings[0].Rule != "brief.frontmatter" {
		t.Fatalf("%+v", r)
	}
}
func TestConcurrentIdenticalRender(t *testing.T) {
	e, _, _ := fixture(t)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := e
			out, errs := &bytes.Buffer{}, &bytes.Buffer{}
			local.Stdout, local.Stderr = out, errs
			if code := Run([]string{"render", "task", "1", "--json"}, local); code != 0 {
				t.Errorf("exit %d: %s %s", code, out, errs)
			}
		}()
	}
	wg.Wait()
	b, err := os.ReadFile(filepath.Join(e.Cwd, "task", "jobs", "J-1", "inputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatal("partial inputs published")
	}
}
func TestIndexGenerationRecorded(t *testing.T) {
	e, out, errs := fixture(t)
	if err := os.WriteFile(filepath.Join(e.Cwd, "knowledge", "backlinks.json"), []byte(`{"generation":7}`), 0644); err != nil {
		t.Fatal(err)
	}
	r := call(t, e, out, errs, 0, "render", "task", "1", "--json")
	b, err := os.ReadFile(r.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	var in struct {
		Generation string `json:"generation"`
	}
	if err = json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	if in.Generation != `{"project":7}` {
		t.Fatal(in.Generation)
	}
}
