package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// Publication works from the plan text it read once; a file replaced after that
// read must not change what is published.
func TestPlanPublicationChecksReadSnapshot(t *testing.T) {
	for _, snapshotRole := range []string{"builder", "writer"} {
		t.Run(snapshotRole+" snapshot", func(t *testing.T) {
			f := setupPlan(t)
			original := plannedText(t)
			changed := strings.Replace(original, "role: writer", "role: builder", 1)
			snapshot, replacement := original, changed
			if snapshotRole == "builder" {
				snapshot, replacement = changed, original
			}
			writePlan(t, f, snapshot)
			snap, err := Load(f.s.Feed)
			if err != nil {
				t.Fatal(err)
			}
			reads := 0
			err = f.s.publishPlanWithReader(snap, &memory{}, func() (string, error) {
				text, err := (plan.Store{TaskDir: f.s.Feed.TaskDir}).Read()
				reads++
				if reads == 2 {
					// Replace the file after the exact publication read, without timing.
					writePlan(t, f, replacement)
				}
				return text, err
			})
			if err != nil || reads != 2 {
				t.Fatalf("publication reads=%d err=%v", reads, err)
			}
			published, err := Load(f.s.Feed)
			if err != nil || len(published.Items) != 1 || published.Items[0].Role != snapshotRole {
				t.Fatalf("publication did not retain the snapshot (want role %s): %+v err=%v", snapshotRole, published.Items, err)
			}
		})
	}
}

func TestUnapprovedPlanPublishesJobs(t *testing.T) {
	f := setup(t)
	writePlan(t, f, plannedText(t))
	if _, err := f.s.Tick(); err != nil {
		t.Fatal(err)
	}
	records, err := f.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil || len(records.Records) == 0 {
		t.Fatalf("unapproved plan published %d Jobs: %v", len(records.Records), err)
	}
}

func rewritePlanFile(t *testing.T, dir, name, old, next string) {
	t.Helper()
	path := filepath.Join(dir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(b), old, next, 1)), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestMalformedPlanDoesNotBlockOtherTasks(t *testing.T) {
	bad, good := setupPlan(t), setupPlan(t)
	text := plannedText(t)
	writePlan(t, bad, strings.Replace(text, "      skills: []\n", "", 1))
	writePlan(t, good, text)
	approvePlan(t, bad)
	approvePlan(t, good)
	badResult, goodResult := make(chan error, 1), make(chan error, 1)
	go func() { _, err := bad.s.Tick(); badResult <- err }()
	go func() { _, err := good.s.Tick(); goodResult <- err }()
	if err := <-badResult; err != nil {
		t.Fatalf("malformed plan stopped its tick: %v", err)
	}
	if err := <-goodResult; err != nil {
		t.Fatalf("other task stopped: %v", err)
	}
	if got := good.state(t); got == nil || got.Assign == nil {
		t.Fatal("other task was not assigned")
	}
	if records := planJobRecords(t, bad); len(records) != 0 {
		t.Fatalf("malformed plan published: %+v", records)
	}
	// Fixing the malformed task lets its next pass proceed independently too.
	writePlan(t, bad, text)
	approvePlan(t, bad)
	if out := bad.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
}
