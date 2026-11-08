package cli

import (
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
