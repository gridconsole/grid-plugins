package lint_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

// rowsRecord is the basic Job record with the given fields replaced.
func rowsRecord(t *testing.T, set map[string]any) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["read"] = []any{}
	for k, v := range set {
		body[k] = v
	}
	out, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func rowsAcceptance(text string) []any {
	return []any{map[string]any{"id": "A1", "text": text, "traces": "item"}}
}

func TestAfterMissingHit(t *testing.T) {
	fs := lint.CheckJSON(rowsRecord(t, map[string]any{"acceptance": rowsAcceptance("The rule from J-3 flags a Job")}), lint.Options{})
	got := vcRepoRule(fs, "job.after-missing")
	if len(got) != 1 || got[0].Severity != "error" {
		t.Fatalf("want one job.after-missing error: %+v", fs)
	}
}

func TestAfterMissingClean(t *testing.T) {
	set := map[string]any{
		"acceptance":      rowsAcceptance("Builds on J-3, J-1, J-4 and J-5"),
		"after":           []any{map[string]any{"job": "J-3"}},
		"discovered_from": "J-4",
		"continues":       "J-5",
	}
	fs := lint.CheckJSON(rowsRecord(t, set), lint.Options{})
	if got := vcRepoRule(fs, "job.after-missing"); len(got) != 0 {
		t.Fatalf("after, self, discovered_from and continues are excepted: %+v", got)
	}
}

func TestLiveUIRowHit(t *testing.T) {
	fs := lint.CheckJSON(rowsRecord(t, map[string]any{"acceptance": rowsAcceptance("The panel shows on the live dev UI")}), lint.Options{})
	got := vcRepoRule(fs, "job.live-ui-row")
	if len(got) != 1 || got[0].Severity != "warning" || got[0].Fix == "" {
		t.Fatalf("want one job.live-ui-row warning: %+v", fs)
	}
	if !contains(got[0].Fix, "grid-checker") || !contains(got[0].Fix, "merge and install") {
		t.Fatalf("hint: %q", got[0].Fix)
	}
}

func TestLiveUIRowClean(t *testing.T) {
	fs := lint.CheckJSON(rowsRecord(t, map[string]any{"acceptance": rowsAcceptance("The unit test for the panel passes")}), lint.Options{})
	if got := vcRepoRule(fs, "job.live-ui-row"); len(got) != 0 {
		t.Fatalf("clean row flagged: %+v", got)
	}
	checker := rowsRecord(t, map[string]any{"role": "checker", "acceptance": rowsAcceptance("The panel shows on the live dev UI")})
	if got := vcRepoRule(lint.CheckJSON(checker, lint.Options{}), "job.live-ui-row"); len(got) != 0 {
		t.Fatalf("a checker Job is not flagged: %+v", got)
	}
}

func TestRowOutsideWriteHit(t *testing.T) {
	set := map[string]any{"write": []any{"internal/a/a.go"}, "acceptance": rowsAcceptance("internal/b/b.go:10-12 holds the check")}
	got := vcRepoRule(lint.CheckJSON(rowsRecord(t, set), lint.Options{}), "job.row-outside-write")
	if len(got) != 1 || got[0].Severity != "error" {
		t.Fatalf("want one job.row-outside-write error: %+v", got)
	}
}

func TestRowOutsideWriteGlobMatch(t *testing.T) {
	set := map[string]any{"write": []any{"internal/cli/*_test.go", "internal/job/lint/"}, "acceptance": rowsAcceptance("internal/cli/x_test.go and internal/job/lint/rows.go hold it")}
	if got := vcRepoRule(lint.CheckJSON(rowsRecord(t, set), lint.Options{}), "job.row-outside-write"); len(got) != 0 {
		t.Fatalf("glob and directory entries cover the rows: %+v", got)
	}
}

func TestRowOutsideWriteClean(t *testing.T) {
	set := map[string]any{"write": []any{"main.go"}, "acceptance": rowsAcceptance("main.go builds and plan.md is untouched")}
	if got := vcRepoRule(lint.CheckJSON(rowsRecord(t, set), lint.Options{}), "job.row-outside-write"); len(got) != 0 {
		t.Fatalf("clean row flagged: %+v", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type intentResolver map[string]string

func (r intentResolver) Resolve(p string) (job.Source, error) {
	return job.Source{Pointer: p, Text: r[p]}, nil
}

func TestAfterMissingIntentText(t *testing.T) {
	body := rowsRecord(t, map[string]any{"acceptance": rowsAcceptance("The panel renders")})
	var rec map[string]any
	if err := json.Unmarshal(body, &rec); err != nil {
		t.Fatal(err)
	}
	intent := rec["intent"].(map[string]any)
	r := intentResolver{}
	for _, k := range []string{"owner", "item", "goal"} {
		r[intent[k].(string)] = "Plain text"
	}
	r[intent["item"].(string)] = "Step 6 builds on the lint from J-3"
	fs := lint.CheckJSON(body, lint.Options{Resolver: r})
	if got := vcRepoRule(fs, "job.after-missing"); len(got) != 1 {
		t.Fatalf("want one job.after-missing for a Job named only in the resolved intent text: %+v", fs)
	}
}
