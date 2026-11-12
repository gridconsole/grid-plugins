package job_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// vcRepoPlanned is the basic Job record cut down to the planned fields, as a plan entry carries them.
func vcRepoPlanned(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(b, &record); err != nil {
		t.Fatal(err)
	}
	planned := map[string]any{}
	for _, key := range job.PlannedFields {
		planned[key] = record[key]
	}
	return planned
}

// vcRepoOf reads the repo a Job carries through its JSON, the shape a Job record is written in.
func vcRepoOf(t *testing.T, j job.Job) (string, bool) {
	t.Helper()
	b, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	repo, ok := m["repo"].(string)
	return repo, ok
}

// Row 1: a planned Job may name its repo, and the decoded Job carries it.
func TestVcRepoPlannedJobNamesItsRepo(t *testing.T) {
	raw := vcRepoPlanned(t)
	raw["repo"] = "app"
	j, problems := job.DecodePlanned(raw, "build")
	if len(problems) != 0 {
		t.Fatalf("DecodePlanned problems: %v", problems)
	}
	if repo, _ := vcRepoOf(t, j); repo != "app" {
		t.Fatalf("Job repo = %q, want app", repo)
	}
}

// Row 1: a planned Job that names no repo decodes as before and writes no repo key.
func TestVcRepoPlannedJobWithoutRepo(t *testing.T) {
	j, problems := job.DecodePlanned(vcRepoPlanned(t), "build")
	if len(problems) != 0 {
		t.Fatalf("DecodePlanned problems: %v", problems)
	}
	if repo, ok := vcRepoOf(t, j); ok {
		t.Fatalf("a Job naming no repo writes repo %q", repo)
	}
}

// The rendered brief keeps the repo among the optional fields, and the brief reads back with it.
func TestVcRepoBriefMetadataKeepsTheRepo(t *testing.T) {
	raw := vcRepoPlanned(t)
	raw["repo"] = "plugin"
	j, problems := job.DecodePlanned(raw, "build")
	if len(problems) != 0 {
		t.Fatalf("DecodePlanned problems: %v", problems)
	}
	meta, err := job.Metadata(j)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(meta, "\nrepo: \"plugin\"\n") {
		t.Fatalf("metadata has no repo line:\n%s", meta)
	}
	back, _, err := job.DecodeBrief(meta + "\n## Intent\n")
	if err != nil {
		t.Fatal(err)
	}
	if repo, _ := vcRepoOf(t, back); repo != "plugin" {
		t.Fatalf("brief reads back repo %q, want plugin", repo)
	}
	none, _ := job.DecodePlanned(vcRepoPlanned(t), "build")
	if meta, err = job.Metadata(none); err != nil || strings.Contains(meta, "\nrepo:") {
		t.Fatalf("a Job naming no repo renders a repo line (%v):\n%s", err, meta)
	}
}
