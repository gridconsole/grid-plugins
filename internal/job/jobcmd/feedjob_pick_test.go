package jobcmd

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func entry(id, jobID string, item int, title string) feed.Entry {
	e := feed.Entry{Verdict: "counted", Record: feed.Record{ID: id, Type: "job"}}
	e.Body = map[string]any{"id": jobID, "item": float64(item), "title": title}
	return e
}

func TestPickJobMarksTheOlderPostStale(t *testing.T) {
	records := []feed.Entry{entry("R1", "J-1", 1, "old"), entry("R2", "J-2", 2, "other"), entry("R3", "J-1", 1, "new")}
	j, err := pickJob(records, "J-1")
	if err != nil || j == nil || j.Title != "new" {
		t.Fatalf("pickJob = %+v, %v; want the newest post", j, err)
	}
	if records[0].Verdict != "stale" || !strings.Contains(records[0].Reason, "R3") {
		t.Errorf("older post: verdict %q reason %q; want stale, superseded by R3", records[0].Verdict, records[0].Reason)
	}
	if records[1].Verdict != "counted" || records[2].Verdict != "counted" {
		t.Errorf("only the older post of the same Job is stale: %q %q", records[1].Verdict, records[2].Verdict)
	}
	// A stale post is never chosen again, by id or by item.
	if j, _ := pickJob(records, "1"); j == nil || j.Title != "new" {
		t.Errorf("by item: %+v", j)
	}
}

func TestPickJobNamesRecordAndFieldOfTheSelectedJobOnly(t *testing.T) {
	bad := entry("R2", "J-2", 2, "broken")
	bad.Body["max_rounds"] = "many"
	records := []feed.Entry{entry("R1", "J-1", 1, "fine"), bad}
	if j, err := pickJob(records, "J-1"); err != nil || j == nil {
		t.Fatalf("selecting J-1 with a malformed J-2 posted: %+v, %v", j, err)
	}
	_, err := pickJob(records, "J-2")
	if err == nil {
		t.Fatal("selecting the malformed Job should fail")
	}
	for _, want := range []string{"J-2", "R2", "max_rounds"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
	// A nested field is named by its path; a record with no readable id still
	// belongs to the Job its record names.
	nested := entry("R3", "J-3", 3, "nested")
	nested.Body["intent"] = map[string]any{"item": 7}
	if _, err := pickJob([]feed.Entry{nested}, "J-3"); err == nil || !strings.Contains(err.Error(), "intent.item") {
		t.Errorf("nested field: %v", err)
	}
	noID := entry("R4", "", 4, "id is wrong")
	noID.Body["id"] = 12
	job := "J-4"
	noID.Job = &job
	if _, err := pickJob([]feed.Entry{noID}, "J-4"); err == nil || !strings.Contains(err.Error(), "id") {
		t.Errorf("record with a malformed id field: %v", err)
	}
}

// A wrapped not-exist error from the Feed read is a Feed that is not there, the
// same as a bare one: os.IsNotExist does not see through the wrapping.
func TestShowTreatsAWrappedNotExistFeedErrorAsNoFeed(t *testing.T) {
	e, out, errs := fixture(t)
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	was := readFeed
	t.Cleanup(func() { readFeed = was })
	readFeed = func(string, feed.Filter) (feed.ReadResult, error) {
		return feed.ReadResult{}, fmt.Errorf("reading the Feed: %w", &fs.PathError{Op: "open", Path: "feed", Err: os.ErrNotExist})
	}
	r := call(t, e, out, errs, 0, "show", "T-01", "J-1", "--json")
	if !r.OK || r.ID != "J-1" {
		t.Fatalf("show with a wrapped not-exist Feed error: %+v; stderr %s", r, errs)
	}
}
