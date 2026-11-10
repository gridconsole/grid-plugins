package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// Deliver writes step: ship. On a task whose plan still waits for a person that
// moves the stage past the plan gate, so it must ask the guard's own question and
// refuse. The tasks are held the way the guard holds them: an inline list that
// names a plan gate, and a named workflow the guard cannot resolve.
var heldDeliveries = []struct{ name, workflow string }{
	{"inline list, step plan", heldList + "step: plan\n"},
	{"inline list, no step", heldList},
	{"named workflow", "workflow: Check and ship\n"},
	{"named workflow, step plan", "workflow: Check and ship\nstep: plan\n"},
}

func deliverableTask(t *testing.T, workflow, state string) (*fixture, taskstore.Store, string) {
	t.Helper()
	f, store := newTaskFixture(t)
	f.write(filepath.Join(f.task, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n")
	if _, err := (proof.Store{TaskDir: f.task}).Add(proof.Input{Item: "1", Text: "ok", Kind: "check", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(f.task, "task.md")
	f.write(path, "---\nid: T-01\ntitle: Example task\nproject: example\nsize: normal\n"+workflow+"state: "+state+"\ncreated: 2026-01-02T03:04:05Z\n---\n")
	return f, store, path
}

func deliver(store taskstore.Store, f *fixture) error {
	_, err := store.Deliver(taskstore.DeliverOptions{Cwd: f.task, Task: "T-01", Outcome: "Ready", By: "grid"})
	return err
}

// files is every regular file in the task folder with its bytes.
func files(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		out[strings.TrimPrefix(path, dir)] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A plan the person approved is not held, and delivers. (Deliver no longer
// refuses a held plan, S6c3; TestDeliverCannotReleaseAHeldPlan is gone.)
func TestDeliverStillReleasesAnApprovedPlan(t *testing.T) {
	for _, c := range heldDeliveries {
		t.Run(c.name, func(t *testing.T) {
			f, store, path := deliverableTask(t, c.workflow, "running")
			// Approve needs the task waiting; the receipt binds plan.md alone.
			f.write(path, newTask)
			if _, err := store.Approve(taskstore.WriteOptions{ID: "T-01"}, ""); err != nil {
				t.Fatal(err)
			}
			f.write(path, "---\nid: T-01\ntitle: Example task\nproject: example\nsize: normal\n"+c.workflow+"state: running\ncreated: 2026-01-02T03:04:05Z\n---\n")
			text, _ := os.ReadFile(path)
			if held(stepsFor(f.env, f.task), f.task, string(text), "") {
				t.Fatal("an approved plan is held")
			}
			if err := deliver(store, f); err != nil {
				t.Fatalf("Deliver: %v", err)
			}
		})
	}
}
