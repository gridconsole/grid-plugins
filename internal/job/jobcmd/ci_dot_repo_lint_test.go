package jobcmd

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// ciDotRecord is the basic Job record naming repo, with no read, write or out_of_scope
// paths, so only the Job's own repo can raise job.repo.
func ciDotRecord(t *testing.T, repo string) []byte {
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
	body["repo"] = repo
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// ciDotRepoFindings counts the job.repo findings lint gives a Job naming repo.
func ciDotRepoFindings(t *testing.T, opts lint.Options, repo string) int {
	t.Helper()
	n := 0
	for _, f := range lint.CheckJSON(ciDotRecord(t, repo), opts) {
		if f.Rule == "job.repo" {
			n++
		}
	}
	return n
}

// A project.md that lists "." and web: the options the post path builds take repo "." and
// "web", and refuse "nope".
func TestCiDotProjectRecordLint(t *testing.T) {
	e, _, _ := fixture(t)
	prRecord(t, e, ".", "web")
	opts, err := LintOptions(prTask(e), e)
	if err != nil {
		t.Fatal(err)
	}
	for repo, want := range map[string]int{".": 0, "web": 0, "nope": 1} {
		if got := ciDotRepoFindings(t, opts, repo); got != want {
			t.Errorf("repo %q: %d job.repo findings, want %d (options repos %v)", repo, got, want, opts.Repos)
		}
	}
}
