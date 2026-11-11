package kqrn

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/evalcmd"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
)

const (
	kqRnCase  = "../../suite/release-notes"
	kqRnTask  = "../../tasks/release-notes"
	kqRnTypes = "regex tool_used tool_order file_exists" // the keys of graderKeys (suitegrade.go:57-64)
)

func kqRnRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// kqRnFront splits a markdown file into its front matter keys and its body.
func kqRnFront(t *testing.T, path string) (map[string]any, string) {
	t.Helper()
	front, body, err := mdfile.Split(kqRnRead(t, path))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var keys map[string]any
	if err := mdfile.Load(front, &keys, false); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return keys, body
}

func kqRnGraders(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(kqRnCase, "graders", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graders in %s (%v)", kqRnCase, err)
	}
	return files
}

func TestKqRnShape(t *testing.T) {
	yamlText := string(kqRnRead(t, filepath.Join(kqRnCase, "case.yaml")))
	for _, want := range []string{`schema_version: "1.1"`, "name: release-notes\n", "scaffold_script: fixture.sh"} {
		if !strings.Contains(yamlText, want) {
			t.Errorf("case.yaml lacks %q", want)
		}
	}

	keys, _ := kqRnFront(t, filepath.Join(kqRnCase, "prompt.md"))
	if keys["name"] != "release-notes" {
		t.Errorf("prompt.md name = %v, want release-notes", keys["name"])
	}
	if keys["runs"] != 1 {
		t.Errorf("prompt.md runs = %v, want 1", keys["runs"])
	}

	for _, f := range kqRnGraders(t) {
		gk, _ := kqRnFront(t, f)
		typ, _ := gk["type"].(string)
		if !strings.Contains(" "+kqRnTypes+" ", " "+typ+" ") {
			t.Errorf("%s: type %q is not a key of graderKeys", f, typ)
		}
		if _, ok := gk["name"]; ok {
			t.Errorf("%s: a grader file takes no name key", f)
		}
	}
}

func TestKqRnMap(t *testing.T) {
	var checks []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(kqRnRead(t, filepath.Join(kqRnTask, "checks.json")), &checks); err != nil {
		t.Fatal(err)
	}
	if len(checks) == 0 {
		t.Fatal("checks.json holds no check")
	}
	var want []string
	for _, c := range checks {
		want = append(want, c.ID)
		if _, err := os.Stat(filepath.Join(kqRnCase, "graders", c.ID+".md")); err != nil {
			t.Errorf("check %s has no grader: %v", c.ID, err)
		}
	}
	var have []string
	for _, f := range kqRnGraders(t) {
		have = append(have, strings.TrimSuffix(filepath.Base(f), ".md"))
	}
	sort.Strings(want)
	sort.Strings(have)
	if !reflect.DeepEqual(want, have) {
		t.Errorf("graders %v, checks %v", have, want)
	}

	_, body := kqRnFront(t, filepath.Join(kqRnCase, "prompt.md"))
	if got, orig := strings.TrimSpace(body), strings.TrimSpace(string(kqRnRead(t, filepath.Join(kqRnTask, "prompt.md")))); got != orig {
		t.Errorf("prompt.md body %q differs from the task prompt %q", got, orig)
	}
	if got, orig := kqRnRead(t, filepath.Join(kqRnCase, "fixture.sh")), kqRnRead(t, filepath.Join(kqRnTask, "setup.sh")); string(got) != string(orig) {
		t.Errorf("fixture.sh differs from setup.sh")
	}
}

func kqRnGrade(t *testing.T, files string) evalcmd.CaseGrade {
	t.Helper()
	grade, err := evalcmd.GradeCase(kqRnCase, kqRnRead(t, "testdata/good.jsonl"), files)
	if err != nil {
		t.Fatal(err)
	}
	return grade
}

func kqRnFailed(g evalcmd.CaseGrade) []string {
	var out []string
	for _, r := range g.Graders {
		if !r.Passed {
			out = append(out, r.Name)
		}
	}
	sort.Strings(out)
	return out
}

func TestKqRnGood(t *testing.T) {
	g := kqRnGrade(t, "testdata/files-good")
	if !g.Passed || len(g.NotGraded) != 0 || len(g.Graders) != 4 {
		t.Fatalf("Passed=%v NotGraded=%v graders=%d, want true, none, 4; failed %v", g.Passed, g.NotGraded, len(g.Graders), kqRnFailed(g))
	}
}

func TestKqRnBad(t *testing.T) {
	g := kqRnGrade(t, "testdata/files-bad")
	if g.Passed {
		t.Error("files-bad passed")
	}
	if got := kqRnFailed(g); !reflect.DeepEqual(got, []string{"fix"}) {
		t.Errorf("files-bad failed graders %v, want [fix]", got)
	}

	g = kqRnGrade(t, "testdata/files-none")
	if g.Passed {
		t.Error("files-none passed")
	}
	found := false
	for _, name := range kqRnFailed(g) {
		found = found || name == "notes"
	}
	if !found {
		t.Errorf("files-none failed graders %v, want notes among them", kqRnFailed(g))
	}
}

func TestKqRnFixture(t *testing.T) {
	script, err := filepath.Abs(filepath.Join(kqRnCase, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd := exec.Command("sh", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("sh fixture.sh: %v\n%s", err, out)
	}
	got := string(kqRnRead(t, filepath.Join(dir, "changes.txt")))
	if want := "Fix empty search results\nAdd CSV export\n"; got != want {
		t.Errorf("changes.txt = %q, want %q", got, want)
	}
}
