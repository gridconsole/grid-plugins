package lint_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// vcRepoRecord is the basic Job record naming repo ("" names none), with no read,
// write or out_of_scope paths, so only the Job's own repo can raise job.repo.
func vcRepoRecord(t *testing.T, repo string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"read", "write", "out_of_scope"} {
		body[key] = []any{}
	}
	if repo != "" {
		body["repo"] = repo
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// vcRepoRule is the findings of one rule.
func vcRepoRule(fs []lint.Finding, rule string) []lint.Finding {
	var out []lint.Finding
	for _, f := range fs {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

// Row 1: a repo the project does not list is one job.repo error naming the repos lint knows.
func TestVcRepoLintUnknownRepo(t *testing.T) {
	fs := lint.CheckJSON(vcRepoRecord(t, "nope"), lint.Options{Repos: []string{"app", "plugin"}})
	got := vcRepoRule(fs, "job.repo")
	if len(got) != 1 {
		t.Fatalf("want one job.repo finding, got %+v", fs)
	}
	f := got[0]
	if f.Severity != "error" || !strings.Contains(f.Message, "nope") || !strings.Contains(f.Message, "app") || !strings.Contains(f.Message, "plugin") {
		t.Fatalf("job.repo finding: %+v", f)
	}
}

// Row 1: a repo the project lists gives no job.repo finding, and the record passes the schema.
func TestVcRepoLintKnownRepo(t *testing.T) {
	fs := lint.CheckJSON(vcRepoRecord(t, "plugin"), lint.Options{Repos: []string{"app", "plugin"}})
	if got := append(vcRepoRule(fs, "job.repo"), vcRepoRule(fs, "job.record")...); len(got) != 0 {
		t.Fatalf("a listed repo has findings: %+v", got)
	}
}

// Row 1: a project with no repo record lints a Job's repo as today: no job.repo finding.
func TestVcRepoLintNoRepoRecord(t *testing.T) {
	fs := lint.CheckJSON(vcRepoRecord(t, "x"), lint.Options{})
	if got := append(vcRepoRule(fs, "job.repo"), vcRepoRule(fs, "job.record")...); len(got) != 0 {
		t.Fatalf("no repo record still has findings: %+v", got)
	}
}
