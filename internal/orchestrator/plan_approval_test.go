package orchestrator

import (
	"bytes"
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

// The one remaining hold is a hard-linked plan.md. The notice is written once per
// blocked stretch and again only after publication was unblocked in between.
func TestPlanPublicationNoticeRepeatsOnlyAfterUnblocked(t *testing.T) {
	t.Parallel()
	f := setupPlan(t)
	text := plannedText(t)
	writePlan(t, f, text)
	planPath := filepath.Join(f.s.Feed.TaskDir, "plan.md")
	linkPath := filepath.Join(f.s.Feed.TaskDir, "plan-link.md")
	if err := os.Link(planPath, linkPath); err != nil {
		t.Skip("hard links unavailable:", err)
	}
	var logs bytes.Buffer
	f.s.Diagnostics = &logs
	const notice = "plan publication skipped: workflow requires approval of the current plan; run grid approve before Jobs can post\n"
	check := func(lines int) {
		t.Helper()
		if got, want := logs.String(), strings.Repeat(notice, lines); got != want {
			t.Fatalf("diagnostics = %q, want %q", got, want)
		}
	}
	before, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if out := f.tick(t); out.Assigned != 0 {
			t.Fatalf("blocked task assigned Jobs: %+v", out)
		}
		check(1)
	}
	after, err := f.s.Feed.Read(feed.Filter{})
	if err != nil || len(after.Records) != len(before.Records) {
		t.Fatalf("blocked ticks changed Feed: %+v %v", after, err)
	}
	if err := os.Remove(linkPath); err != nil {
		t.Fatal(err)
	}
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatalf("unblocked assignments = %d, want 1", out.Assigned)
	}
	check(1)
	// Blocking again after an unblocked pass writes the notice a second time.
	if err := os.Link(planPath, linkPath); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		f.tick(t)
		check(2)
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
	if err := <-badResult; err == nil {
		t.Fatal("malformed plan was accepted")
	}
	if err := <-goodResult; err != nil {
		t.Fatalf("other task stopped: %v", err)
	}
	if got := good.state(t); got == nil || got.Assign == nil {
		t.Fatal("other task was not assigned")
	}
	records, err := bad.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil || len(records.Records) != 0 {
		t.Fatalf("malformed plan published: %+v %v", records, err)
	}
	// Fixing the malformed task lets its next pass proceed independently too.
	writePlan(t, bad, text)
	approvePlan(t, bad)
	if out := bad.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
}
