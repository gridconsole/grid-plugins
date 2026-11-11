package lint_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// record turns the golden brief into the JSON record the orchestrator posts.
func record(t *testing.T) []byte {
	t.Helper()
	j, _, err := job.DecodeBrief(brief(t))
	if err != nil {
		t.Fatal(err)
	}
	b, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCheckJSONValidRecordIsClean(t *testing.T) {
	fs := lint.CheckJSON(record(t), lint.Options{Resolver: testFiles(t)})
	if lint.HasErrors(fs) {
		t.Fatalf("findings: %+v", fs)
	}
}

func TestCheckJSONMissingRequiredFieldNamesThePath(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(record(t), &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "title")
	b, _ := json.Marshal(m)
	fs := lint.CheckJSON(b, lint.Options{})
	if !lint.HasErrors(fs) || !has(fs, "job.record") || !strings.Contains(fs[0].Message, "title") {
		t.Fatalf("findings: %+v", fs)
	}
}

func TestCheckJSONRunsTheBriefRules(t *testing.T) {
	var m map[string]any
	if err := json.Unmarshal(record(t), &m); err != nil {
		t.Fatal(err)
	}
	m["on_blocked"] = "ask-and-continue"
	m["max_rounds"] = 0
	b, _ := json.Marshal(m)
	fs := lint.CheckJSON(b, lint.Options{})
	if !lint.HasErrors(fs) {
		t.Fatalf("expected a finding: %+v", fs)
	}
}

func TestCheckJSONNotJSON(t *testing.T) {
	fs := lint.CheckJSON([]byte("not json"), lint.Options{})
	if !lint.HasErrors(fs) || !has(fs, "job.record") {
		t.Fatalf("findings: %+v", fs)
	}
}

func TestIsJSON(t *testing.T) {
	for _, c := range []struct {
		name, text string
		want       bool
	}{
		{"a.json", "---\n", true},
		{"a.md", "  \n\t{\"id\":1}", true},
		{"a.md", "---\nid: J-1\n", false},
		{"a.md", "", false},
	} {
		if got := lint.IsJSON(c.name, []byte(c.text)); got != c.want {
			t.Errorf("%q %q: %v", c.name, c.text, got)
		}
	}
}
