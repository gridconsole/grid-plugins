package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// icjTitled is personBody with the title set.
func icjTitled(t *testing.T, id, title string) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, id, nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["title"] = title
	out, _ := json.Marshal(body)
	return string(out)
}

// icjStateTitle reads a Job's title from feed state --json.
func icjStateTitle(t *testing.T, h *harness, path, job string) string {
	t.Helper()
	if code := h.run(path, "feed", "state", "--json"); code != 0 {
		t.Fatalf("feed state: %d %s", code, h.errb.String())
	}
	var st struct {
		Jobs map[string]struct {
			Title string `json:"title"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &st); err != nil {
		t.Fatal(err, h.out.String())
	}
	return st.Jobs[job].Title
}

// icjJobRecords counts the job records on the Feed that carry Job id id.
func icjJobRecords(t *testing.T, store feed.Store, id string) int {
	t.Helper()
	res, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range res.Records {
		if got, _ := e.Body["id"].(string); got == id {
			n++
		}
	}
	return n
}

func TestIcjJobPostExistsCLI(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-2", "--body", icjTitled(t, "J-2", "Second A")); code != 0 {
		t.Fatalf("first J-2: %d %s", code, h.errb.String())
	}
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-2", "--body", icjTitled(t, "J-2", "Second B"), "--json"); code != 3 {
		t.Fatalf("second J-2 exit = %d, want 3: %s", code, h.errb.String())
	}
	var out struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil || out.Error.Code != "job_exists" {
		t.Fatalf("stdout = %q (%v), want error.code job_exists", h.out.String(), err)
	}
	if !strings.Contains(h.errb.String(), "Job J-2 already exists") || out.Error.Message != strings.TrimSuffix(h.errb.String(), "\n") {
		t.Fatalf("stderr = %q, message = %q", h.errb.String(), out.Error.Message)
	}
	t.Setenv("GRID_LEASE_EPOCH", "")
	if code := h.run(path, "feed", "post", "--type", "job", "--job", "J-2", "--body", icjTitled(t, "J-2", "Second C")); code != 3 || !strings.Contains(h.errb.String(), "Job J-2 already exists") {
		t.Fatalf("feed post J-2: %d %s", code, h.errb.String())
	}
	if got := icjStateTitle(t, h, path, "J-2"); got != "Second A" {
		t.Fatalf("J-2 title = %q, want Second A", got)
	}
	if n := icjJobRecords(t, store, "J-2"); n != 1 {
		t.Fatalf("J-2 records = %d, want 1", n)
	}
}

func TestIcjUnleasedKeepsFirstJob(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	before := icjStateTitle(t, h, path, "J-1")
	if before == "" || before == "Other title" {
		t.Fatalf("leased J-1 title = %q", before)
	}
	t.Setenv("GRID_ROLE", "member")
	t.Setenv("GRID_LEASE_EPOCH", "")
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-1", "--body", icjTitled(t, "J-1", "Other title")); code != 3 || !strings.Contains(h.errb.String(), "Job J-1 already exists") {
		t.Fatalf("unleased J-1: %d %s", code, h.errb.String())
	}
	if got := icjStateTitle(t, h, path, "J-1"); got != before {
		t.Fatalf("J-1 title = %q, want %q", got, before)
	}
	if n := icjJobRecords(t, store, "J-1"); n != 1 {
		t.Fatalf("J-1 records = %d, want 1", n)
	}
}

func TestIcjErrorCode(t *testing.T) {
	err := fmt.Errorf("x: %w", &feed.JobExistsError{Job: "J-2", Post: "p"})
	if CodeJobExists != "job_exists" {
		t.Fatalf("CodeJobExists = %q", CodeJobExists)
	}
	if got := errorCode(err); got != CodeJobExists {
		t.Fatalf("code = %q, want job_exists", got)
	}
	if got := exitCode(err); got != 3 {
		t.Fatalf("exit = %d, want 3", got)
	}
	if got := errorCode(errors.New("Job J-2 already exists (post p); revisions need a new Job")); got != "" {
		t.Fatalf("wording alone gave code %q", got)
	}
}
