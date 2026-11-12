package orchestrator

import (
	"fmt"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
)

// yaUrExit posts a usage_limit worker-exit for J-1 as the fixture's worker. A nil
// code posts none, as the fixture's own exit does.
func yaUrExit(t *testing.T, f *fixture, code any) {
	t.Helper()
	j := f.state(t)
	body := map[string]any{"reason": "usage_limit"}
	if code != nil {
		body["code"] = code
	}
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: body}); err != nil {
		t.Fatal(err)
	}
}

// yaUrParked runs J-1 to a usage_limit exit carrying code and ticks once, so the
// Job is Parked with reason usage_limit.
func yaUrParked(t *testing.T, code any) *fixture {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	yaUrExit(t, f, code)
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "usage_limit" {
		t.Fatalf("not parked on usage_limit: %+v", j)
	}
	return f
}

// yaUrRestored says whether the Feed holds the control that makes J-1 Ready with
// the reason tick.go gives a restored capacity.
func yaUrRestored(t *testing.T, f *fixture) bool {
	t.Helper()
	res, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Records {
		if r.Type == "control" && fmt.Sprint(r.Body["status"]) == string(contract.Ready) && r.Body["reason"] == "usage capacity restored" {
			return true
		}
	}
	return false
}

func TestYaUrReoffer(t *testing.T) {
	room := func(r, k bool) func() (bool, bool) { return func() (bool, bool) { return r, k } }
	for _, tc := range []struct {
		name string
		// code is the exit code as a function of the reset time T; later moves the
		// clock to T plus an hour before the next tick.
		code        func(T int64) any
		later       bool
		holder      bool
		noUsageFunc bool
		account     func() (bool, bool)
		ready       bool
	}{
		{name: "reset in the future, no account numbers", code: func(T int64) any { return T }, holder: true},
		{name: "clock past the reset", code: func(T int64) any { return T }, later: true, holder: true, ready: true},
		{name: "reset in the future, account has room", code: func(T int64) any { return T }, holder: true, account: room(true, true), ready: true},
		{name: "reset in the future, account known and full", code: func(T int64) any { return T }, holder: true, account: room(false, true)},
		{name: "reset as a numeric string, in the future", code: func(T int64) any { return fmt.Sprint(T) }, holder: true},
		{name: "reset as a numeric string, past", code: func(T int64) any { return fmt.Sprint(T) }, later: true, holder: true, ready: true},
		{name: "no code, no account numbers, as on main", code: func(int64) any { return nil }, holder: true, ready: true},
		{name: "no code, account has room", code: func(int64) any { return nil }, holder: true, account: room(true, true), ready: true},
		{name: "no code, account known and full", code: func(int64) any { return nil }, holder: true, account: room(false, true)},
		{name: "no code, account numbers not known", code: func(int64) any { return nil }, holder: true, account: room(false, false), ready: true},
		{name: "code prepare_failed reads as no code", code: func(int64) any { return "prepare_failed" }, holder: true, ready: true},
		{name: "code prepare_failed, account known and full", code: func(int64) any { return "prepare_failed" }, holder: true, account: room(false, true)},
		{name: "not the holder, clock past the reset", code: func(T int64) any { return T }, later: true, holder: false},
		{name: "not the holder, no code", code: func(int64) any { return nil }, holder: false},
		{name: "not the holder, account has room", code: func(T int64) any { return T }, holder: false, account: room(true, true)},
		{name: "UsageReady unset, clock past the reset", code: func(T int64) any { return T }, later: true, noUsageFunc: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now().UTC()
			T := start.Add(time.Hour).Unix()
			f := yaUrParked(t, tc.code(T))
			f.now = start
			if tc.later {
				f.now = start.Add(2 * time.Hour)
			}
			if !tc.noUsageFunc {
				holder := tc.holder
				f.s.UsageReady = func(string) bool { return holder }
			}
			f.s.AccountRoom = tc.account
			assigned := f.tick(t).Assigned
			j := f.state(t)
			if tc.ready {
				if assigned != 1 || !yaUrRestored(t, f) || j.Status == contract.Parked {
					t.Fatalf("want Ready again with reason %q; assigned %d, restored %v, job %+v", "usage capacity restored", assigned, yaUrRestored(t, f), j)
				}
				return
			}
			if assigned != 0 || yaUrRestored(t, f) || j.Status != contract.Parked {
				t.Fatalf("want Parked; assigned %d, restored %v, job %+v", assigned, yaUrRestored(t, f), j)
			}
		})
	}
}

// A reset time keeps a parked Job and its host's other Ready Jobs held until the
// clock passes it, then the Job is granted again with a higher fence.
func TestYaUrWalk(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	fence := f.worker.Fence
	start := time.Now().UTC()
	f.now = start
	T := start.Add(time.Hour).Unix()
	yaUrExit(t, f, T)
	f.tick(t)
	if j := f.state(t); j.Status != contract.Parked || j.Reason != "usage_limit" {
		t.Fatalf("not parked: %+v", j)
	}
	f.s.UsageReady = func(string) bool { return true }
	f.job(t, "J-2", nil)

	held := func() bool {
		snap, err := Load(f.s.Feed)
		if err != nil {
			t.Fatal(err)
		}
		snap.Assignee = f.s.Assignee
		ready, err := Ready(snap, f.now)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range ready.Waiting {
			if w.Job == "J-2" && w.Reason == "usage_limit" {
				return true
			}
		}
		return false
	}
	if !held() {
		t.Fatal("J-2 is not held while J-1 is parked on usage_limit")
	}
	if out := f.tick(t); out.Assigned != 0 || f.state(t).Status != contract.Parked {
		t.Fatalf("before the reset: assigned %d, job %+v", out.Assigned, f.state(t))
	}
	f.now = start.Add(30 * time.Minute)
	if out := f.tick(t); out.Assigned != 0 || f.state(t).Status != contract.Parked {
		t.Fatalf("still before the reset: assigned %d, job %+v", out.Assigned, f.state(t))
	}
	if !held() {
		t.Fatal("J-2 was let go before the reset")
	}
	f.now = start.Add(2 * time.Hour)
	if out := f.tick(t); out.Assigned != 2 {
		t.Fatalf("after the reset: assigned %d", out.Assigned)
	}
	j := f.state(t)
	if j.Status == contract.Parked || j.Attempt != 1 || j.Assign == nil || j.Assign.Fence <= fence {
		t.Fatalf("J-1 not granted again with a higher fence than %d: %+v", fence, j)
	}
	if held() {
		t.Fatal("J-2 is still held after the reset")
	}
}
