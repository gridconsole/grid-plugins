package lint_test

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// narrowed is the golden brief with acceptance that misses plan words and an
// out_of_scope entry that names a plan word, so both coverage rules would fire.
func narrowed(t *testing.T, role string) string {
	t.Helper()
	text := brief(t)
	text = strings.Replace(text, `role: "writer"`, `role: "`+role+`"`, 1)
	text = strings.Replace(text, `"text":"Document the export format and column order."`, `"text":"Document the column order."`, 1)
	text = strings.Replace(text, `out_of_scope: ["runtime/**"]`, `out_of_scope: ["runtime/**", "export format"]`, 1)
	return text
}

func TestOrchestratorJobSkipsPlanWordCoverage(t *testing.T) {
	text := narrowed(t, "orchestrator")
	fs := lint.Check(text, lint.Options{})
	for _, rule := range []string{"job.acceptance-coverage", "job.scope-narrowing"} {
		if has(fs, rule) {
			t.Errorf("orchestrator Job reported %s: %+v", rule, fs)
		}
	}
	if lint.HasErrors(fs) {
		t.Errorf("orchestrator Job has errors: %+v", fs)
	}
	// Other rules still run on an orchestrator Job.
	bad := strings.Replace(text, `on_blocked: "ask-and-continue"`, `on_blocked: "wait"`, 1)
	if !has(lint.Check(bad, lint.Options{}), "job.on-blocked") {
		t.Fatal("job.on-blocked no longer reported for an orchestrator Job")
	}
}

func TestWorkerJobStillGetsPlanWordCoverage(t *testing.T) {
	fs := lint.Check(narrowed(t, "writer"), lint.Options{})
	if !has(fs, "job.acceptance-coverage") {
		t.Errorf("worker Job lost job.acceptance-coverage: %+v", fs)
	}
	if !has(fs, "job.scope-narrowing") || !lint.HasErrors(fs) {
		t.Errorf("worker Job lost job.scope-narrowing error: %+v", fs)
	}
}
