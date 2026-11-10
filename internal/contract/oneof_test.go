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
