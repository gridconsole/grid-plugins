package task

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// The Job fixture is the shared internal/approvefix: the lease the Feed checks,
// the handover, the approve person Job and its accepting control.
func taskLease(t *testing.T, dir string) (feed.Writer, feed.Store) {
	t.Helper()
	return approvefix.Lease(t, dir)
}

// openJob posts the approve person Job of the plan with no result and returns its id.
func openJob(t *testing.T, dir string) string {
	t.Helper()
	return approvefix.OpenJob(t, dir)
}

// approveFixturePlan leaves what a person's approval of plan.md leaves, so a task
// in any state can be delivered: the approve person Job and its accepting result.
// History stays as the test finds it.
func approveFixturePlan(t *testing.T, s Store, task Task) {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(task.Path, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	approvefix.Approve(t, task.Path, string(text), "")
}

// approve is what grid approve does around Store.Approve: the compare-and-set and
// the history line, then the approval as the result of the approve person Job
// (the CLI posts it once Approve has released its locks). A refusal posts nothing.
func approve(t *testing.T, s Store, task Task, o WriteOptions, token string) (Event, error) {
	t.Helper()
	event, err := s.Approve(o, token)
	if err != nil {
		return event, err
	}
	postApprovalResult(t, task.Path, event)
	return event, nil
}

// postApprovalResult answers the plan Job with e: the open Job when there is one,
// a new Job otherwise (the plan changed since the last result). A plan Job that
// already holds this approval gets nothing new, as for a repeat press.
func postApprovalResult(t *testing.T, dir string, e Event) {
	t.Helper()
	found, err := planJobResult(dir)
	if err == nil && found.PlanHash == e.PlanHash && found.PlanDigest == e.PlanDigest && found.LandsHash == e.LandsHash {
		return
	}
	var open planJobOpenError
	job := ""
	if errors.As(err, &open) {
		job = open.job
	} else {
		job = openJob(t, dir)
	}
	fields := map[string]any{"plan_hash": e.PlanHash, "plan_digest": e.PlanDigest}
	if e.LandsHash != "" {
		fields["lands_hash"] = e.LandsHash
	}
	if e.Note != "" {
		fields["note"] = e.Note
	}
	postResult(t, dir, job, fields)
}

// postResult posts an accepting control on job with the given result fields.
func postResult(t *testing.T, dir, job string, fields map[string]any) {
	t.Helper()
	w, store := taskLease(t, dir)
	body := map[string]any{"target": job, "status": "accepted", "reason": "answered by dana", "answer": "approved"}
	for k, v := range fields {
		body[k] = v
	}
	if _, err := store.Post(feed.Input{Type: "control", Body: body}, w); err != nil {
		t.Fatal(err)
	}
}

// postReopenJob posts what grid task reopen posts: a new approve person Job that
// continues the last one, so the results before it no longer approve the plan.
func postReopenJob(t *testing.T, dir string) {
	t.Helper()
	w, store := taskLease(t, dir)
	read, err := store.Read(feed.Filter{Type: "job"})
	if err != nil {
		t.Fatal(err)
	}
	highest, last := 0, ""
	var lastBody map[string]any
	for _, r := range read.Records {
		id, _ := r.Body["id"].(string)
		if n, err := strconv.Atoi(strings.TrimPrefix(id, "J-")); err == nil && n > highest {
			highest = n
		}
		if kind, _ := r.Body["kind"].(string); kind == "approve" {
			last, lastBody = id, r.Body
		}
	}
	if last == "" {
		t.Fatal("no approve Job to continue")
	}
	body := map[string]any{}
	for k, v := range lastBody {
		body[k] = v
	}
	id := "J-" + strconv.Itoa(highest+1)
	body["id"], body["continues"], body["title"] = id, last, "Approve the plan again after the reopen"
	rec, err := feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{ID: rec, Type: "job", Job: id, Body: body}, w); err != nil {
		t.Fatal(err)
	}
}

// noReceiptFile fails when the task folder holds an old approval receipt: nothing
// writes one any more.
func noReceiptFile(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if legacyReceipt.MatchString(e.Name()) {
			t.Fatalf("%s was written", e.Name())
		}
	}
}

// sameApproval compares what a Job result carries with what Approve returned: the
// plan revision, the lands hash and the note. The Feed record names who and when.
func sameApproval(a, b Event) bool {
	return a.PlanHash == b.PlanHash && a.PlanDigest == b.PlanDigest && a.LandsHash == b.LandsHash && a.Note == b.Note && a.What == b.What && a.Gate == b.Gate
}
