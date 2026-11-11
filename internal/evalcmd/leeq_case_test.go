package evalcmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const (
	leEqCaseDir = "../../evals/suite/orchestrator-asks-question"
	leEqDataDir = "testdata/leeq"
	leEqName    = "orchestrator-asks-question"
)

func leEqRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func leEqGrade(t *testing.T, transcript string) map[string]GraderResult {
	t.Helper()
	grade, err := GradeCase(leEqCaseDir, []byte(leEqRead(t, filepath.Join(leEqDataDir, transcript))), "")
	if err != nil {
		t.Fatalf("GradeCase on %s: %v", transcript, err)
	}
	if want := []string{"later-job-waits"}; !reflect.DeepEqual(grade.NotGraded, want) {
		t.Errorf("NotGraded = %v, want %v (the llm grader only)", grade.NotGraded, want)
	}
	byName := map[string]GraderResult{}
	for _, r := range grade.Graders {
		byName[r.Name] = r
		t.Logf("%s on %s: passed=%v (%s)", r.Name, transcript, r.Passed, r.Reason)
	}
	for _, name := range []string{"question-asked", "no-person-job"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("grader %s did not run on %s", name, transcript)
		}
	}
	return byName
}

// leEqBashLines returns the Bash commands of a transcript file, one per tool_use line.
func leEqBashLines(t *testing.T, transcript string) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(leEqRead(t, filepath.Join(leEqDataDir, transcript)), "\n") {
		if strings.Contains(line, `"name":"Bash"`) {
			out = append(out, line)
		}
	}
	return out
}

func TestLeEqShape(t *testing.T) {
	prompt, _, err := hjrFrontMatter(leEqRead(t, filepath.Join(leEqCaseDir, "prompt.md")))
	if err != nil {
		t.Fatal(err)
	}
	if prompt["name"] != leEqName || prompt["tags"] != "[live]" || prompt["runs"] != "1" {
		t.Errorf("prompt.md front matter: name=%q tags=%q runs=%q", prompt["name"], prompt["tags"], prompt["runs"])
	}
	if n, err := strconv.Atoi(prompt["max_turns"]); err != nil || n < 1 {
		t.Errorf("prompt.md max_turns=%q, want a positive number", prompt["max_turns"])
	}
	if prompt["allowed_tools"] == "" {
		t.Error("prompt.md has no allowed_tools")
	}
	caseYAML := leEqRead(t, filepath.Join(leEqCaseDir, "case.yaml"))
	meta := hjrFlat(strings.Split(caseYAML, "\n"))
	if meta["schema_version"] != "1.1" || meta["name"] != leEqName {
		t.Errorf("case.yaml: schema_version=%q name=%q", meta["schema_version"], meta["name"])
	}
	if !strings.Contains(caseYAML, "\n  scaffold_script: fixture.sh\n") {
		t.Error("case.yaml context has no scaffold_script: fixture.sh")
	}
	if _, err := os.Stat(filepath.Join(leEqCaseDir, "fixture.sh")); err != nil {
		t.Error(err)
	}
	files, err := filepath.Glob(filepath.Join(leEqCaseDir, "graders", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no graders under %s: %v", leEqCaseDir, err)
	}
	seen := map[string]string{}
	for _, f := range files {
		gm, _, err := hjrFrontMatter(leEqRead(t, f))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if !hjrKnownTypes[gm["type"]] {
			t.Errorf("%s: unknown grader type %q", f, gm["type"])
		}
		// GradeCase refuses a `name` key on a graded type: the file name is the name.
		if _, has := gm["name"]; has {
			t.Errorf("%s: grader has a name key", f)
		}
		seen[strings.TrimSuffix(filepath.Base(f), ".md")] = gm["type"]
	}
	want := map[string]string{"question-asked": "tool_used", "no-person-job": "tool_used", "later-job-waits": "llm"}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("graders = %v, want %v", seen, want)
	}
	for _, name := range []string{"bad.jsonl", "good.jsonl"} {
		if n := strings.Count(leEqRead(t, filepath.Join(leEqDataDir, name)), "\n"); n > 40 {
			t.Errorf("%s has %d lines, want at most 40", name, n)
		}
	}
}

func TestLeEqGood(t *testing.T) {
	asks := 0
	for _, line := range leEqBashLines(t, "good.jsonl") {
		if strings.Contains(line, "grid question ask") {
			asks++
		}
		if strings.Contains(line, "--person") {
			t.Errorf("good.jsonl posts a person Job by hand: %s", line)
		}
	}
	if asks == 0 {
		t.Fatal("good.jsonl never runs grid question ask")
	}
	if !strings.Contains(leEqRead(t, filepath.Join(leEqDataDir, "good.jsonl")), `\"after\":[{\"job\":\"J-2\"`) {
		t.Error("good.jsonl has no later Job whose after names the person Job J-2")
	}
	for name, r := range leEqGrade(t, "good.jsonl") {
		if !r.Passed {
			t.Errorf("grader %s fails on good.jsonl: %s", name, r.Reason)
		}
	}
}

func TestLeEqBad(t *testing.T) {
	person := 0
	for _, line := range leEqBashLines(t, "bad.jsonl") {
		if strings.Contains(line, "grid question ask") {
			t.Errorf("bad.jsonl runs grid question ask: %s", line)
		}
		if strings.Contains(line, "grid job post") && strings.Contains(line, "--person") {
			person++
		}
	}
	if person == 0 {
		t.Fatal("bad.jsonl never runs grid job post --person")
	}
	got := leEqGrade(t, "bad.jsonl")
	for _, name := range []string{"question-asked", "no-person-job"} {
		if got[name].Passed {
			t.Errorf("grader %s must fail on bad.jsonl: %s", name, got[name].Reason)
		}
	}
}
