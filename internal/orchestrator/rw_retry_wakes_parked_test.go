package orchestrator

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// rwParkAndRetry crash-loops J-1 into the park question, posts the control with no status the
// app's judge posts on a parked Job's wake (it replaces the Job's reason), then answers Retry.
func rwParkAndRetry(t *testing.T, f *fixture) {
	t.Helper()
	f.job(t, "J-1", nil)
	f.tick(t)
	for range 3 {
		f.ack(t)
		f.exit(t, "crash")
		f.tick(t)
	}
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "needs_person" {
		t.Fatalf("three crashes did not park J-1 for the person: %+v", j)
	}
	f.post(t, feed.Input{Type: "control", Job: "J-1", Body: map[string]any{"target": "J-1", "reason": "the Job is parked, nothing to judge (wake R/3)"}}, f.s.Writer)
	if j := f.state(t); j.Status != contract.Parked || j.Reason == "needs_person" {
		t.Fatalf("the control with no status left the reason as it was: %+v", j)
	}
	state, err := f.s.Feed.State()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Questions) != 1 {
		t.Fatalf("questions %d, want the one park question", len(state.Questions))
	}
	for id := range state.Questions {
		f.post(t, feed.Input{Type: "answer", Re: id, Body: map[string]any{"answer": "Retry"}}, feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine})
	}
}

// T-11's J-16: the owner answered Retry on the park question after the judge had replaced the
// parked Job's reason, and the Job stayed parked with nothing running.
func TestRwRetryWakesParkedJobWhoseReasonWasReplaced(t *testing.T) {
	f := setup(t)
	rwParkAndRetry(t, f)
	if got := f.tick(t).Assigned; got != 1 {
		t.Fatalf("the owner's Retry did not wake the parked Job: %d assigned, job %+v", got, f.state(t))
	}
}

// The Retry the Job already took does not wake a later park of the same Job: the failure the
// question waits on stays in memory, so only the ready control that answered it tells them apart.
func TestRwRetryWakesOnlyOnce(t *testing.T) {
	f := setup(t)
	rwParkAndRetry(t, f)
	if got := f.tick(t).Assigned; got != 1 {
		t.Fatalf("the owner's Retry did not wake the parked Job: %d assigned", got)
	}
	f.ack(t)
	f.exit(t, "usage_limit")
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "usage_limit" {
		t.Fatalf("the usage_limit exit did not park J-1: %+v", j)
	}
	if got := f.tick(t).Assigned; got != 0 {
		t.Fatalf("the Retry J-1 already took woke its usage_limit park: %d assigned, job %+v", got, f.state(t))
	}
}
