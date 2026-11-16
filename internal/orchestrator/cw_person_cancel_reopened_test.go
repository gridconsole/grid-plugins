package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// Lane C-cw-person-cancel-request-edges. A person's cancel request made while J-1 was open is for
// the Job as it was: when the holder closes J-1 and then reopens it (a control with status posted,
// the body of internal/cli/ship_land.go reopenShipJob), the holder's pass does not carry the old
// request out (person.go personCancels, `r.At <= reopened`), and a new request is not taken for a
// repeat of the old one (person_write.go openCancelRequest). TestR20CrN8ClosedFirst covers a target
// that is still closed.

// cwPcClock makes the Feed stamp each record one millisecond after the last, from now on, so a
// request made after the reopen is later than it by the clock and not by luck of the real one.
func cwPcClock(f *fixture) {
	at := time.Now().UTC()
	f.s.Feed.Now = func() time.Time {
		at = at.Add(time.Millisecond)
		return at
	}
}

// cwPcReopened is the fixture after a request by dana at t1, the holder's close of J-1 (status
// accepted) and its reopen (status posted, a leased control as reopenShipJob writes it). It returns
// the request and the reopen control.
func cwPcReopened(t *testing.T) (*fixture, feed.Record, feed.Record) {
	t.Helper()
	f := r20CrRunning(t)
	cwPcClock(f)
	req := r20CrRequest(t, f, "owner", "dana")
	if _, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": "accepted", "reason": "done"}}); err != nil {
		t.Fatal(err)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Accepted {
		t.Fatalf("J-1 is %s after the close, want accepted", got)
	}
	reopen, err := Write(f.s.Feed, f.s.Writer, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "status": string(contract.Posted), "reason": "land failed: no remote"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := jobStatus(t, f, "J-1"); got != contract.Posted {
		t.Fatalf("J-1 is %s after the reopen, want posted (contract/fold.go:251-256 takes a leased control's status)", got)
	}
	if req.At >= reopen.At {
		t.Fatalf("the request at %s is not before the reopen at %s", req.At, reopen.At)
	}
	return f, req, reopen
}

// Row 2: the request was made before the reopen, so the holder's pass writes one not-used
// message re it, with the reopen's time, and no control; J-1 is not Cancelled. A second pass
// writes nothing more re the request.
func TestCwPcRequestBeforeReopenIsNotUsed(t *testing.T) {
	f, req, reopen := cwPcReopened(t)
	f.tick(t)
	got := r20CrRe(t, f, req.ID)
	if len(got) != 1 {
		t.Fatalf("want one record re the request, got %d: %+v", len(got), got)
	}
	m := got[0]
	text := stringValue(m.Body["text"])
	if m.Type != "post" || m.Lease == nil || *m.Lease != "task" || m.Fence != f.s.Writer.Fence ||
		!strings.Contains(text, "closed and was reopened at "+reopen.At) || !strings.Contains(text, req.ID) || !strings.Contains(text, "not used") {
		t.Fatalf("want a not-used post under the task lease naming the reopen at %s, got %+v", reopen.At, m)
	}
	if m.ID != personCancelID(req.ID, "unused") {
		t.Fatalf("the message id %s is not the one derived from the request", m.ID)
	}
	for _, r := range got {
		if r.Type == "control" {
			t.Fatalf("a control was written re the request: %+v", r)
		}
	}
	// The same pass puts the reopened Job back on the queue (it reads assigned), so the check is
	// that the request did not cancel it.
	if s := jobStatus(t, f, "J-1"); s == contract.Cancelled || s == contract.Rejected || s == contract.Accepted {
		t.Fatalf("J-1 is %s; the old request moved a reopened Job", s)
	}
	f.tick(t)
	if again := r20CrRe(t, f, req.ID); len(again) != 1 {
		t.Fatalf("a second pass wrote again re the request: %+v", again)
	}
}

// Row 3: after the reopen the same person's new request is saved, not skipped as a repeat of the
// old open one, and the next pass cancels J-1 with one leased control re the new request; the old
// request still has only its not-used message.
func TestCwPcNewRequestAfterReopenCancels(t *testing.T) {
	// No pass runs between the reopen and the new request, so the old request is still unhandled:
	// only the reopen cut-off (person_write.go:63) keeps it from being taken for a repeat.
	f, old, reopen := cwPcReopened(t)
	before := recordCount(t, f)
	w := personWriterOf("owner")
	w.Author.ID, w.Worker = "dana", "dana@"+machine
	rec, skipped, err := PostPersonUnless(f.s.Feed, w, r20CrInput("J-1"), nil)
	if err != nil || skipped {
		t.Fatalf("the new request after the reopen: skipped %v, err %v", skipped, err)
	}
	if rec.ID == old.ID || rec.At <= reopen.At || rec.Lease != nil || rec.Fence != 0 {
		t.Fatalf("want a new unleased request later than the reopen at %s, got %+v", reopen.At, rec)
	}
	if got := recordCount(t, f); got != before+1 {
		t.Fatalf("%d records before and %d after the new request, want one more", before, got)
	}
	if s := jobStatus(t, f, "J-1"); s != contract.Posted {
		t.Fatalf("J-1 is %s after the request alone, want posted", s)
	}
	f.tick(t)
	done := r20CrRe(t, f, rec.ID)
	if len(done) != 1 {
		t.Fatalf("want one record re the new request, got %d: %+v", len(done), done)
	}
	c := done[0]
	if c.Type != "control" || c.Lease == nil || *c.Lease != "task" || c.Fence != f.s.Writer.Fence || c.Body["target"] != "J-1" || c.Body["status"] != "cancelled" {
		t.Fatalf("want the holder's leased cancel re the new request, got %+v", c)
	}
	if s := jobStatus(t, f, "J-1"); s != contract.Cancelled {
		t.Fatalf("J-1 is %s after the holder's pass, want cancelled", s)
	}
	if oldRe := r20CrRe(t, f, old.ID); len(oldRe) != 1 || oldRe[0].Type != "post" {
		t.Fatalf("the old request should have only its not-used message: %+v", oldRe)
	}
}
