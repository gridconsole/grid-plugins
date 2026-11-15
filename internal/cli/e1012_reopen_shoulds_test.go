package cli

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// e1012 (end review 1012Z S1 and S5): the reopen of a closed task posts the Continue
// Job only when a stop closed it, and a refused reopen leaves no Handover record.

// e1012Handovers counts the handover records on the Feed.
func e1012Handovers(t *testing.T, store feed.Store) int {
	t.Helper()
	r, err := store.ReadOnly(feed.Filter{Type: "handover"})
	if err != nil {
		t.Fatal(err)
	}
	return len(r.Records)
}

// S1: a task that was delivered and closed (grid task close, as the land approval
// writes it) and once had a cancelled Job (a deliver rework the Judge cancelled)
// reopens to queued and posts no Continue Job: it was not stopped.
// The stopped task keeps its Job: TestR13ReopenAfterTaskStopPostsTheOrchestratorJob.
func TestE1012ReopenOfADeliveredTaskPostsNoContinueJob(t *testing.T) {
	h, path, store := r13Stopped(t, "running", "superseded")
	if c := h.run(path, "task", "close", "T-07"); c != 0 {
		t.Fatalf("task close: %d %s", c, h.errb)
	}
	before := joRpIDs(t, store)
	if c := h.run(path, "task", "reopen", "T-07"); c != 0 {
		t.Fatalf("task reopen: %d %s", c, h.errb)
	}
	if got := r13State(t, path); got != "queued" {
		t.Fatalf("state %q after the reopen, want queued", got)
	}
	after := joRpIDs(t, store)
	if len(after) != len(before) || after["J-2"] != 0 {
		t.Fatalf("the reopen of a delivered task posted a Job: before %v after %v", before, after)
	}
	if st, err := store.State(); err != nil || st.Jobs["J-2"] != nil {
		t.Fatalf("a Job J-2 is on the Feed: %v", err)
	}
}

// S5: a reopen the task write refuses (a --reason with a control character, which
// the move checks after the writer and the task lease are taken) leaves the Feed as
// it was: no Handover record, the task still closed.
func TestE1012RefusedReopenLeavesNoHandoverRecord(t *testing.T) {
	h, path, store := r13Stopped(t, "running", "stop_requested")
	if c := h.run(path, "task", "stop", "T-07"); c != 0 {
		t.Fatalf("task stop: %d %s", c, h.errb)
	}
	handovers, jobs := e1012Handovers(t, store), joRpIDs(t, store)
	h.errb.Reset()
	if c := h.run(path, "task", "reopen", "--reason", "bad\x01reason", "T-07"); c == 0 {
		t.Fatalf("task reopen with a control character in the reason succeeded: %s", h.out)
	}
	if got := r13State(t, path); got != "closed" {
		t.Fatalf("state %q after the refusal, want closed", got)
	}
	if got := e1012Handovers(t, store); got != handovers {
		t.Fatalf("the refused reopen left %d handover records on the Feed, want %d", got, handovers)
	}
	if after := joRpIDs(t, store); len(after) != len(jobs) {
		t.Fatalf("the refused reopen posted a Job: before %v after %v", jobs, after)
	}
}
