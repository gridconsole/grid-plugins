package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// Fifth review of the stuck-tasks lane, must-fix 3. A Job Ready releases can still
// be held by the tick (orchestrator tick.go): the host of the task's holder is
// offline, the Job waits out its back-off after a failed attempt, or a Job parked on
// usage_limit holds the holder's host. job blocked lists such a Job with the reason
// the tick holds it for now, and not with the reason an earlier tick stored.

// stBlocked runs job blocked --json and returns each listed Job's reason.
func stBlocked(t *testing.T, h *harness, path string) map[string]string {
	t.Helper()
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(path, "job", "blocked", "--json"); code != 0 {
		t.Fatalf("job blocked: %d %s", code, h.errb)
	}
	var rows []orchestrator.Row
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("%v: %s", err, h.out)
	}
	why := map[string]string{}
	for _, r := range rows {
		why[r.ID] = r.Why
	}
	return why
}

// stControl posts the status-less control the tick writes for a held Job.
func stControl(t *testing.T, h *harness, store feed.Store, body map[string]any) {
	t.Helper()
	w, err := orchestratorWriter(h.env, "task", os.Getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{Type: "control", Body: body}, w); err != nil {
		t.Fatal(err)
	}
}

func TestJobBlockedListsTheTickHolds(t *testing.T) {
	next := time.Now().Add(time.Hour).UTC()
	past := time.Now().Add(-time.Hour).UTC()
	for _, tc := range []struct{ name, memory, stored, want string }{
		{"holder's host offline", `{"offline_hosts": {"` + feedMachine + `": true}, "jobs": {}, "delivered": {}}`, "host_offline", "host_offline"},
		{"another host offline", `{"offline_hosts": {"other": true}, "jobs": {}, "delivered": {}}`, "host_offline", ""},
		{"back-off running", `{"jobs": {"J-1": {"retry_at": "` + next.Format(time.RFC3339Nano) + `"}}, "delivered": {}}`, "",
			"Waiting to retry after a failed attempt, next try " + next.Local().Format("15:04:05")},
		{"back-off over", `{"jobs": {"J-1": {"retry_at": "` + past.Format(time.RFC3339Nano) + `"}}, "delivered": {}}`,
			"Waiting to retry after a failed attempt, next try " + past.Local().Format("15:04:05"), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, path, store := orchestratorHarness(t)
			t.Setenv("GRID_ROLE", "member")
			if err := os.WriteFile(filepath.Join(store.Dir(), ".orchestrator.json"), []byte(tc.memory), 0o600); err != nil {
				t.Fatal(err)
			}
			if tc.stored != "" {
				stControl(t, h, store, map[string]any{"target": "J-1", "reason": tc.stored})
			}
			got, listed := stBlocked(t, h, path)["J-1"]
			if tc.want == "" && listed {
				t.Errorf("J-1 is not held and listed as blocked: %q", got)
			}
			if tc.want != "" && got != tc.want {
				t.Errorf("J-1 is listed with %q, want %q", got, tc.want)
			}
		})
	}
}

func TestJobBlockedListsTheUsageLimitHold(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("job next: %d %s", code, h.errb)
	}
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--type", "job", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("post J-2: %d %s", code, h.errb)
	}
	stControl(t, h, store, map[string]any{"target": "J-1", "status": "parked", "reason": "usage_limit"})
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := st.Jobs["J-1"]; j == nil || j.Status != "parked" || j.Assign == nil || j.Assign.To != "member@"+feedMachine {
		t.Fatalf("J-1 in state.json: %+v, want parked on usage_limit on the holder's host", j)
	}
	why := stBlocked(t, h, path)
	if why["J-2"] != "usage_limit" || why["J-1"] != "usage_limit" {
		t.Errorf("job blocked lists %v, want J-1 and J-2 held on usage_limit", why)
	}
}

func TestJobBlockedSaysNothingBlocked(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(path, "job", "blocked"); code != 0 || h.out.String() != "Nothing blocked.\n" {
		t.Errorf("job blocked: %d %q %s, want Nothing blocked.", code, h.out, h.errb)
	}
}
