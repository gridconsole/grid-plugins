package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The project merge slot is gone: `grid lease` takes only the task word, and
// the task lease status answer is unchanged.

const taskStatusFreeJSON = `{"lease":"task","holder":null,"fence":0,"gen":0,"attempt":null,"since":"","pid":null,"pid_start":null,"process":"free"}` + "\n"

func TestLeaseTaskStatusJSONUnchanged(t *testing.T) {
	h, path := feedHarness(t)
	if code := h.run(path, "lease", "status", "task", "--task", feedTaskID, "--json"); code != 0 {
		t.Fatalf("status: %d %s", code, h.errb)
	}
	if got := h.out.String(); got != taskStatusFreeJSON {
		t.Fatalf("task lease status changed:\n got %q\nwant %q", got, taskStatusFreeJSON)
	}
}

func TestLeaseMergeSlotIsGone(t *testing.T) {
	holder := "worker@01K9J000000000000000000001"
	for _, args := range [][]string{
		{"lease", "take", "merge", "--project", "example", "--holder", holder},
		{"lease", "take", "--holder", holder},
		{"lease", "status", "merge", "--project", "example"},
		{"lease", "renew", "merge", "--project", "example", "--holder", holder, "--fence", "1"},
		{"lease", "release", "--project", "example", "--holder", holder, "--fence", "1"},
	} {
		h := newHarness(t)
		task := livenessProject(h)
		if code := h.run(task, args...); code != 2 {
			t.Errorf("%v: exit %d, want 2: %s", args, code, h.errb)
		}
		usage := h.errb.String()
		if !strings.Contains(usage, "Usage: grid lease") || !strings.Contains(usage, "task") || strings.Contains(strings.ToLower(usage), "merge") {
			t.Errorf("%v: usage must name only task: %q", args, usage)
		}
		if _, err := os.Stat(filepath.Join(h.gridHome, "projects", "example", "lease")); !os.IsNotExist(err) {
			t.Errorf("%v: wrote a project lease folder: %v", args, err)
		}
	}
}
