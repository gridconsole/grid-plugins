package job_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestPlanJobRecordRoundTrip(t *testing.T) {
	h := jobtest.New(t, testdata())
	j, err := job.LoadPlan(h.Task, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	body := validJobRecord(t, j)
	for _, field := range []string{"follow_up", "status", "review_reason"} {
		if _, ok := body[field]; ok {
			t.Errorf("unexpected record field %s", field)
		}
	}
	if string(body["posts_back"]) != "[]" {
		t.Fatalf("posts_back = %s, want []", body["posts_back"])
	}
	if bytes.Contains(body["verify"], []byte(`"min_tests"`)) {
		t.Fatalf("file-exists verify includes min_tests: %s", body["verify"])
	}
	after, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("Record changed the Job's brief representation")
	}

	j.Kind, j.Visible, j.Irreversible, j.ReviewReason = "build", "big", true, "The owner approved the review exception."
	body = validJobRecord(t, j)
	for field, want := range map[string]string{
		"kind": `"build"`, "visible": `"big"`, "irreversible": `true`,
		"review_reason": `"The owner approved the review exception."`,
	} {
		if string(body[field]) != want {
			t.Errorf("%s = %s, want %s", field, body[field], want)
		}
	}
	t.Run("invalid visible", func(t *testing.T) {
		j.Visible = "huge"
		data, err := j.Record()
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate("job-record", data); err == nil || err.Error() != "$.visible: value outside enum" {
			t.Fatalf("Validate = %v, want $.visible: value outside enum", err)
		}
	})
	t.Run("empty review reason", func(t *testing.T) {
		// Job's optional string cannot distinguish absent from explicitly empty.
		// Supply the empty field on the projected wire record to test the latter.
		body["review_reason"] = json.RawMessage(`""`)
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate("job-record", data); err == nil || err.Error() != "$.review_reason: string length" {
			t.Fatalf("Validate = %v, want $.review_reason: string length", err)
		}
	})
}

func validJobRecord(t *testing.T, j job.Job) map[string]json.RawMessage {
	t.Helper()
	data, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestTestdataPlanJobRecords(t *testing.T) {
	var cases []struct{ task, id string }
	err := filepath.WalkDir(testdata(), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "plan.md" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		items, err := plan.Items(string(data))
		if err != nil {
			return err
		}
		for _, item := range items {
			for _, raw := range item.Jobs {
				id, _ := raw["id"].(string)
				cases = append(cases, struct{ task, id string }{filepath.Dir(path), id})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no planned Jobs found in testdata")
	}
	for _, tc := range cases {
		t.Run(filepath.ToSlash(tc.task)+"/"+tc.id, func(t *testing.T) {
			h := jobtest.New(t, testdata())
			data, err := os.ReadFile(filepath.Join(tc.task, "plan.md"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(h.Task, "plan.md"), data, 0600); err != nil {
				t.Fatal(err)
			}
			h.Approve(t)
			j, err := job.LoadPlan(h.Task, tc.id)
			if err != nil {
				t.Fatal(err)
			}
			validJobRecord(t, j)
		})
	}
}

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
