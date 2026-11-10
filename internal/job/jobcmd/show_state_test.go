package jobcmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

const showMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

// postJob posts J-1 to the task's Feed under a task lease and returns the store and writer.
func postJob(t *testing.T, task string) (feed.Store, feed.Writer) {
	t.Helper()
	store := feed.Store{TaskDir: task}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + showMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", w.Fence))
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contract", "fixtures", "job-record", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	return store, w
}

func TestShowReadsTheStateFromTheFeed(t *testing.T) {
	cases := []struct {
		name    string
		control map[string]any
		state   string
		reason  string
		line    string
	}{
		{"running", map[string]any{"target": "J-1", "status": "running", "reason": "worker started"}, "Started", "", "state: Started\n"},
		{"accepted", map[string]any{"target": "J-1", "status": "accepted", "reason": "reviewed"}, "Done", "", "state: Done\n"},
		{"cancelled", map[string]any{"target": "J-1", "status": "cancelled", "reason": "owner call", "close_reason": "scope withdrawn"}, "Done", "cancelled", "state: Done (cancelled)\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, out, errs := fixture(t)
			call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
			// Posted but not moved: the Feed says Not started.
			store, w := postJob(t, filepath.Join(e.Cwd, "T-01"))
			if r := call(t, e, out, errs, 0, "show", "T-01", "J-1", "--json"); r.State != "Not started" {
				t.Fatalf("posted Job reads %q", r.State)
			}
			if _, err := store.Post(feed.Input{Type: "control", Body: c.control}, w); err != nil {
				t.Fatal(err)
			}
			r := call(t, e, out, errs, 0, "show", "T-01", "J-1", "--json")
			if r.State != c.state || r.Reason != c.reason {
				t.Fatalf("json state %q (%q) want %q (%q)", r.State, r.Reason, c.state, c.reason)
			}
			call(t, e, out, errs, 0, "show", "T-01", "J-1")
			if !strings.HasSuffix(out.String(), c.line) {
				t.Fatalf("text output lacks %q: %s", c.line, out)
			}
		})
	}
}

func TestShowUnpostedJobFallsBackToTheBrief(t *testing.T) {
	e, out, errs := fixture(t)
	call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if r := call(t, e, out, errs, 0, "show", "T-01", "J-1", "--json"); r.State != "Not started" {
		t.Fatalf("unposted Job reads %q", r.State)
	}
}
