package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func plannedText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/job/task/plan.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writePlan(t *testing.T, f *fixture, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(f.s.Feed.TaskDir, "plan.md"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPlanPublicationRetriesWithoutResettingJobs(t *testing.T) {
	f := setupPlan(t)
	text := plannedText(t)
	// A prior pass published J-1 before stopping; J-2 still needs publication.
	second := strings.ReplaceAll(text, "J-1", "J-2")
	second = strings.ReplaceAll(second, "- [ ] 1 ", "- [ ] 2 ")
	second = strings.ReplaceAll(second, "after: []", "after: [{job: J-1, kind: done, why: Reuse the documented format}]")
	writePlan(t, f, text+"\n"+second)
	approvePlan(t, f)
	first, err := job.LoadPlan(f.s.Feed.TaskDir, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := first.Record()
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	f.post(t, feed.Input{Type: "job", Job: "J-1", Body: body}, f.s.Writer)
	// Concurrent retries serialize publication and assignment through the pass lock.
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, err := f.s.Tick(); errs <- err })
	}
	wg.Wait()
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	before := f.state(t)
	if before.Status != contract.Assigned {
		t.Fatalf("first Job: %+v", before)
	}
	fence := before.Assign.Fence
	// Edits to an already published Job cannot replace its intent or reset its run.
	writePlan(t, f, strings.ReplaceAll(text+"\n"+second, "role: writer", "role: builder"))
	f.tick(t)
	after := f.state(t)
	if after.Assign.Fence != fence || after.Role != "writer" {
		t.Fatalf("plan reset published Job: %+v", after)
	}
	if records := planJobRecords(t, f); len(records) != 2 {
		t.Fatalf("duplicate or missing publication: %+v", records)
	}
	snap, err := Load(f.s.Feed)
	if err != nil || snap.State.Jobs["J-2"] == nil || snap.State.Jobs["J-2"].Assign != nil {
		t.Fatalf("dependent Job: %+v %v", snap.State.Jobs["J-2"], err)
	}
}

func TestPlanPublicationValidatesBeforeWriting(t *testing.T) {
	text := plannedText(t)
	// causes names a part of the one-line cause each case's notice must carry.
	causes := map[string]string{
		"missing field":      "plan.md Job J-2: missing required field skills",
		"invalid contract":   "plan.md Job J-2: $.review: value outside enum",
		"missing dependency": `after target "J-9" does not exist`,
		"cycle":              "Job cycle: J-2 -> J-2",
		"duplicate":          "plan.md: duplicate Job J-1",
	}
	for name, bad := range map[string]string{
		"missing field":      strings.Replace(text, "      skills: []\n", "", 1),
		"invalid contract":   strings.Replace(text, "review: L0", "review: L99", 1),
		"missing dependency": strings.Replace(text, "after: []", "after: [{job: J-9, kind: done, why: Reuse the documented format}]", 1),
		"cycle":              strings.Replace(text, "after: []", "after: [{job: J-1, kind: done, why: Reuse the documented format}]", 1),
		"duplicate":          text + "\n" + strings.Replace(text, "- [ ] 1 ", "- [ ] 2 ", 1),
	} {
		t.Run(name, func(t *testing.T) {
			f := setupPlan(t)
			if name != "duplicate" {
				bad = text + "\n" + strings.ReplaceAll(strings.ReplaceAll(bad, "J-1", "J-2"), "- [ ] 1 ", "- [ ] 2 ")
			}
			writePlan(t, f, bad)
			approvePlan(t, f)
			// An invalid plan no longer stops the tick; the Feed says why once.
			if _, err := f.s.Tick(); err != nil {
				t.Fatalf("invalid plan stopped the tick: %v", err)
			}
			if records := planJobRecords(t, f); len(records) != 0 {
				t.Fatalf("invalid plan published Jobs: %+v", records)
			}
			notices := hvPlanNotices(t, f)
			if len(notices) != 1 || !strings.Contains(notices[0], causes[name]) {
				t.Fatalf("want one notice naming %q, got %q", causes[name], notices)
			}
		})
	}
}

func setupPlan(t *testing.T) *fixture {
	t.Helper()
	f := setup(t)
	dir := filepath.Join(t.TempDir(), "T-01-example")
	if err := os.Rename(f.s.Feed.TaskDir, dir); err != nil {
		t.Fatal(err)
	}
	f.s.Feed.TaskDir = dir
	text := "---\nid: T-01\nulid: " + taskID + "\ntitle: Example\nproject: example\nsize: normal\nworkflow: ''\nstate: waiting\ncreated: 2026-01-01T00:00:00Z\n---\n"
	for name, body := range map[string]string{"task.md": text, "intent.md": "Example"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func approvePlan(t *testing.T, f *fixture) {
	t.Helper()
	text, err := (plan.Store{TaskDir: f.s.Feed.TaskDir}).Read()
	if err != nil {
		t.Fatal(err)
	}
	approveRevision(t, f, text)
	if !task.Approved(f.s.Feed.TaskDir) {
		t.Fatal("invalid approval fixture")
	}
}

// approveRevision posts the approve person Job of the plan and its accepting
// result for the plan revision text, the only approval record. The Job is J-90
// (J-91 for a second approval), beside the plan's own Jobs.
func approveRevision(t *testing.T, f *fixture, text string) {
	t.Helper()
	all, err := f.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	approved := 0
	for _, r := range all.Records {
		if r.Body["kind"] == "approve" {
			approved++
		}
	}
	id := "J-" + strconv.Itoa(90+approved)
	f.post(t, feed.Input{Type: "job", Job: id, Body: approvefix.JobBody(id)}, f.s.Writer)
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": id, "status": "accepted", "reason": "answered by dana", "answer": "approved", "plan_hash": plan.Hash(text), "plan_digest": plan.Digest(text)}}, f.s.Writer)
}

// planJobRecords are the job records of the plan's own Jobs, without the approve
// person Job that approvePlan posts.
func planJobRecords(t *testing.T, f *fixture) []feed.Entry {
	t.Helper()
	records, err := f.s.Feed.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	var out []feed.Entry
	for _, r := range records.Records {
		if r.Body["kind"] != "approve" {
			out = append(out, r)
		}
	}
	return out
}
