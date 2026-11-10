package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// Jobs posted with two kinds read back with those kinds from grid feed state
// --json. A Job posted without a kind reads without one, as state did before.
func TestFeedStateCarriesJobKind(t *testing.T) {
	h, task := feedHarness(t)
	run := func(args ...string) []byte {
		t.Helper()
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(task, args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
		return append([]byte(nil), h.out.Bytes()...)
	}
	store := feed.Store{TaskDir: task, TaskID: feedTaskID}
	writer := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(writer, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", grant.Fence))
	run("feed", "handover", "--reason", "Example handover", "--lease", "task", "--json")
	raw, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	jobs := []struct{ id, kind string }{{"J-1", "code"}, {"J-2", "user-docs"}, {"J-3", ""}}
	for _, j := range jobs {
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		body["id"] = j.id
		if j.kind != "" {
			body["kind"] = j.kind
		}
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		run("job", "post", j.id, "--type", "job", "--body", string(data), "--json")
	}
	out := run("feed", "state", "--json")
	if err := contract.Validate("cmd-feed-state", out); err != nil {
		t.Fatalf("cmd-feed-state: %v\n%s", err, out)
	}
	var state struct {
		Jobs map[string]map[string]any `json:"jobs"`
	}
	if err := json.Unmarshal(out, &state); err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		got, has := state.Jobs[j.id]["kind"]
		if j.kind == "" {
			if has {
				t.Errorf("%s kind = %v, want none", j.id, got)
			}
			continue
		}
		if got != j.kind {
			t.Errorf("%s kind = %v, want %q", j.id, got, j.kind)
		}
	}
}
