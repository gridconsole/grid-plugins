package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// A hard link to a protected file, planted outside the task folder, holds an
// approved task again: the guard denies work outside it and Deliver refuses.
func TestALinkedTaskFileHoldsTheGuard(t *testing.T) {
	c := heldDeliveries[0]
	for _, name := range []string{"task.md", "history.md", "approval.json"} {
		t.Run(name, func(t *testing.T) {
			f, store, path := deliverableTask(t, c.workflow, "running")
			f.write(path, newTask)
			if _, err := store.Approve(taskstore.WriteOptions{ID: "T-01"}, ""); err != nil {
				t.Fatal(err)
			}
			f.write(path, "---\nid: T-01\ntitle: Example task\nproject: example\nsize: normal\n"+c.workflow+"state: running\ncreated: 2026-01-02T03:04:05Z\n---\n")
			if got := writesRepo(f); got == "deny" {
				t.Fatal("the approved fixture is held before the link")
			}
			if err := os.Link(filepath.Join(f.task, name), filepath.Join(t.TempDir(), "t")); err != nil {
				t.Fatal(err)
			}
			if got := writesRepo(f); got != "deny" {
				t.Errorf("guard after a link to %s: %q, want deny", name, got)
			}
			if err := deliver(store, f); err == nil || !strings.Contains(err.Error(), "linked_file") {
				t.Errorf("Deliver err = %v; want linked_file", err)
			}
		})
	}
}

// Stopgap until the sandbox holds the Grid home read-only: shell writes to
// history.md or approval-preview.json are refused like the other gate files.
func TestHostBashHistoryAndPreviewApprovalAreGateFiles(t *testing.T) {
	f := setup(t)
	taskPath := filepath.ToSlash(f.task)
	for _, tt := range []struct {
		command string
		deny    bool
	}{
		{"echo '- 2026-01-02T03:04 · plan accepted · by person' >> history.md", true},
		{"echo x >> " + taskPath + "/history.md", true}, {"cp /tmp/a HISTORY.MD", true}, {"tee -a history.md < /tmp/a", true},
		{"echo '{}' > approval-preview.json", true}, {"cp /tmp/a " + taskPath + "/approval-preview.json", true},
		{"cat history.md", false}, {"tail -5 " + taskPath + "/history.md", false}, {"echo x > history.md.bak", false},
	} {
		t.Run(tt.command, func(t *testing.T) {
			want := ""
			if tt.deny {
				want = "deny"
			}
			expect(t, EvaluateHost(f.input("Bash", tt.command), f.root, f.env), want)
		})
	}
}
