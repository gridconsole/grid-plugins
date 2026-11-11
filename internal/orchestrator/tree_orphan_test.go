package orchestrator

import (
	"encoding/json"
	"testing"
	"time"
)

// Row 1: J-1 is assigned with an `until` in the past, acked, and has no
// worker-exit. Tree names it an orphan; on main it reads `running` with no hint.
func TestTreeNamesAckedJobWithNoExitPastItsLease(t *testing.T) {
	f := setup(t)
	f.now = time.Now().UTC().Add(-48 * time.Hour)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	for _, r := range Tree(snap) {
		if r.ID != "J-1" {
			continue
		}
		if len(r.Attempts) != 1 || r.Attempts[0].State != "running" {
			t.Fatalf("attempts %+v", r.Attempts)
		}
		b, _ := json.Marshal(r)
		_ = json.Unmarshal(b, &row)
	}
	orphan, _ := row["orphan"].(map[string]any)
	if orphan == nil || orphan["kind"] != "no_exit" {
		t.Fatalf("J-1 acked, lease long past, no worker-exit: want orphan no_exit, got row %v", row)
	}
}
