package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
)

// The FP1R2 review's PROBE2 (FP1R3 D1, D3). `grid task new` run from a task
// folder without --project: the person gate reads only that task's
// workflow.md, so alpha's workflows.md is read by task creation alone. Linked,
// it exits 1 with linked_file and makes no task, for an agent and a person.
// Unlinked, the task carries its workflow_digest; after S1 the guard holds it
// and deliver exits 1 with workflow_changed. Before FP1R3 the linked case made
// a task with no digest, and S1 released its plan.
func TestTaskNewFromTaskFolderRefusesLinkedWorkflows(t *testing.T) {
	for _, row := range []struct {
		name, agent string
		linked      bool
	}{
		{"unlinked", "", false},
		{"linked, person", "", true},
		{"linked, agent", "codex", true},
	} {
		t.Run(row.name, func(t *testing.T) {
			w := newWCTask(t, "large", "build")
			h := w.h
			tasks := filepath.Join(h.gridHome, "projects", "alpha", "tasks")
			before := wcEntries(t, tasks)
			link := filepath.Join(t.TempDir(), "t")
			if row.linked {
				if err := os.Link(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"), link); err != nil {
					t.Fatal(err)
				}
			}
			procs := h.env.Procs
			if row.agent != "" {
				h.env.Procs = testProcs(row.agent)
			}
			code := h.run(w.dir, "task", "new", "Third", "--workflow", "Large", "--size", "large", "--json")
			h.env.Procs = procs
			out, errText := h.out.String(), h.errb.String()
			if row.linked {
				if code != 1 || !strings.HasPrefix(out, `{"error":{"code":"linked_file","message":"grid task new: `) || !strings.Contains(errText, "linked_file") {
					t.Errorf("linked task new: exit %d, stdout %q, stderr %q; want exit 1 with linked_file", code, out, errText)
				}
				if strings.Contains(out+errText, h.root) {
					t.Errorf("linked task new names a path: %s %s", out, errText)
				}
				if after := wcEntries(t, tasks); !slices.Equal(after, before) {
					t.Errorf("tasks %v, before %v; want no task made", after, before)
				}
				if code != 0 {
					return
				}
				// The task was made anyway: show what the missing binding releases.
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
			}
			var made struct{ ID, Path string }
			if code != 0 || json.Unmarshal([]byte(out), &made) != nil || made.Path == "" {
				t.Fatalf("task new from the task folder: exit %d %s %s", code, out, errText)
			}
			w2 := &wcTask{t: t, h: h, id: made.ID, dir: made.Path}
			if w2.digest() == "" {
				t.Errorf("task new wrote no workflow_digest: %q", w2.read("task.md"))
			}
			text := w2.read("task.md")
			h.write(filepath.Join(w2.dir, "task.md"), strings.Replace(text, "\nstate: queued\n", "\nstate: running\nstep: plan\n", 1))
			h.write(filepath.Join(w2.dir, "history.md"), "")
			h.write(filepath.Join(w2.dir, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n")
			if _, err := (proof.Store{TaskDir: w2.dir}).Add(proof.Input{Item: "1", Text: "ok", Kind: "check", By: "grid"}); err != nil {
				t.Fatal(err)
			}
			if got := w2.hook(); got != "deny" {
				t.Errorf("at the plan gate before S1: guard %q, want deny", got)
			}
			w2.s1()
			if got := w2.hook(); got != "deny" {
				t.Errorf("after S1: guard %q, want deny", got)
			}
			dcode, dout := w2.deliver()
			if dcode != 1 || !strings.Contains(dout, `{"error":{"code":"workflow_changed"`) {
				t.Errorf("after S1: deliver --json exit %d, stdout %q, stderr %q; want 1 with workflow_changed", dcode, dout, h.errb)
			}
		})
	}
}

func wcEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
