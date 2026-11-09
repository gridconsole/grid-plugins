package task

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"github.com/gridconsole/grid-plugins/internal/workflow"
	"go.yaml.in/yaml/v4"
)

const inlineList = "workflow:\n  - {step: plan, kind: approve}\n  - {step: build, kind: run}\n"

var workflowLine = regexp.MustCompile(`(?m)^workflow:.*\n`)

// inlineTask gives a task the old plugin's inline workflow list.
func inlineTask(t *testing.T, task Task, list string) string {
	t.Helper()
	path := filepath.Join(task.Path, "task.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.Replace(workflowLine.ReplaceAllString(string(data), ""), "state: ", list+"state: ", 1))
	return path
}

func keepsInlineSteps(t *testing.T, s Store, task Task, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []taskfile.WorkflowStep{{Step: "plan", Kind: "approve"}, {Step: "build", Kind: "run"}}
	if got := taskfile.WorkflowSteps(string(data)); !reflect.DeepEqual(got, want) {
		t.Fatalf("WorkflowSteps = %+v; task.md:\n%s", got, data)
	}
	got, err := workflow.TaskSteps(s.Resolver, task.Project, string(data))
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("TaskSteps = %+v, %v; task.md:\n%s", got, err, data)
	}
}

func TestChangeKeepsInlineWorkflowList(t *testing.T) {
	s, task := writeFixture(t)
	path := inlineTask(t, task, inlineList)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
		t.Fatal(err)
	}
	keepsInlineSteps(t, s, task, path)
}

func TestDeliverKeepsInlineWorkflowList(t *testing.T) {
	s, task := deliverFixture(t, "")
	path := inlineTask(t, task, inlineList)
	if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	keepsInlineSteps(t, s, task, path)
}

// A flow mapping split over two lines is one YAML item but not one list item
// to the plugin's reader; the dump joins it, so the list would change.
func TestChangeRefusesAChangedWorkflowList(t *testing.T) {
	s, task := writeFixture(t)
	path := inlineTask(t, task, "workflow:\n  - {step: plan,\n     kind: approve}\n  - {step: build, kind: run}\n")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err == nil || !strings.Contains(err.Error(), "workflow list") {
		t.Fatalf("err=%v", err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Fatalf("task.md changed:\n%s", after)
	}
}

func TestRewriteTaskRefusesADroppedWorkflowList(t *testing.T) {
	data := []byte("---\nid: T-01\n" + inlineList + "state: queued\n---\n")
	out, err := rewriteTask(data, func(mapping *yaml.Node) error {
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if mapping.Content[i].Value == "workflow" {
				mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
				return nil
			}
		}
		return nil
	}, nil)
	if err == nil || out != nil {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
