package contract

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// The review-job skill has to tell the reviewer to post each finding as a found
// record under the review Job's lease, before the verdict, and to list the ids
// in the result's found list. These tests read the shipped skill text.

const (
	kbRvSkill      = "skills/review-job/SKILL.md"
	kbRvResultSpan = "`grid job result --task <task> --job <review-job> --body <json>`"
)

// kbRvFoundSpan returns the code span that starts `grid job post` and holds
// --type found, and where it starts in text.
func kbRvFoundSpan(text string) (string, int, bool) {
	for _, loc := range codeSpanPattern.FindAllStringSubmatchIndex(text, -1) {
		span := text[loc[2]:loc[3]]
		if strings.HasPrefix(span, "grid job post") && strings.Contains(span, "--type found") {
			return span, loc[0], true
		}
	}
	return "", 0, false
}

// kbRvBodySpan returns the code span that holds the found body example.
func kbRvBodySpan(text string) (string, bool) {
	for _, m := range codeSpanPattern.FindAllStringSubmatch(text, -1) {
		if strings.HasPrefix(m[1], `{"job":`) {
			return m[1], true
		}
	}
	return "", false
}

func TestKbRvSkillPostsFound(t *testing.T) {
	text := readPluginFile(t, kbRvSkill)
	span, _, ok := kbRvFoundSpan(text)
	if !ok {
		t.Fatal("review-job skill has no code span that starts `grid job post` and holds --type found")
	}
	if !strings.Contains(span, "--job <review-job>") {
		t.Errorf("found span %q does not pass --job <review-job>", span)
	}
	if !strings.Contains(text, "security") {
		t.Error("review-job skill does not name security")
	}
	if !strings.Contains(text, "ids") || !strings.Contains(text, "`found`") {
		t.Error("review-job skill does not say the printed ids go in `found`")
	}
}

func TestKbRvFoundBeforeVerdict(t *testing.T) {
	text := readPluginFile(t, kbRvSkill)
	_, foundAt, ok := kbRvFoundSpan(text)
	if !ok {
		t.Fatal("review-job skill has no `grid job post ... --type found` span")
	}
	resultAt := strings.Index(text, kbRvResultSpan)
	if resultAt < 0 {
		t.Fatalf("review-job skill lacks the span %s", kbRvResultSpan)
	}
	if foundAt > resultAt {
		t.Errorf("the --type found span (offset %d) comes after the result span (offset %d)", foundAt, resultAt)
	}

	// The body fields the text names are the required fields of found.json,
	// read from the schema file.
	var schema struct {
		AllOf []struct {
			Properties struct {
				Body struct {
					OneOf []struct {
						Required []string `json:"required"`
					} `json:"oneOf"`
				} `json:"body"`
			} `json:"properties"`
		} `json:"allOf"`
	}
	raw, err := contractdata.Schemas.ReadFile("schemas/found.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, part := range schema.AllOf {
		for _, branch := range part.Properties.Body.OneOf {
			if len(branch.Required) > 0 && branch.Required[0] != "ref" {
				want = branch.Required
			}
		}
	}
	if len(want) == 0 {
		t.Fatal("found.json has no inline body branch with required fields")
	}
	body, ok := kbRvBodySpan(text)
	if !ok {
		t.Fatal(`review-job skill has no body example span that starts {"job":`)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(body), &fields); err != nil {
		t.Fatalf("found body example is not JSON: %v", err)
	}
	var got []string
	for k := range fields {
		got = append(got, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("body example fields %v are not the required fields %v of found.json", got, want)
	}
}
