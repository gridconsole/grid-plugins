package job_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func TestJobPlannedFieldFixtures(t *testing.T) {
	for _, tc := range []struct {
		file string
		want string
	}{
		{"valid/planned-fields.json", ""},
		{"invalid/visible-not-enum.json", "$.visible: value outside enum"},
		{"invalid/review-reason-empty.json", "$.review_reason: string length"},
		{"invalid/irreversible-not-boolean.json", "$.irreversible: expected boolean"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "job-record", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			err = contract.Validate("job-record", data)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("Validate = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestJobPlannedFieldSchema(t *testing.T) {
	data, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, field string
		value       any
		valid       bool
	}{
		{"future kind", "kind", "future-kind", true},
		{"short kind", "kind", "a", true},
		{"longest kind", "kind", strings.Repeat("a", 40), true},
		{"kind too long", "kind", strings.Repeat("a", 41), false},
		{"empty kind", "kind", "", false},
		{"capital kind", "kind", "Build", false},
		{"numeric kind", "kind", "build2", false},
		{"leading hyphen", "kind", "-build", false},
		{"nonstring kind", "kind", 1, false},
		{"small", "visible", "small", true},
		{"big", "visible", "big", true},
		{"null visible", "visible", nil, false},
		{"reversible", "irreversible", false, true},
		{"irreversible", "irreversible", true, true},
		{"numeric irreversible", "irreversible", 1, false},
		{"short reason", "review_reason", "x", true},
		{"longest reason", "review_reason", strings.Repeat("x", 500), true},
		{"reason too long", "review_reason", strings.Repeat("x", 501), false},
		{"nonstring reason", "review_reason", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			if err := json.Unmarshal(data, &body); err != nil {
				t.Fatal(err)
			}
			body[tc.field] = tc.value
			encoded, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			err = contract.Validate("job-record", encoded)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.HasPrefix(err.Error(), "$."+tc.field+":") {
				t.Fatalf("Validate = %v, want failure at $.%s", err, tc.field)
			}
		})
	}
}

func TestJobCompatibilityFieldsRoundTrip(t *testing.T) {
	for _, populated := range []bool{false, true} {
		j := job.Job{}
		if populated {
			j.Defaults()
		}
		data, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"status", "follow_up"} {
			if _, present := fields[name]; present != populated {
				t.Errorf("%s present = %v, want %v", name, present, populated)
			}
		}
		var decoded job.Job
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Status != j.Status || decoded.FollowUp != j.FollowUp {
			t.Fatalf("compatibility fields changed: %+v", decoded)
		}
	}
}
