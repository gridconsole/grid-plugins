package routine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func TestRoutineLegacyFingerprint(t *testing.T) {
	// Pinned from plugin base 0cd9a7cf8be54dbe6197ccd18a6a5efbdd9fc219.
	const want = "a0b16821620a7c0a8f9bf3fd44d3890514df8094797106e4b3b316f6f6132ab9"
	e := Entry{Run: "grid version", Every: "1h"}
	if got := e.Fingerprint(); got != want {
		t.Fatalf("legacy fingerprint = %s, want %s", got, want)
	}
}

func writeRoutineWorkflow(t *testing.T, s Store, front string) {
	t.Helper()
	dir := s.GridHome
	if s.Project != "" {
		dir = filepath.Join(dir, "projects", s.Project)
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workflow.md"), []byte("---\n"+front+"\n---\nWorkflow notes.\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTimerRunRefusesEntryLimitChange(t *testing.T) {
	for _, tc := range []struct{ key, before, after string }{
		{"routine_budget", "1", "50"},
		{"routine_max_turns", "10", "100"},
		{"routine_timeout", "10m", "2h"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			testTimerLimitChange(t, false, tc.key, tc.before, tc.after)
		})
	}
}

func TestTimerRunRefusesWorkflowLimitChange(t *testing.T) {
	for _, key := range []string{"routine_budget", "routine_max_turns", "routine_timeout"} {
		t.Run(key, func(t *testing.T) {
			before := map[string]string{"routine_budget": "1", "routine_max_turns": "10", "routine_timeout": "10m"}[key]
			after := map[string]string{"routine_budget": "50", "routine_max_turns": "100", "routine_timeout": "2h"}[key]
			testTimerLimitChange(t, true, key, before, after)
		})
	}
}

func testTimerLimitChange(t *testing.T, workflow bool, key, before, after string) {
	t.Helper()
	for _, project := range []string{"", "example"} {
		t.Run("scope_"+project, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			now := time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)
			store := Store{GridHome: filepath.Join(home, "data"), Project: project, Now: func() time.Time { return now }}
			policy := LimitPolicy{}
			if workflow {
				writeRoutineWorkflow(t, store, key+": "+before)
			} else {
				policy = policyForTest(t, map[string]string{key: before})
			}
			if _, err := store.Add("daily", "1h", "grid version", policy); err != nil {
				t.Fatal(err)
			}
			calls := &fakeOS{}
			scheduler := Scheduler{Home: home, GridHome: store.GridHome, Binary: filepath.Join(home, "grid"), Backend: Systemd{}, Runner: calls}
			if err := scheduler.Schedule(context.Background(), store); err != nil {
				t.Fatal(err)
			}
			runs := 0
			runner := executorFunc(func(context.Context, Store, Entry) error { runs++; return nil })
			if _, err := store.Execute(context.Background(), "daily", true, runner, nil); err != nil || runs != 1 {
				t.Fatalf("initial timer run: %v, runs %d", err, runs)
			}
			e, err := store.Get("daily")
			if err != nil {
				t.Fatal(err)
			}
			if workflow {
				writeRoutineWorkflow(t, store, key+": "+after)
			} else {
				if err := e.LimitPolicy.SetText(key, after); err != nil {
					t.Fatal(err)
				}
				if err := store.save(e); err != nil {
					t.Fatal(err)
				}
			}
			now = now.Add(2 * time.Hour)
			for range 2 {
				_, err = store.Execute(context.Background(), "daily", true, runner, nil)
				if !errors.Is(err, ErrScheduleChanged) || runs != 1 {
					t.Fatalf("changed limit ran: err=%v, runs=%d; want schedule refusal and runs=1", err, runs)
				}
				if want := "routine changed since it was scheduled: daily; run grid routine schedule again to approve it"; err.Error() != want {
					t.Fatalf("different refusal: %v", err)
				}
			}
			current, err := store.Get("daily")
			if err != nil || current.LastRun != e.LastRun || current.Outcome != e.Outcome {
				t.Fatalf("refusal changed run state: %+v %v", current, err)
			}
			osCalls := len(calls.calls)
			if err := scheduler.Schedule(context.Background(), store); err != nil || len(calls.calls) != osCalls {
				t.Fatalf("re-arm: %v", err)
			}
			if _, err := store.Execute(context.Background(), "daily", true, runner, nil); err != nil || runs != 2 {
				t.Fatalf("re-armed timer: %v, runs %d", err, runs)
			}
			if workflow {
				raw, err := store.brain().ReadConfig(store.level(), "routines", "daily")
				if err != nil || strings.Contains(string(raw), key) {
					t.Fatalf("workflow default persisted in entry: %v", err)
				}
			}
		})
	}
}
