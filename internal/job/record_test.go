package job_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func TestJobRecordStatus(t *testing.T) {
	j, err := job.LoadPlan("../../testdata/job/task", "J-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{
		"", "briefed", "done", "blocked", "posted", "ready", "assigned", "running",
		"submitted", "integrated", "accepted", "stale", "parked", "rejected", "conflict", "cancelled",
	} {
		t.Run(status, func(t *testing.T) {
			j.Status = status
			j.CloseReason = nil
			if status == "rejected" || status == "cancelled" {
				reason := "The owner closed this Job."
				j.CloseReason = &reason
			}
			body := validJobRecord(t, j)
			switch status {
			case "", "briefed", "done", "blocked":
				if _, ok := body["status"]; ok {
					t.Fatalf("brief status %q leaked into record", status)
				}
			default:
				if string(body["status"]) != `"`+status+`"` {
					t.Fatalf("status = %s, want %q", body["status"], status)
				}
			}
		})
	}
	j.Status = "unknown"
	data, err := j.Record()
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err == nil || !strings.HasPrefix(err.Error(), "$.status:") {
		t.Fatalf("unknown status was hidden: %v", err)
	}
}

func TestJobRecordVerify(t *testing.T) {
	j, err := job.LoadPlan("../../testdata/job/task", "J-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		verify job.Verify
		valid  bool
	}{
		{"file exists", job.Verify{Kind: "file-exists", Path: "docs/export.md"}, true},
		{"reviewer check", job.Verify{Kind: "reviewer-check", Job: "J-2"}, true},
		{"command", job.Verify{Command: "go test ./...", MinTests: 1}, true},
		{"command zero tests", job.Verify{Command: "go test ./..."}, false},
		{"command negative tests", job.Verify{Command: "go test ./...", MinTests: -1}, false},
		{"file exists with tests", job.Verify{Kind: "file-exists", Path: "docs/export.md", MinTests: 1}, false},
		{"reviewer check with tests", job.Verify{Kind: "reviewer-check", Job: "J-2", MinTests: 1}, false},
		{"mixed branches", job.Verify{Command: "go test ./...", MinTests: 1, Kind: "file-exists", Path: "docs/export.md"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j.Verify = tc.verify
			data, err := j.Record()
			if err != nil {
				t.Fatal(err)
			}
			err = contract.Validate("job-record", data)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				var decoded job.Job
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decoded.Verify, tc.verify) {
					t.Fatalf("verify = %+v, want %+v", decoded.Verify, tc.verify)
				}
			} else if err == nil || !strings.HasPrefix(err.Error(), "$.verify:") {
				t.Fatalf("Validate = %v, want failure at $.verify", err)
			}
		})
	}
}

func TestJobRecordPostsBack(t *testing.T) {
	j, err := job.LoadPlan("../../testdata/job/task", "J-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, posts := range [][]string{nil, {}, {"result", "question", "knowledge", "checkpoint"}} {
		j.PostsBack = posts
		body := validJobRecord(t, j)
		want := `[]`
		if len(posts) > 0 {
			want = `["result","question","knowledge","checkpoint"]`
		}
		if string(body["posts_back"]) != want {
			t.Fatalf("posts_back = %s, want %s", body["posts_back"], want)
		}
	}
}
