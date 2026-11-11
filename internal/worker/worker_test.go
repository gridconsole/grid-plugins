package worker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatusFormat pins the file the app writes (worker-status.json) and the entry
// List builds from it: the entry has the words callers read and nothing else, so
// the token and uptime_seconds of the file do not show.
func TestStatusFormat(t *testing.T) {
	var status appStatus
	if err := json.Unmarshal([]byte(appStatusJSON), &status); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 8, 9, 0, 0, 123456789, time.UTC)
	if status.PID != 4242 || status.Version != "0.9.1-test" || !status.StartedAt.Equal(want) || status.Address != "127.0.0.1:43111" {
		t.Fatalf("decoded %+v", status)
	}
	b, err := json.Marshal(Entry{Machine: testMachine, PID: status.PID, Version: status.Version, StartedAt: status.StartedAt, Address: status.Address, State: "running", Presence: "reported"})
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]any
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"machine", "pid", "version", "started_at", "address", "state", "presence"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("entry %s lacks %q", b, k)
		}
		delete(keys, k)
	}
	if len(keys) != 0 {
		t.Errorf("entry %s has more than the listed words: %v", b, keys)
	}
}

// TestList walks every combination of the two app files: a lock nobody holds is
// "stopped" only while a status file is left; a held lock is "running".
func TestList(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		lockFile, held, stat bool
		state, presence      string // "" for no worker
	}{
		{"nothing", false, false, false, "", ""},
		{"held lock and status", true, true, true, "running", "reported"},
		{"held lock, status not written yet", true, true, false, "running", "reported"},
		{"free lock and leftover status", true, false, true, "stopped", "stale"},
		{"leftover status, no lock file", false, false, true, "stopped", "stale"},
		{"free lock only", true, false, false, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := appHome(t)
			if tc.stat {
				writeApp(t, home, "worker-status.json", appStatusJSON)
			}
			if tc.held {
				holdLock(t, home)
			} else if tc.lockFile {
				writeApp(t, home, "worker.lock", "")
			}
			got, err := (Store{GridHome: home}).List()
			if err != nil {
				t.Fatal(err)
			}
			if tc.state == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected workers: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].State != tc.state || got[0].Presence != tc.presence || got[0].Machine != testMachine {
				t.Fatalf("got %+v, want state %s presence %s", got, tc.state, tc.presence)
			}
		})
	}
}

func TestWorkerListRejectsUnsafeFiles(t *testing.T) {
	for _, name := range []string{"worker-status.json", "worker.lock"} {
		for _, kind := range []string{"malformed", "symlink", "directory"} {
			if name == "worker.lock" && kind == "malformed" {
				continue // the lock file is never read, so any content is fine
			}
			t.Run(name+" "+kind, func(t *testing.T) {
				home := appHome(t)
				path := filepath.Join(home, name)
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
}
