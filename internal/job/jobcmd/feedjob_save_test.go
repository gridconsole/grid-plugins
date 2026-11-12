package jobcmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

// stubFeed hands back a fixed Feed read for the test and restores readFeed after.
func stubFeed(t *testing.T, fn func(string, feed.Filter) (feed.ReadResult, error)) {
	t.Helper()
	was := readFeed
	t.Cleanup(func() { readFeed = was })
	readFeed = fn
}

func planTask(t *testing.T) string {
	t.Helper()
	e, _, _ := fixture(t)
	return filepath.Join(e.Cwd, "T-01")
}

func TestLoadJobFromPlanDoesNotReadTheFeed(t *testing.T) {
	task := planTask(t)
	stubFeed(t, func(string, feed.Filter) (feed.ReadResult, error) {
		t.Fatal("the Feed was read although plan.md holds the Job")
		return feed.ReadResult{}, nil
	})
	j, err := loadJob(task, "J-1")
	if err != nil || j.ID != "J-1" {
		t.Fatalf("loadJob(J-1) = %+v, %v; want the plan Job", j.ID, err)
	}
}

func TestLoadJobFallsBackToTheFeedWhenThePlanHasNoSuchJob(t *testing.T) {
	task := planTask(t)
	stubFeed(t, func(string, feed.Filter) (feed.ReadResult, error) {
		return feed.ReadResult{Records: []feed.Entry{entry("R1", "J-2", 2, "posted")}}, nil
	})
	j, err := loadJob(task, "J-2")
	if err != nil || j.ID != "J-2" || j.Title != "posted" {
		t.Fatalf("loadJob(J-2) = %q %q, %v; want the posted Job", j.ID, j.Title, err)
	}
}

func TestLoadJobWithEmptyFeedReturnsThePlanError(t *testing.T) {
	task := planTask(t)
	stubFeed(t, func(string, feed.Filter) (feed.ReadResult, error) { return feed.ReadResult{}, nil })
	_, err := loadJob(task, "J-2")
	if !errors.Is(err, job.ErrJobNotInPlan) {
		t.Fatalf("loadJob with no plan item and an empty Feed: %v; want job.ErrJobNotInPlan", err)
	}
}

func TestLoadJobJoinsThePlanErrorAndTheFeedReadError(t *testing.T) {
	task := planTask(t)
	feedErr := errors.New("feed store unreadable")
	stubFeed(t, func(string, feed.Filter) (feed.ReadResult, error) { return feed.ReadResult{}, feedErr })
	_, err := loadJob(task, "J-2")
	if !errors.Is(err, job.ErrJobNotInPlan) || !errors.Is(err, feedErr) {
		t.Fatalf("loadJob = %v; want both the plan error and the Feed error", err)
	}
}

func TestPickJobSelectsByItemNumber(t *testing.T) {
	records := []feed.Entry{entry("R1", "J-1", 1, "one"), entry("R2", "J-3", 3, "three")}
	j, err := pickJob(records, "3")
	if err != nil || j == nil || j.ID != "J-3" || j.Title != "three" {
		t.Fatalf("pickJob(3) = %+v, %v; want J-3", j, err)
	}
}

func TestPickJobNeverChoosesAStaleRecord(t *testing.T) {
	stale := entry("R1", "J-5", 5, "stale only")
	stale.Verdict = "stale"
	if j, err := pickJob([]feed.Entry{stale}, "J-5"); err != nil || j != nil {
		t.Fatalf("pickJob over a stale-only Job = %+v, %v; want nil", j, err)
	}
	if j, err := pickJob([]feed.Entry{stale}, "5"); err != nil || j != nil {
		t.Fatalf("pickJob by item over a stale-only Job = %+v, %v; want nil", j, err)
	}
	// A stale record newer than a counted one does not displace it.
	older := entry("R2", "J-6", 6, "counted")
	newer := entry("R3", "J-6", 6, "stale newer")
	newer.Verdict = "stale"
	j, err := pickJob([]feed.Entry{older, newer}, "J-6")
	if err != nil || j == nil || j.Title != "counted" {
		t.Fatalf("pickJob = %+v, %v; want the counted record", j, err)
	}
}

func TestIdentityTakesTheRecordJobWhenTheBodyHasNoID(t *testing.T) {
	e := entry("R1", "", 4, "no body id")
	delete(e.Body, "id")
	name := "J-4"
	e.Job = &name
	id, item := identity(e)
	if id != "J-4" || item != 4 {
		t.Fatalf("identity = %q, %d; want J-4, 4", id, item)
	}
	j, err := pickJob([]feed.Entry{e}, "J-4")
	if err != nil || j == nil || j.ID != "J-4" {
		t.Fatalf("pickJob = %+v, %v; want Job J-4", j, err)
	}
}

