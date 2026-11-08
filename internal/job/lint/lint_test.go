package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

func brief(t *testing.T) string {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "golden", "job-brief.golden"))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func testFiles(t *testing.T) *job.Files {
	t.Helper()
	h := jobtest.New(t, filepath.Join("..", "..", "..", "testdata"))
	return &job.Files{Task: h.Task, Owner: contract.PersonTeam, Notes: h.Store, Folders: []string{h.Folder}, Repo: h.Folder}
}
func has(fs []lint.Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}
func TestGoldenBriefPasses(t *testing.T) {
	f := testFiles(t)
	fs := lint.Check(brief(t), lint.Options{Resolver: f})
	if len(fs) != 0 {
		t.Fatalf("findings: %+v", fs)
	}
}
func TestEveryLegacyFieldAndHeading(t *testing.T) {
	base := brief(t)
	for _, key := range []string{"id", "role", "status", "verify", "follow_up"} {
		t.Run(key, func(t *testing.T) {
			lines := strings.Split(base, "\n")
			for i, l := range lines {
				if strings.HasPrefix(l, key+":") {
					lines[i] = key + ": \"\""
				}
			}
			if !has(lint.Check(strings.Join(lines, "\n"), lint.Options{}), "brief.field") {
				t.Fatal("missing legacy field rule")
			}
		})
	}
	for _, heading := range []string{"Intent", "Job", "Context", "Out of scope", "Open", "Acceptance", "Outcome", "Verdict", "Follow-up"} {
		t.Run(heading, func(t *testing.T) {
			text := strings.Replace(base, "## "+heading+"\n", "", 1)
			if !has(lint.Check(text, lint.Options{}), "brief.section") {
				t.Fatal("missing legacy heading rule")
			}
		})
	}
	if !has(lint.Check("no frontmatter", lint.Options{}), "brief.frontmatter") {
		t.Fatal("missing frontmatter rule")
	}
}
func TestContractRules(t *testing.T) {
	base := brief(t)
	j, _, e := job.DecodeBrief(base)
	if e != nil {
		t.Fatal(e)
	}
	body := base[strings.Index(base, "\n## Intent"):]
	tests := []struct {
		name, rule string
		edit       func(*job.Job)
	}{
		{"id", "job.id", func(j *job.Job) { j.ID = "../J-1" }},
		{"item", "job.item", func(j *job.Job) { j.Item = 0 }},
		{"intent", "job.intent", func(j *job.Job) { j.Intent.Owner = "" }},
		{"review", "job.review", func(j *job.Job) { j.Review = "L4" }},
		{"rounds", "job.max-rounds", func(j *job.Job) { j.MaxRounds = 0 }},
		{"risk", "job.risk", func(j *job.Job) { j.Risk = []string{"unknown"} }},
		{"budget", "job.budget", func(j *job.Job) { j.Budget.TurnsEstimate = 0 }},
		{"blocked", "job.on-blocked", func(j *job.Job) { j.OnBlocked = "wait" }},
		{"verify", "job.verify", func(j *job.Job) { j.Verify = job.Verify{} }},
		{"zero tests", "job.verify-tests", func(j *job.Job) { j.Verify = job.Verify{Command: "go test ./..."} }},
		{"reason", "job.link-why", func(j *job.Job) {
			j.Links = []job.Link{{To: "brain:project/export-format", Rel: "uses", Why: "see also"}}
		}},
		{"target name", "job.link-why", func(j *job.Job) {
			j.Links = []job.Link{{To: "brain:project/export-format", Rel: "uses", Why: "export format"}}
		}},
		{"rel", "job.link-rel", func(j *job.Job) {
			j.Links = []job.Link{{To: "brain:project/a", Rel: "informs", Why: "Explains the requirement"}}
		}},
		{"derived", "job.derived-link", func(j *job.Job) { j.Links = []job.Link{{To: "job:J-2", Rel: "blocks", Why: "Wait for the other Job"}} }},
		{"brain id", "job.brain-id", func(j *job.Job) { j.Keep = []string{"copied rule text"} }},
		{"trace", "job.acceptance-trace", func(j *job.Job) {
			j.Acceptance = []job.Acceptance{{ID: "A1", Text: "Export format", Traces: "unknown"}}
		}},
		{"narrowing", "job.narrowing", func(j *job.Job) {
			j.Acceptance = []job.Acceptance{{ID: "A1", Text: "Export format", Traces: "item", State: "dropped"}}
		}},
		{"scope", "job.scope-narrowing", func(j *job.Job) { j.OutOfScope = []string{"export format"} }},
		{"repo", "job.repo", func(j *job.Job) { j.Write = []string{"unlisted:docs/**"} }},
		{"close reason", "job.close-reason", func(j *job.Job) { j.Status = "cancelled" }},
		{"after", "job.after-kind", func(j *job.Job) { j.After = []job.Dependency{{Job: "J-2", Kind: "unknown"}} }},
		{"self dependency", "job.after-cycle", func(j *job.Job) { j.After = []job.Dependency{{Job: "J-1", Kind: "done"}} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			copy := j
			tc.edit(&copy)
			m, e := job.Metadata(copy)
			if e != nil {
				t.Fatal(e)
			}
			fs := lint.Check(m+body, lint.Options{})
			if !has(fs, tc.rule) {
				t.Fatalf("missing %s in %+v", tc.rule, fs)
			}
			for _, f := range fs {
				if f.Line < 1 || f.Fix == "" {
					t.Fatalf("missing diagnostic location or fix: %+v", f)
				}
			}
		})
	}
}
func TestMissingFieldsAndReadRanges(t *testing.T) {
	base := brief(t)
	for _, key := range []string{"title", "skills", "risk", "keep", "after", "intent", "verify", "budget"} {
		lines := strings.Split(base, "\n")
		for i, l := range lines {
			if strings.HasPrefix(l, key+":") {
				lines[i] = ""
			}
		}
		if !has(lint.Check(strings.Join(lines, "\n"), lint.Options{}), "job.required") {
			t.Errorf("missing required check for %s", key)
		}
	}
	f := testFiles(t)
	bad := strings.Replace(base, "task:research.md#format", "task:research.md:1-999", 1)
	if !has(lint.Check(bad, lint.Options{Resolver: f}), "job.read-range") {
		t.Fatal("bad read passed")
	}
}
func TestAcceptanceCoverageIsWarning(t *testing.T) {
	text := strings.Replace(brief(t), `"text":"Document the export format and column order."`, `"text":"Document the column order."`, 1)
	fs := lint.Check(text, lint.Options{})
	if !has(fs, "job.acceptance-coverage") || lint.HasErrors(fs) {
		t.Fatalf("wrong coverage severity: %+v", fs)
	}
}
