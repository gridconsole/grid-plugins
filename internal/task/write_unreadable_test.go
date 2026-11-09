package task

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
	"go.yaml.in/yaml/v4"
)

// Each list is one the plugin's reader cannot see: it reads the workflow as "",
// so the task is held for plan approval. The dump would turn it into an
// indented list the reader can see, and the held task would gain steps.
var unreadableLists = map[string]string{
	"items at column 0":      "workflow:\n- {step: build, kind: run}\n",
	"flow list on next line": "workflow:\n  [{step: build, kind: run}]\n",
}

// A split flow mapping is one YAML item but two lines to the reader.
const splitFlowList = "workflow:\n  - {step: build,\n     kind: run}\n"

type heldTask struct {
	task    Task
	dir     string
	text    string
	history []byte
}

func (h heldTask) read(t *testing.T) (string, []byte) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	history, _ := os.ReadFile(filepath.Join(h.dir, "history.md"))
	return string(data), history
}

func holdTask(t *testing.T, task Task, path string) heldTask {
	t.Helper()
	h := heldTask{task: task, dir: filepath.Dir(path)}
	h.text, h.history = h.read(t)
	if !taskfile.PlanApprovalRequired(h.text) {
		t.Fatalf("fixture is not held:\n%s", h.text)
	}
	return h
}

// refused asserts a write left task.md and history.md as they were and the
// reader still holds the task.
func (h heldTask) refused(t *testing.T, err error, why string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), why) {
		t.Fatalf("err=%v; want a refusal naming %q", err, why)
	}
	text, history := h.read(t)
	if text != h.text || string(history) != string(h.history) {
		t.Fatalf("a refused write changed the task:\n%s", text)
	}
	if !taskfile.PlanApprovalRequired(text) {
		t.Fatalf("PlanApprovalRequired flipped to false:\n%s", text)
	}
}

func TestChangeRefusesAWorkflowListTheReaderCannotSee(t *testing.T) {
	for name, list := range unreadableLists {
		t.Run(name, func(t *testing.T) {
			s, task := writeFixture(t)
			h := holdTask(t, task, inlineTask(t, task, list))
			_, err := s.Change(WriteOptions{ID: task.ID, Action: "close"})
			h.refused(t, err, "workflow list")
		})
	}
}

func TestDeliverRefusesAWorkflowListTheReaderCannotSee(t *testing.T) {
	for name, list := range unreadableLists {
		t.Run(name, func(t *testing.T) {
			s, task := deliverFixture(t, "")
			h := holdTask(t, task, inlineTask(t, task, list))
			_, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"})
			h.refused(t, err, "workflow list")
		})
	}
}

// The reader also holds a task whose header has a key it refuses; a write that
// removes that key would release the hold.
func TestDeliverRefusesToDropTheKeyThatHoldsATask(t *testing.T) {
	s, task := deliverFixture(t, "next: {__proto__: x}\n")
	h := holdTask(t, task, inlineTask(t, task, "workflow:\n  - {step: build, kind: run}\n"))
	_, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"})
	h.refused(t, err, "__proto__")
}

func TestRefusedWorkflowWriteSaysHowToFix(t *testing.T) {
	lists := map[string]string{"split flow mapping": splitFlowList}
	for name, list := range unreadableLists {
		lists[name] = list
	}
	for name, list := range lists {
		t.Run(name, func(t *testing.T) {
			s, task := writeFixture(t)
			inlineTask(t, task, list)
			_, err := s.Change(WriteOptions{ID: task.ID, Action: "close"})
			if err == nil {
				t.Fatal("the write was not refused")
			}
			for _, want := range []string{"workflow list is not readable", "indent each item under workflow:", "or remove the list, then retry", "nothing was written"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err=%q lacks %q", err, want)
				}
			}
		})
	}
}

func setWorkflow(value string, add bool) func(*yaml.Node) error {
	return func(mapping *yaml.Node) error {
		setScalar(mapping, "workflow", value, add)
		return nil
	}
}

func dropWorkflow(mapping *yaml.Node) error {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == "workflow" {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
		}
	}
	return nil
}

// The reader's workflow value counts whatever its type: a name, an empty value
// or an absent key is as much part of the task as a list.
func TestRewriteTaskRefusesAChangedWorkflowOfAnyType(t *testing.T) {
	cases := []struct {
		name, workflow string
		edit           func(*yaml.Node) error
	}{
		{"name renamed", "workflow: normal\n", setWorkflow("small", false)},
		{"name removed", "workflow: normal\n", dropWorkflow},
		{"empty value named", "workflow:\n", setWorkflow("normal", false)},
		{"absent key added", "", setWorkflow("normal", true)},
		{"list replaced by a name", "workflow:\n  - {step: build, kind: run}\n", setWorkflow("normal", false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := rewriteTask([]byte("---\nid: T-01\n"+c.workflow+"state: queued\n---\n"), c.edit, nil)
			if err == nil || out != nil {
				t.Fatalf("out=%q err=%v", out, err)
			}
		})
	}
}

// A write that leaves the workflow alone is not refused, whatever form it has.
func TestRewriteTaskKeepsAWorkflowItDoesNotTouch(t *testing.T) {
	for name, workflow := range map[string]string{
		"name":          "workflow: normal\n",
		"quoted name":   "workflow: 'normal'\n",
		"commented":     "workflow: normal # c\n",
		"empty value":   "workflow:\n",
		"empty string":  "workflow: \"\"\n",
		"null word":     "workflow: null\n",
		"absent":        "",
		"indented list": "workflow:\n  - {step: build, kind: run}\n",
		"flow list":     "workflow: [{step: build, kind: run}]\n",
		"empty list":    "workflow: []\n",
	} {
		t.Run(name, func(t *testing.T) {
			data := "---\nid: T-01\n" + workflow + "state: queued\n---\n"
			out, err := rewriteTask([]byte(data), func(mapping *yaml.Node) error {
				setScalar(mapping, "state", "closed", false)
				return nil
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got, want := taskfile.ParseFrontmatter(string(out)).Data["workflow"], taskfile.ParseFrontmatter(data).Data["workflow"]; !strings.Contains(string(out), "state: closed") || !reflect.DeepEqual(got, want) {
				t.Fatalf("workflow %#v; want %#v\n%s", got, want, out)
			}
		})
	}
}
