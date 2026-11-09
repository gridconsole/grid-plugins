package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

const heldList = "workflow:\n  - {step: plan, kind: approve}\n  - {step: build, kind: run}\n"

// A task writer must not move a held task past its plan gate. The reader sees
// "step: plan" and holds the task; a hidden continuation line made the dump write
// "step: plan build", a step the list names, so the hold read as released.
func TestChangeCannotReleaseAHeldInlineTask(t *testing.T) {
	for name, step := range map[string]string{
		"continuation line": "step: plan\n  build\n",
		"step on next line": "step:\n  build\n",
	} {
		t.Run(name, func(t *testing.T) {
			f, store := newTaskFixture(t)
			path := filepath.Join(f.task, "task.md")
			f.write(path, strings.Replace(newTask, "workflow: Check and ship\n", heldList+step, 1))
			before, _ := os.ReadFile(path)
			stage := planStage(string(before), "")
			if !heldStage(stage) || (name == "continuation line") != (stage == "gate") {
				t.Fatalf("fixture stage %q", stage)
			}
			if got := writesRepo(f); got != "deny" {
				t.Fatalf("before: %q", got)
			}
			_, err := store.Change(taskstore.WriteOptions{ID: "T-01", Action: "status", State: "running"})
			if err == nil || !strings.Contains(err.Error(), `key "step"`) {
				t.Fatalf("err=%v", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatalf("a refused write changed task.md:\n%s", after)
			}
			if got := planStage(string(after), ""); got != stage || !held(f.task, string(after), "") {
				t.Fatalf("stage %q, want %q still held", got, stage)
			}
			if got := writesRepo(f); got != "deny" {
				t.Fatalf("after: %q", got)
			}
		})
	}
}