func TestDecodeJobFillsTheSelectorIDAndDefaults(t *testing.T) {
	e := entry("R1", "", 4, "bare")
	delete(e.Body, "id")
	j, err := decodeJob(e, "J-4")
	if err != nil || j == nil {
		t.Fatalf("decodeJob = %+v, %v", j, err)
	}
	if j.ID != "J-4" || j.Status != "briefed" || j.FollowUp != "none" || j.Attempt != 1 {
		t.Errorf("id %q status %q follow_up %q attempt %d; want J-4, briefed, none, 1", j.ID, j.Status, j.FollowUp, j.Attempt)
	}
}

func TestDecodeJobNamesRecordAndFieldForAStringWhereAListIsWanted(t *testing.T) {
	e := entry("R9", "J-2", 2, "bad after")
	e.Body["after"] = "J-1"
	_, err := decodeJob(e, "J-2")
	if err == nil {
		t.Fatal("a string `after` should not decode")
	}
	for _, want := range []string{"R9", "field after", "want ", ", got string"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestBadFieldWithoutATypedPathNamesTheFirstBadKeyInSortedOrder(t *testing.T) {
	// A dependency object with a too-short why fails in its own decoder, with a
	// plain error and no field path; max_rounds is wrong too but sorts later.
	e := entry("R1", "J-2", 2, "two bad keys")
	e.Body["max_rounds"] = "many"
	e.Body["after"] = []any{map[string]any{"job": "J-1", "why": "x"}}
	_, err := decodeJob(e, "J-2")
	if err == nil {
		t.Fatal("decode should fail")
	}
	if !strings.Contains(err.Error(), "field after:") || strings.Contains(err.Error(), "field max_rounds") {
		t.Errorf("error %q; want the sorted-first bad key `after`", err)
	}
}

func TestBadFieldAndProblemFallbacks(t *testing.T) {
	plain := errors.New("boom")
	if got := badField(map[string]any{"title": "fine", "item": float64(1)}, plain); got != "(unknown)" {
		t.Errorf("badField with no bad key = %q; want (unknown)", got)
	}
	if got := problem(plain); got != "boom" {
		t.Errorf("problem(plain error) = %q; want its text", got)
	}
}

func saveFixture(brief string) (job.Job, job.Rendered) {
	return job.Job{ID: "J-1"}, job.Rendered{Brief: brief}
}

func TestSaveRefusesAnInvalidID(t *testing.T) {
	task := t.TempDir()
	_, r := saveFixture("brief")
	_, err := save(task, job.Job{ID: "bad id"}, r)
	if err == nil || !strings.Contains(err.Error(), "invalid Job id") {
		t.Fatalf("save with id %q: %v; want invalid Job id", "bad id", err)
	}
}

func TestSaveRepeatedIdenticalRenderReturnsTheSamePath(t *testing.T) {
	task := t.TempDir()
	j, r := saveFixture("same brief")
	first, err := save(task, j, r)
	if err != nil {
		t.Fatal(err)
	}
	second, err := save(task, j, r)
	if err != nil || second != first {
		t.Fatalf("second save = %q, %v; want %q", second, err, first)
	}
}

func TestSaveRefusesAChangedRenderAndKeepsTheFirstFiles(t *testing.T) {
	task := t.TempDir()
	j, r := saveFixture("first brief")
	path, err := save(task, j, r)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(path)
	before := map[string]string{}
	for _, name := range []string{"brief.md", "inputs.json", "job.yaml"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		before[name] = string(b)
	}
	_, r2 := saveFixture("changed brief")
	j.Title = "changed title"
	if _, err := save(task, j, r2); err == nil || !strings.Contains(err.Error(), "create a new Job for revised work") {
		t.Fatalf("changed save: %v; want a refusal to create a new Job", err)
	}
	for name, want := range before {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(b) != want {
			t.Errorf("%s changed after the refused save: %q, %v", name, b, err)
		}
	}
}

func TestSaveLeavesNoTempDirectoryAfterSuccess(t *testing.T) {
	task := t.TempDir()
	j, r := saveFixture("brief")
	if _, err := save(task, j, r); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(task, "jobs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".render-") {
			t.Errorf("temp directory %s left in jobs/", e.Name())
		}
	}
	if len(entries) != 1 || entries[0].Name() != "J-1" {
		t.Errorf("jobs/ holds %d entries; want only J-1", len(entries))
	}
}
