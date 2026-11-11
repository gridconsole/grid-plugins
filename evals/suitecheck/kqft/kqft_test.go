package kqft

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/evalcmd"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"go.yaml.in/yaml/v4"
)

const (
	kqFtCaseDir  = "../../suite/fix-total"
	kqFtTaskDir  = "../../tasks/fix-total"
	kqFtGoodJSON = "testdata/good.jsonl"
)

// kqFtGraderKeys copies graderKeys (internal/evalcmd/suitegrade.go:57-64), which is
// not exported: the grader types the offline grader accepts.
var kqFtGraderKeys = map[string]bool{"regex": true, "tool_used": true, "tool_order": true, "file_exists": true}

// kqFtDropped lists the checks of evals/tasks/fix-total/checks.json that have no
// grader in the case, each with its reason.
var kqFtDropped = map[string]string{
	"sum": "no command grader (suiteexport.go:107-108); an arithmetic check such as `sh total.sh 2 3` has no counterpart, only the agent can run the script (evals/builtin-fit.md, fix-total row and answer (a))",
	"script": "file_exists total.sh can never pass: fixture.sh writes total.sh and only files created during the run count (evals/builtin-fit.md, fix-total row and partial list); " +
		"the zero grader still fails when total.sh is missing",
}

func kqFtRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func kqFtFront(t *testing.T, path string) (map[string]any, string) {
	t.Helper()
	front, body, err := mdfile.Split(kqFtRead(t, path))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var meta map[string]any
	if err := mdfile.Load(front, &meta, false); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return meta, body
}

func kqFtRun(t *testing.T, dir string, args ...string) error {
	t.Helper()
	cmd := exec.Command("sh", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	t.Logf("sh %s in %s: err=%v out=%q", strings.Join(args, " "), filepath.Base(dir), err, out)
	return err
}

// kqFtFixtureDir runs fixture.sh in a fresh folder and returns it.
func kqFtFixtureDir(t *testing.T) string {
	t.Helper()
	fixture, err := filepath.Abs(filepath.Join(kqFtCaseDir, "fixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := kqFtRun(t, dir, fixture); err != nil {
		t.Fatalf("fixture.sh failed: %v", err)
	}
	return dir
}

func TestKqFtShape(t *testing.T) {
	var c struct {
		SchemaVersion string `yaml:"schema_version"`
		Name          string `yaml:"name"`
		Context       struct {
			ScaffoldScript string `yaml:"scaffold_script"`
		} `yaml:"context"`
	}
	if err := yaml.Unmarshal(kqFtRead(t, filepath.Join(kqFtCaseDir, "case.yaml")), &c); err != nil {
		t.Fatal(err)
	}
	if c.SchemaVersion != "1.1" || c.Name != "fix-total" || c.Context.ScaffoldScript != "fixture.sh" {
		t.Errorf("case.yaml: %+v", c)
	}
	meta, _ := kqFtFront(t, filepath.Join(kqFtCaseDir, "prompt.md"))
	if meta["name"] != "fix-total" || meta["runs"] != 1 {
		t.Errorf("prompt.md front matter: name=%v runs=%v", meta["name"], meta["runs"])
	}
	entries, err := os.ReadDir(filepath.Join(kqFtCaseDir, "graders"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no grader")
	}
	for _, e := range entries {
		g, _ := kqFtFront(t, filepath.Join(kqFtCaseDir, "graders", e.Name()))
		typ, _ := g["type"].(string)
		if !kqFtGraderKeys[typ] {
			t.Errorf("%s: type %q is not a graderKeys key", e.Name(), typ)
		}
	}
}

func TestKqFtMap(t *testing.T) {
	var checks []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(kqFtRead(t, filepath.Join(kqFtTaskDir, "checks.json")), &checks); err != nil {
		t.Fatal(err)
	}
	if len(checks) == 0 {
		t.Fatal("checks.json holds no check")
	}
	seen := map[string]bool{}
	for _, c := range checks {
		seen[c.ID] = true
		_, statErr := os.Stat(filepath.Join(kqFtCaseDir, "graders", c.ID+".md"))
		hasGrader := statErr == nil
		reason, dropped := kqFtDropped[c.ID]
		switch {
		case hasGrader && dropped:
			t.Errorf("check %s has a grader and is also in kqFtDropped", c.ID)
		case !hasGrader && !dropped:
			t.Errorf("check %s has no grader and is not in kqFtDropped", c.ID)
		case dropped && strings.TrimSpace(reason) == "":
			t.Errorf("dropped check %s has no reason", c.ID)
		}
	}
	for id := range kqFtDropped {
		if !seen[id] {
			t.Errorf("kqFtDropped names %s, which is not a check", id)
		}
	}
	entries, err := os.ReadDir(filepath.Join(kqFtCaseDir, "graders"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if id := strings.TrimSuffix(e.Name(), ".md"); !seen[id] {
			t.Errorf("grader %s is not a check id", e.Name())
		}
	}
	_, body := kqFtFront(t, filepath.Join(kqFtCaseDir, "prompt.md"))
	if strings.TrimSpace(body) != strings.TrimSpace(string(kqFtRead(t, filepath.Join(kqFtTaskDir, "prompt.md")))) {
		t.Errorf("prompt.md body differs from %s/prompt.md", kqFtTaskDir)
	}
	if got, want := kqFtRead(t, filepath.Join(kqFtCaseDir, "fixture.sh")), kqFtRead(t, filepath.Join(kqFtTaskDir, "setup.sh")); string(got) != string(want) {
		t.Error("fixture.sh differs from setup.sh")
	}
}

func TestKqFtGood(t *testing.T) {
	grade, err := evalcmd.GradeCase(kqFtCaseDir, kqFtRead(t, kqFtGoodJSON), "testdata/files-good")
	if err != nil {
		t.Fatal(err)
	}
	if !grade.Passed || len(grade.NotGraded) != 0 {
		t.Errorf("Passed=%v NotGraded=%v graders=%+v", grade.Passed, grade.NotGraded, grade.Graders)
	}
	good := kqFtRead(t, "testdata/files-good/total.sh")
	if !strings.Contains(string(good), "\nsum=0\n") {
		t.Error("files-good/total.sh does not start at sum=0")
	}
	// The dropped sum check holds on the good data.
	dir := kqFtFixtureDir(t)
	if err := os.WriteFile(filepath.Join(dir, "total.sh"), good, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := kqFtRun(t, dir, "verify.sh"); err != nil {
		t.Errorf("sh verify.sh failed on files-good: %v", err)
	}
}

func TestKqFtBad(t *testing.T) {
	grade, err := evalcmd.GradeCase(kqFtCaseDir, kqFtRead(t, kqFtGoodJSON), "testdata/files-bad")
	if err != nil {
		t.Fatal(err)
	}
	if grade.Passed {
		t.Error("files-bad passed")
	}
	failedZero := false
	for _, g := range grade.Graders {
		if g.Name == "zero" && !g.Passed {
			failedZero = true
		}
	}
	if !failedZero {
		t.Errorf("zero is not among the failed graders: %+v", grade.Graders)
	}
}

func TestKqFtFixture(t *testing.T) {
	dir := kqFtFixtureDir(t)
	for _, f := range []string{"total.sh", "verify.sh"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("fixture.sh did not write %s: %v", f, err)
		}
	}
	if err := kqFtRun(t, dir, "verify.sh"); err == nil {
		t.Error("sh verify.sh passes on the fixture; the bug is missing")
	}
}
