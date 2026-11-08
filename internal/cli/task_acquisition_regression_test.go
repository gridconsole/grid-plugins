package cli

import (
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"os"
	"path/filepath"
	"testing"
)

func TestGuestTaskTakeLeavesNoGrant(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, path := feedHarness(t)
	t.Setenv("GRID_ROLE", "guest")
	if code := h.run(path, taskTakeArgs(pid, start)...); code == 0 {
		t.Fatal("guest acquired task")
	}
	if _, err := os.Stat(filepath.Join(path, "feed", "lease")); !os.IsNotExist(err) {
		t.Fatalf("guest take left lease storage: %v", err)
	}
}

func TestEmptyTaskLeaseRecoversCLI(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, path := feedHarness(t)
	h.write(filepath.Join(path, "feed", "lease", "task.json"), "")
	if code := h.run(path, "lease", "status", "task"); code != 0 {
		t.Errorf("empty task status: %d %s", code, h.errb)
	}
	if code := h.run(path, taskTakeArgs(pid, start)...); code != 0 {
		t.Fatalf("empty task take: %d %s", code, h.errb)
	}
}

func TestTaskTakeoverPreservesGenerationAndEpochChecks(t *testing.T) {
	pid, start := taskLeaseIdentity(t)
	h, path := feedHarness(t)
	args := taskTakeArgs(pid, start)
	if code := h.run(path, append(args, "--gen", "2")...); code != 0 {
		t.Fatal(h.errb)
	}
	takeover := append(taskTakeArgs(pid, start), "--takeover", "--server-offline-since", "2000-01-01T00:00:00Z")
	if code := h.run(path, append(takeover, "--expected-epoch", "2.1", "--gen", "3")...); code != 0 {
		t.Fatal(h.errb)
	}
	store := lease.TaskStore{Feed: feed.Store{TaskDir: path, TaskID: feedTaskID}}
	for _, args := range [][]string{
		append(append([]string{}, takeover...), "--expected-epoch", "2.1", "--gen", "4"),
		append(append([]string{}, takeover...), "--expected-epoch", "3.2", "--gen", "2"),
	} {
		if code := h.run(path, args...); code == 0 {
			t.Fatal("stale epoch or regressed generation took over")
		}
		got, err := store.Read("task")
		if err != nil || got.Gen != 3 || got.Fence != 2 {
			t.Fatalf("refusal changed grant: %+v %v", got, err)
		}
	}
}
