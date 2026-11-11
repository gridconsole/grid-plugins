package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// These tests run `grid worker list` on the files the app's grid-worker writes: an
// empty worker.lock it holds with an OS lock, and worker-status.json.

const (
	appWorkerToken  = "5ecr3t-token-value-0123456789abcdef"
	appWorkerStatus = `{"pid":4242,"version":"0.9.1-test","started_at":"2026-11-08T09:00:00Z","uptime_seconds":61,"address":"127.0.0.1:43111","token":"` + appWorkerToken + `"}`
	appWorkerHome   = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
)

// appWorkerFiles writes the app's files into the Grid home. held makes another handle
// hold the empty worker.lock; lock without held leaves it free; status writes the file.
func appWorkerFiles(t *testing.T, h *harness, lock, held, status bool) {
	t.Helper()
	h.write(filepath.Join(h.gridHome, "machine"), appWorkerHome+"\n")
	if status {
		h.write(filepath.Join(h.gridHome, "worker-status.json"), appWorkerStatus)
	}
	if !lock {
		return
	}
	path := filepath.Join(h.gridHome, "worker.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	if held {
		if err := fslock.LockExclusive(f); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkerListOnTheAppsFiles(t *testing.T) {
	t.Run("a held empty lock and a status file list one running worker", func(t *testing.T) {
		h := newHarness(t)
		appWorkerFiles(t, h, true, true, true)
		if code := h.run(h.home, "worker", "list", "--json"); code != 0 {
			t.Fatalf("exit %d: %s", code, h.errb)
		}
		if err := contract.Validate("cmd-worker-list", h.out.Bytes()); err != nil {
			t.Fatalf("%v\n%s", err, h.out)
		}
		var got []map[string]any
		if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got) != 1 {
			t.Fatalf("%v: %s", err, h.out)
		}
		want := map[string]any{"machine": appWorkerHome, "pid": 4242.0, "version": "0.9.1-test", "started_at": "2026-11-08T09:00:00Z", "address": "127.0.0.1:43111", "state": "running", "presence": "reported"}
		if len(got[0]) != len(want) {
			t.Errorf("entry %v, want exactly %v", got[0], want)
		}
		for k, v := range want {
			if got[0][k] != v {
				t.Errorf("%s = %v, want %v", k, got[0][k], v)
			}
		}
		if code := h.run(h.home, "worker", "list"); code != 0 {
			t.Fatalf("text exit %d: %s", code, h.errb)
		}
		for _, line := range []string{"machine: " + appWorkerHome, "pid: 4242", "version: 0.9.1-test", "started_at: 2026-11-08T09:00:00.000Z", "address: 127.0.0.1:43111", "state: running", "presence: reported"} {
			if !strings.Contains(h.out.String(), line+"\n") {
				t.Errorf("text lacks %q:\n%s", line, h.out)
			}
		}
	})

	t.Run("a free lock and a leftover status file list a stopped, stale worker", func(t *testing.T) {
		h := newHarness(t)
		appWorkerFiles(t, h, true, false, true)
		if code := h.run(h.home, "worker", "list", "--json"); code != 0 {
			t.Fatalf("exit %d: %s", code, h.errb)
		}
		var got []struct{ State, Presence string }
		if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got) != 1 || got[0].State != "stopped" || got[0].Presence != "stale" {
			t.Fatalf("%v: %s", err, h.out)
		}
	})

	t.Run("no files list nothing and the probe creates no lock", func(t *testing.T) {
		h := newHarness(t)
		if code := h.run(h.home, "worker", "list", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != "[]" {
			t.Fatalf("exit %d: %q %s", code, h.out, h.errb)
		}
		if code := h.run(h.home, "worker", "list"); code != 0 || h.out.Len() != 0 {
			t.Fatalf("text exit %d: %q %s", code, h.out, h.errb)
		}
		if _, err := os.Lstat(filepath.Join(h.gridHome, "worker.lock")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("worker.lock was created: %v", err)
		}
	})

	t.Run("the token is in neither the text nor the JSON", func(t *testing.T) {
		h := newHarness(t)
		appWorkerFiles(t, h, true, true, true)
		for _, args := range [][]string{{"worker", "list"}, {"worker", "list", "--json"}} {
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("%v: exit %d: %s", args, code, h.errb)
			}
			if h.out.Len() == 0 {
				t.Fatalf("%v printed nothing", args)
			}
			if all := h.out.String() + h.errb.String(); strings.Contains(all, appWorkerToken) || strings.Contains(strings.ToLower(all), "token") {
				t.Errorf("%v shows the token:\n%s", args, all)
			}
		}
	})
}
