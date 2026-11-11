package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

const ftHintGeneric = "Fix the field the error names so the record matches the job-record schema."

func ftHintRead(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", rel))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// ftHintSchema returns the job-record schema as the file in contract/schemas holds it.
func ftHintSchema(t *testing.T) map[string]any {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(ftHintRead(t, "schemas/job-record.json"), &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

// ftHintBasic returns the valid basic record as a map to mutate.
func ftHintBasic(t *testing.T) map[string]any {
	t.Helper()
	var rec map[string]any
	if err := json.Unmarshal(ftHintRead(t, "fixtures/job-record/valid/basic.json"), &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

func ftHintErr(t *testing.T, rec map[string]any) error {
	t.Helper()
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	verr := contract.Validate("job-record", data)
	if verr == nil {
		t.Fatalf("record passed the schema: %s", data)
	}
	return verr
}

func ftHintEnum(t *testing.T, node map[string]any) []string {
	t.Helper()
	list, ok := node["enum"].([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("schema node has no enum: %v", node)
	}
	out := []string{}
	for _, v := range list {
		out = append(out, v.(string))
	}
	return out
}

func TestFtHintEnumListsEverySchemaValue(t *testing.T) {
	schema := ftHintSchema(t)
	props := schema["properties"].(map[string]any)
	rel := props["links"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["rel"].(map[string]any)
	cases := []struct {
		name   string
		mutate func(rec map[string]any)
		values []string
		at     string
	}{
		{"top level", func(rec map[string]any) { rec["visible"] = "huge" }, ftHintEnum(t, props["visible"].(map[string]any)), "$.visible"},
		{"in an array item", func(rec map[string]any) {
			rec["links"] = []any{map[string]any{"to": "J-2", "rel": "bogus", "why": "example"}}
		}, ftHintEnum(t, rel), "$.links[0].rel"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := ftHintBasic(t)
			c.mutate(rec)
			err := ftHintErr(t, rec)
			if !strings.Contains(err.Error(), "value outside enum") {
				t.Fatalf("setup: not an enum failure: %v", err)
			}
			hint := contract.Hint("job-record", err)
			if hint == "" {
				t.Fatalf("no hint for %v", err)
			}
			if !strings.Contains(hint, c.at) {
				t.Errorf("hint %q does not name the path %s", hint, c.at)
			}
			for _, v := range c.values {
				if !strings.Contains(hint, v) {
					t.Errorf("hint %q lacks enum value %q", hint, v)
				}
			}
		})
	}
}

func TestFtHintUnknownPropertyNamesKnownProperties(t *testing.T) {
	rec := ftHintBasic(t)
	rec["priority"] = "high"
	err := ftHintErr(t, rec)
	if !strings.Contains(err.Error(), "unknown property priority") {
		t.Fatalf("setup: %v", err)
	}
	hint := contract.Hint("job-record", err)
	if hint == "" {
		t.Fatalf("no hint for %v", err)
	}
	known := []string{}
	for k := range ftHintSchema(t)["properties"].(map[string]any) {
		known = append(known, k)
	}
	sort.Strings(known)
	for _, k := range known {
		if !strings.Contains(hint, k) {
			t.Errorf("hint %q lacks known property %q", hint, k)
		}
	}
	if !strings.Contains(hint, `"priority"`) {
		t.Errorf("hint %q does not name the unknown property", hint)
	}
}

func TestFtHintMissingRequiredNamesField(t *testing.T) {
	rec := ftHintBasic(t)
	delete(rec["intent"].(map[string]any), "goal")
	err := ftHintErr(t, rec)
	if !strings.Contains(err.Error(), "missing required goal") {
		t.Fatalf("setup: %v", err)
	}
	hint := contract.Hint("job-record", err)
	if !strings.Contains(hint, `"goal"`) || !strings.Contains(hint, "$.intent") {
		t.Errorf("hint %q does not name the field goal at $.intent", hint)
	}
	top := ftHintBasic(t)
	delete(top, "title")
	if hint := contract.Hint("job-record", ftHintErr(t, top)); !strings.Contains(hint, `"title"`) || !strings.Contains(hint, "the record") {
		t.Errorf("top-level hint %q does not name title", hint)
	}
}

func TestFtHintEmptyWhenItCannotTell(t *testing.T) {
	rec := ftHintBasic(t)
	rec["order"] = -1
	err := ftHintErr(t, rec)
	if got := contract.Hint("job-record", err); got != "" {
		t.Errorf("below-minimum failure got hint %q", got)
	}
	if got := contract.Hint("job-record", nil); got != "" {
		t.Errorf("nil error got hint %q", got)
	}
	rec = ftHintBasic(t)
	rec["visible"] = "huge"
	if got := contract.Hint("no-such-schema", ftHintErr(t, rec)); got != "" {
		t.Errorf("unknown schema got hint %q", got)
	}
}

func TestFtHintMessagesUnchanged(t *testing.T) {
	rec := ftHintBasic(t)
	rec["visible"] = "huge"
	if err := ftHintErr(t, rec); err.Error() != "$.visible: value outside enum" {
		t.Errorf("message changed: %q", err)
	}
}

// Every invalid job-record fixture goes through CheckJSON: the Fix is the hint
// where Hint has text, the generic line where it has none.
func TestFtHintCheckJSONFixOverInvalidFixtures(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", "job-record", "invalid", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no invalid fixtures: %v", err)
	}
	hinted, generic := 0, 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		verr := contract.Validate("job-record", data)
		if verr == nil {
			t.Errorf("%s: fixture passes the schema", file)
			continue
		}
		hint := contract.Hint("job-record", verr)
		findings := lint.CheckJSON(data, lint.Options{})
		if len(findings) != 1 || findings[0].Rule != "job.record" {
			t.Errorf("%s: findings %+v", file, findings)
			continue
		}
		f := findings[0]
		if f.Message != verr.Error() {
			t.Errorf("%s: message %q is not the schema error %q", file, f.Message, verr)
		}
		if hint != "" {
			hinted++
			if f.Fix == ftHintGeneric || f.Fix != hint {
				t.Errorf("%s: Fix %q, want the hint %q", file, f.Fix, hint)
			}
		} else {
			generic++
			if f.Fix != ftHintGeneric {
				t.Errorf("%s: Fix %q, want the generic line", file, f.Fix)
			}
		}
	}
	if hinted == 0 || generic == 0 {
		t.Errorf("fixtures cover %d hinted and %d generic findings; want both", hinted, generic)
	}
}
