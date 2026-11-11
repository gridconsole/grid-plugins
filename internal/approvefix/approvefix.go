// Package approvefix builds, for tests, the Feed records that stand for a
// person's approval of a plan: the approve person Job of a task and the accepting
// result that carries the plan_hash and plan_digest the person saw. Since the Job
// result is the only approval record, a test that needs an approved (or
// not yet approved) task calls this instead of writing a file. It is imported by
// tests only.
package approvefix

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

const machine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

var (
	mu      sync.Mutex
	writers = map[string]feed.Writer{}
)

// Lease gives the task folder a Feed identity when it has none, takes the task
// lease for a member of the team and posts the tenure's handover, as a worker's
// first pass does. It returns the lease writer and the folder's Feed store. A
// second call for the same folder returns the same writer.
func Lease(t testing.TB, dir string) (feed.Writer, feed.Store) {
	t.Helper()
	id, err := feed.EnsureTaskID(dir)
	if err != nil {
		t.Fatal(err)
	}
	store := feed.Store{TaskDir: dir, TaskID: id}
	mu.Lock()
	defer mu.Unlock()
	if w, ok := writers[dir+"\x00"+id]; ok {
		return w, store
	}
	// The lease file is written here, not taken through internal/lease: lease
	// imports job, which imports task, and tests of those packages use this helper.
	worker := "dana@" + machine
	w := feed.Writer{Author: feed.Author{ID: "dana", Role: "member"}, Worker: worker, Lease: "task", Gen: 0, Fence: 1}
	data, err := json.Marshal(feed.Lease{Lease: "task", Holder: &worker, Gen: w.Gen, Fence: w.Fence})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(store.Dir(), "lease"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(store.Dir(), "lease", "task.json"), append(data, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	writers[dir+"\x00"+id] = w
	return w, store
}

// OpenJob posts the approve person Job of the plan with no result, the state of
// a task that waits for a person, and returns its id.
func OpenJob(t testing.TB, dir string) string {
	t.Helper()
	w, store := Lease(t, dir)
	read, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	highest := 0
	for _, r := range read.Records {
		id, _ := r.Body["id"].(string)
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "J-")); err == nil && n > highest {
			highest = n
		}
	}
	id := "J-" + strconv.Itoa(highest+1)
	jobID, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{ID: jobID, Type: "job", Job: id, Body: jobBody(id)}, w); err != nil {
		t.Fatal(err)
	}
	return id
}

// Approve posts the approve person Job of the plan and its accepting result for
// the revision planText, as grid approve leaves them, and returns the Job id.
// lands is the lands_hash of the result, or "".
func Approve(t testing.TB, dir, planText, lands string) string {
	t.Helper()
	id := OpenJob(t, dir)
	Accept(t, dir, id, planText, lands)
	return id
}

// Accept posts the accepting result of an open approve Job for the revision planText.
func Accept(t testing.TB, dir, job, planText, lands string) {
	t.Helper()
	w, store := Lease(t, dir)
	body := map[string]any{"target": job, "status": "accepted", "reason": "answered by dana", "answer": "approved", "plan_hash": plan.Hash(planText), "plan_digest": plan.Digest(planText)}
	if lands != "" {
		body["lands_hash"] = lands
	}
	if _, err := store.Post(feed.Input{Type: "control", Body: body}, w); err != nil {
		t.Fatal(err)
	}
}

// JobBody is the record body of the approve person Job with the given id, for a
// test that posts it with its own writer.
func JobBody(id string) map[string]any { return jobBody(id) }

func jobBody(id string) map[string]any {
	return map[string]any{
		"id": id, "kind": "approve", "item": 1, "title": "Approve the plan", "role": "Person",
		"skills": []string{}, "risk": []string{},
		"intent":       map[string]any{"owner": "dana", "item": "plan.md#approve", "goal": "plan.md#approve"},
		"links":        []any{},
		"keep":         []string{},
		"decisions":    []string{},
		"read":         []string{"plan.md"},
		"write":        []string{},
		"out_of_scope": []string{},
		"acceptance":   []any{map[string]any{"id": "A1", "text": "The plan revision is approved", "traces": "item"}},
		"verify":       map[string]any{"command": "grid plan hash", "min_tests": 1},
		"budget":       map[string]any{"turns_estimate": 1, "reads_before_first_edit": 1},
		"on_blocked":   "ask-and-continue", "after": []any{}, "attempt": 1,
		"continues": nil, "discovered_from": nil, "close_reason": nil, "checkpoint": nil,
		"review_rounds": 0, "level_raised": nil, "worktree": nil,
		"posts_back": []string{"result"},
	}
}
