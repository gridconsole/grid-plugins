package contract

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// A oneOf that matches no branch reports the branch that got furthest into the
// value, so a Job with verify.min_tests 0 is refused for min_tests.
func TestOneOfReportsTheDeepestBranch(t *testing.T) {
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	body["verify"] = map[string]any{"command": "go test ./...", "min_tests": 0}
	data, _ := json.Marshal(body)
	err = Validate("job-record", data)
	if err == nil || !strings.Contains(err.Error(), "min_tests") {
		t.Fatalf("want an error naming min_tests, got %v", err)
	}
}

func resultFixture(t *testing.T) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("../../contract/fixtures/result/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(raw, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}

// A full inline result body missing one required key is refused for that key,
// not for the one-key ref branch that sits later in the oneOf.
func TestOneOfTieNamesTheBranchTheBodyMatches(t *testing.T) {
	rec := resultFixture(t)
	delete(rec["body"].(map[string]any), "knowledge")
	data, _ := json.Marshal(rec)
	err := Validate("result", data)
	if err == nil || !strings.Contains(err.Error(), "missing required knowledge") || strings.Contains(err.Error(), "missing required ref") {
		t.Fatalf("want missing required knowledge, got %v", err)
	}
}

// The bare external body the store writes still validates.
func TestOneOfBareRefBodyStillValidates(t *testing.T) {
	rec := resultFixture(t)
	rec["body"] = map[string]any{"ref": "bodies/01K9J000000000000000000001.json"}
	data, _ := json.Marshal(rec)
	if err := Validate("result", data); err != nil {
		t.Fatalf("bare ref body refused: %v", err)
	}
}
