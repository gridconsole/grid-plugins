package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func TestStatusFormat(t *testing.T) {
	fixture := `{"version":1,"machine":"01ARZ3NDEKTSV4RRFFQ69G5FAV","pid":123,"started_at":"2026-11-08T09:00:00Z","updated_at":"2026-11-08T09:00:30Z","state":"running"}`
	var status Status
	if err := json.Unmarshal([]byte(fixture), &status); err != nil {
		t.Fatal(err)
	}
	if err := status.Validate(); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(status)
	if err != nil || string(b) != fixture {
		t.Fatalf("roundtrip: %s %v", b, err)
	}
}

func TestList(t *testing.T) {
	now := time.Date(2026, 11, 8, 9, 1, 0, 0, time.UTC)
	for _, tc := range []struct {
		name         string
		status, lock bool
		presence     string
	}{{"empty", false, false, ""}, {"reported", true, true, "reported"}, {"missing lock", true, false, "stale"}, {"missing status", false, true, "unknown"}, {"old", true, true, "stale"}, {"pid mismatch", true, true, "stale"}, {"stopped", true, true, "stale"}, {"foreign", true, true, ""}, {"invalid", true, true, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			machine := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
			if err := os.WriteFile(filepath.Join(home, "machine"), []byte(machine), 0o600); err != nil {
				t.Fatal(err)
			}
			status := Status{Version: 1, Machine: machine, PID: 123, StartedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-30 * time.Second), State: "running"}
			lock := Lock{Version: 1, Machine: machine, PID: 123, StartedAt: status.StartedAt}
			switch tc.name {
			case "old":
				status.UpdatedAt = now.Add(-2 * time.Minute)
			case "pid mismatch":
				lock.PID++
			case "stopped":
				status.State = "stopped"
			case "foreign":
				status.Machine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"
			case "invalid":
				status.PID = 0
			}
			for name, value := range map[string]any{"worker.json": status, "worker.lock": lock} {
				if name == "worker.json" && !tc.status || name == "worker.lock" && !tc.lock {
					continue
				}
				b, _ := json.Marshal(value)
				if err := os.WriteFile(filepath.Join(home, name), b, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := (Store{GridHome: home, Now: func() time.Time { return now }}).List()
			if tc.name == "invalid" {
				if err == nil {
					t.Fatal("accepted invalid status")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tc.presence == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected workers: %v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Presence != tc.presence {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestWorkerListRejectsUnsafeFiles(t *testing.T) {
	for _, kind := range []string{"malformed", "symlink", "directory"} {
		t.Run(kind, func(t *testing.T) {
			home := gridhome.Real(t.TempDir())
			path := filepath.Join(home, "worker.json")
			switch kind {
			case "malformed":
				if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(filepath.Join(home, "elsewhere"), path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := (Store{GridHome: home}).List(); err == nil {
				t.Fatal("accepted unsafe worker file")
			}
		})
	}
}
