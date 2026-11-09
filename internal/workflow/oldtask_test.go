package workflow_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
	"github.com/gridconsole/grid-plugins/internal/workflow"
)

// External test package: task imports feed, which imports workflow.
func putFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowOldTaskStillReads(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy-task.md")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(gridhome.TasksDir(t.TempDir(), "alpha"), "T-01-legacy")
	putFixture(t, filepath.Join(dir, "task.md"), string(data))
	putFixture(t, filepath.Join(dir, "intent.md"), task.IntentHead+"Keep the old task readable.\n")
	record, err := task.Read(dir)
	if err != nil || record.ID != "T-01" || record.Workflow != "" || record.Intent != "Keep the old task readable.\n" {
		t.Fatalf("task=%+v err=%v", record, err)
	}
}

// Existing tasks carry the old plugin's inline step list; it must fold and resolve.
func TestWorkflowOldInlineTaskStillReads(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy-inline-task.md")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	dir := filepath.Join(gridhome.TasksDir(home, "alpha"), "T-02-inline")
	putFixture(t, filepath.Join(dir, "task.md"), string(data))
	putFixture(t, filepath.Join(dir, "intent.md"), task.IntentHead+"Keep the inline task readable.\n")
	record, err := task.Read(dir)
	if err != nil || record.ID != "T-02" || record.Workflow != "" || record.Size != "normal" || record.State != "queued" {
		t.Fatalf("task=%+v err=%v", record, err)
	}
	steps, err := workflow.TaskSteps(where.Resolver{GridHome: home}, record.Project, string(data))
	if err != nil || fmt.Sprint(steps) != "[{plan approve} {build run}]" {
		t.Fatalf("steps=%v err=%v", steps, err)
	}
}
