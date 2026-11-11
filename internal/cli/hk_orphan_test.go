package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// hkOrphHarness is orchestratorHarness with created_at set on the J-1 body
// before it is posted (left out when createdAt is ""), so a test fixes the
// instant orphanOf compares with the clock instead of waiting for it.
func hkOrphHarness(t *testing.T, createdAt string) (*harness, string, feed.Store) {
	t.Helper()
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	w := feed.Writer{Author: feed.Author{ID: "member", Role: "member"}, Worker: "member@" + feedMachine, Lease: "task"}
	grant, err := (lease.TaskStore{Feed: store}).TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", w.Fence))
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if createdAt != "" {
		body["created_at"] = createdAt
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	return h, path, store
}

// hkOrphTreeRows reads job tree --json on the first read after the post.
func hkOrphTreeRows(t *testing.T, createdAt string) []map[string]any {
	t.Helper()
	h, path, _ := hkOrphHarness(t, createdAt)
	h.out.Reset()
	if code := h.run(path, "job", "tree", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("%v %s", err, h.out)
	}
	return rows
}

func TestHkOrphOldCreated(t *testing.T) {
	rows := hkOrphTreeRows(t, "2020-01-01T00:00:00.000Z")
	if len(rows) != 1 || rows[0]["id"] != "J-1" {
		t.Fatalf("rows: %v", rows)
	}
	orphan, _ := rows[0]["orphan"].(map[string]any)
	if orphan["kind"] != "never_picked_up" || orphan["since"] != "2020-01-01T00:00:00.000Z" {
		t.Fatalf("J-1 row: %v", rows[0])
	}
}

func TestHkOrphFutureCreated(t *testing.T) {
	rows := hkOrphTreeRows(t, "2999-01-01T00:00:00.000Z")
	if len(rows) != 1 || rows[0]["id"] != "J-1" {
		t.Fatalf("rows: %v", rows)
	}
	if orphan, has := rows[0]["orphan"]; has {
		t.Fatalf("J-1 carries orphan: %v", orphan)
	}
	h, path, _ := hkOrphHarness(t, "2999-01-01T00:00:00.000Z")
	h.out.Reset()
	if code := h.run(path, "job", "tree"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if strings.Contains(h.out.String(), "orphan") {
		t.Fatalf("text tree shows an orphan:\n%s", h.out)
	}
}
