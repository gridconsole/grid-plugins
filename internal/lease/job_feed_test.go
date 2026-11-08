//go:build linux || darwin

package lease_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// The task is built by the real Feed and session stores, so the reader is held
// to the folder layout they write instead of a layout the test invents.
func TestReadJobFindsLeaseAndBeatWrittenByTheRealStores(t *testing.T) {
	// 0, 10 and 120 ms are the shapes Go's shortest-fraction format used to break.
	for _, ms := range []int{123, 0, 10, 120} {
		readJobFindsBeatAt(t, ms)
	}
}

func readJobFindsBeatAt(t *testing.T, ms int) {
	t.Helper()
	const holder = "worker@01K9J000000000000000000002"
	task := t.TempDir()
	store := feed.Store{TaskDir: task, TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}
	// The real store creates feed/; the grant sits where its lease reader looks.
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "agent"}, Worker: holder}
	if _, err := store.Post(feed.Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": "started"}}, w); err != nil {
		t.Fatal(err)
	}
	grant, err := os.ReadFile("../../contract/fixtures/lease/valid/alternate.json")
	if err != nil {
		t.Fatal(err)
	}
	leaseDir := filepath.Join(store.Dir(), "lease")
	if err := os.MkdirAll(leaseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leaseDir, "J-1.json"), grant, 0o600); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 11, 8, 10, 0, 0, ms*int(time.Millisecond), time.UTC)
	sessions := session.Store{TaskDir: task, Now: func() time.Time { return now }}
	key := session.Key{SessionID: "sess-1", MachineID: "01K9J000000000000000000002"}
	rec, err := sessions.Start(session.Record{Key: key, PID: os.Getpid(), PIDStart: "2026-11-08T10:00:00.000Z", Lease: "J-1", Holder: holder, Fence: 2, Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Reconcile(key, session.Observation{Process: session.Alive, PID: rec.PID, PIDStart: rec.PIDStart, OutputAt: now}); err != nil {
		t.Fatal(err)
	}
	beat, err := sessions.ReadHeartbeat(rec)
	if err != nil {
		t.Fatal(err)
	}

	got, err := lease.ReadJob(task, "J-1")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Known || got.Fence != 2 || got.Holder == nil || *got.Holder != holder {
		t.Fatalf("grant not found: %+v", got)
	}
	if !got.Until.Equal(beat.Until) {
		t.Fatalf("heartbeat not found: until %v, session wrote %v", got.Until, beat.Until)
	}
	if got.Expired(2, now) || !got.Expired(2, beat.Until) || !got.Expired(1, now) {
		t.Fatalf("expiry wrong: %+v", got)
	}
}
