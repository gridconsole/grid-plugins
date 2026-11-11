package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// These tests use the files the app's grid-worker really writes: an EMPTY worker.lock
// that the worker holds with an OS lock for its lifetime, and worker-status.json
// with {pid, version, started_at, uptime_seconds, address, token}.

const (
	testMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	secretToken = "5ecr3t-token-value-0123456789abcdef"
	// appStatusJSON is the shape of the app's instance.go endpoint struct.
	appStatusJSON = `{"pid":4242,"version":"0.9.1-test","started_at":"2026-11-08T09:00:00.123456789Z","uptime_seconds":61,"address":"127.0.0.1:43111","token":"` + secretToken + `"}`
)

func appHome(t *testing.T) string {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	writeApp(t, home, "machine", testMachine+"\n")
	return home
}

func writeApp(t *testing.T, home, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// holdLock makes worker.lock an empty file that another handle holds, as the running
// app does.
func holdLock(t *testing.T, home string) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(home, "worker.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
}

// lockIsHeld tries the lock from a fresh handle, as the app does.
func lockIsHeld(t *testing.T, home string) bool {
	t.Helper()
	f, err := os.Open(filepath.Join(home, "worker.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = fslock.TryLockExclusive(f)
	if errors.Is(err, fslock.ErrLocked) {
		return true
	}
	if err != nil {
		t.Fatal(err)
	}
	fslock.Unlock(f)
	return false
}

func TestAppFilesEmptyLockAndStatusListOneWorker(t *testing.T) {
	home := appHome(t)
	writeApp(t, home, "worker-status.json", appStatusJSON)
	holdLock(t, home)
	if st, err := os.Stat(filepath.Join(home, "worker.lock")); err != nil || st.Size() != 0 {
		t.Fatalf("worker.lock is not empty: %v %v", st, err)
	}
	got, err := (Store{GridHome: home}).List()
	if err != nil {
		t.Fatalf("an empty worker.lock failed the list: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d workers: %+v", len(got), got)
	}
	e := got[0]
	if e.Machine != testMachine || e.PID != 4242 || e.Version != "0.9.1-test" || e.Address != "127.0.0.1:43111" ||
		!e.StartedAt.Equal(time.Date(2026, 11, 8, 9, 0, 0, 123456789, time.UTC)) {
		t.Fatalf("entry %+v", e)
	}
}

func TestAppFilesRunningIsDecidedByTheLock(t *testing.T) {
	t.Run("held by another handle", func(t *testing.T) {
		home := appHome(t)
		writeApp(t, home, "worker-status.json", appStatusJSON)
		holdLock(t, home)
		got, err := (Store{GridHome: home}).List()
		if err != nil || len(got) != 1 || got[0].State != "running" || got[0].Presence != "reported" {
			t.Fatalf("got %+v, %v", got, err)
		}
		if !lockIsHeld(t, home) {
			t.Fatal("the probe let go of the app's lock")
		}
	})
	t.Run("free with a leftover status file", func(t *testing.T) {
		home := appHome(t)
		writeApp(t, home, "worker.lock", "")
		writeApp(t, home, "worker-status.json", appStatusJSON)
		got, err := (Store{GridHome: home}).List()
		if err != nil || len(got) != 1 || got[0].State != "stopped" || got[0].Presence != "stale" {
			t.Fatalf("got %+v, %v", got, err)
		}
		if got[0].PID != 4242 {
			t.Errorf("a stale entry lost its pid: %+v", got[0])
		}
		if lockIsHeld(t, home) {
			t.Fatal("the probe kept the lock it took")
		}
	})
}

func TestAppFilesNoFilesListNothingAndCreateNoLock(t *testing.T) {
	home := appHome(t)
	got, err := (Store{GridHome: home}).List()
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %#v, %v; want an empty list", got, err)
	}
	if _, err := os.Lstat(filepath.Join(home, "worker.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the probe created worker.lock: %v", err)
	}
	// A leftover status file alone does not make the probe create the lock either.
	writeApp(t, home, "worker-status.json", appStatusJSON)
	if _, err := (Store{GridHome: home}).List(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(home, "worker.lock")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the probe created worker.lock next to a status file: %v", err)
	}
}

// The app takes the lock before it writes the status file, and a crash can leave a
// torn one. The lock alone says a worker runs.
func TestAppFilesHeldLockWithoutUsableStatusIsRunning(t *testing.T) {
	for name, body := range map[string]*string{"no status file": nil, "empty status file": new(string)} {
		t.Run(name, func(t *testing.T) {
			home := appHome(t)
			if body != nil {
				writeApp(t, home, "worker-status.json", *body)
			}
			holdLock(t, home)
			got, err := (Store{GridHome: home}).List()
			if err != nil || len(got) != 1 || got[0].State != "running" || got[0].Presence != "reported" || got[0].PID != 0 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestAppFilesTokenIsNeverListed(t *testing.T) {
	home := appHome(t)
	writeApp(t, home, "worker-status.json", appStatusJSON)
	holdLock(t, home)
	got, err := (Store{GridHome: home}).List()
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, shown := range []string{string(b), fmt.Sprintf("%+v %#v", got, got)} {
		if strings.Contains(shown, secretToken) || strings.Contains(strings.ToLower(shown), "token") {
			t.Fatalf("the token shows in %s", shown)
		}
	}
}
