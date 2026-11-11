package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/task"
)

var remainingTaskCommands = []string{"deliver", "proof add", "proof index", "history add", "land", "brain show", "brain search", "brain keep", "brain decide"}

func remainingTaskArgs(command, operand string) []string {
	var args []string
	switch command {
	case "deliver":
		args = []string{"deliver", "--outcome", "Ready", "--task", operand}
	case "proof add":
		args = []string{"proof", "add", "1", "--text", "Passed", "--task", operand}
	case "proof index":
		args = []string{"proof", "index", "--task", operand}
	case "history add":
		args = []string{"history", "add", "Checked result", "--task", operand}
	case "land":
		args = []string{"land", operand, "--dry-run"}
	case "brain show":
		args = []string{"brain", "show", "lesson", "--task", operand}
	case "brain search":
		args = []string{"brain", "search", "checks", "--task", operand}
	case "brain keep":
		args = []string{"brain", "keep", "--task", operand}
	case "brain decide":
		args = []string{"brain", "decide", operand, "lesson", "reject", "--note", "Reviewed"}
	}
	return append(args, "--json")
}

func remainingTaskFixture(t *testing.T, command string) (*harness, task.Task, task.Task) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	personal := taskFixture(t, h)
	projectDir := filepath.Join(h.root, "example-project")
	h.mkdir(projectDir)
	shared, err := taskWriter(h.env).New(task.NewOptions{Cwd: projectDir, Words: "Shared task"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(shared.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: check\n  jobs: []\n")
	h.write(filepath.Join(h.gridHome, "projects", shared.Project, "knowledge", "deploy", "lands.md"), "---\nlands:\n  target: local\n  branch: main\n  command: echo landed\n  needs-approve: true\n---\n")
	if command == "deliver" {
		approveDeliveryPlan(t, h, shared)
		addDeliveryProof(t, h, shared.Path)
	}
	if strings.HasPrefix(command, "brain ") {
		n := brain.Note{Name: "lesson", Topic: "conventions", Type: "reference", Description: "When checking changes", Summary: "Run the checks", Source: shared.ID, Level: "project", Status: "accepted", Origin: brain.Origin{Kind: "session", ID: "example"}, Body: "Run checks before landing.\n"}
		b, err := brain.Encode(n)
		if err != nil {
			t.Fatal(err)
		}
		h.write(filepath.Join(h.gridHome, "projects", shared.Project, "knowledge", "conventions", "lesson.md"), string(b))
		n.Status = "proposed"
		b, err = brain.Encode(n)
		if err != nil {
			t.Fatal(err)
		}
		h.write(filepath.Join(shared.Path, "brain-proposals", "lesson.md"), string(b))
	}
	return h, shared, personal
}

func TestRemainingTaskOperands(t *testing.T) {
	for _, command := range remainingTaskCommands {
		t.Run(command, func(t *testing.T) {
			for _, form := range []string{"id", "folder", "folder-name", "relative", "outside", "unknown", "hidden-id", "hidden-folder", "hidden-folder-name"} {
				t.Run(form, func(t *testing.T) {
					h, shared, personal := remainingTaskFixture(t, command)
					operand, wantCode, wantMessage := shared.ID, 0, ""
					switch form {
					case "folder":
						operand = shared.Path
					case "folder-name":
						operand = filepath.Base(shared.Path)
					case "relative":
						var err error
						operand, err = filepath.Rel(h.home, shared.Path)
						if err != nil {
							t.Fatal(err)
						}
					case "outside":
						operand = filepath.Join(h.root, filepath.Base(shared.Path))
						h.mkdir(operand)
						wantCode, wantMessage = 2, "invalid task operand: folder must be directly inside an accessible project's tasks directory"
					case "unknown", "hidden-id", "hidden-folder", "hidden-folder-name":
						h.env.Getenv = func(key string) string {
							if key == "GRID_ROLE" {
								return "guest"
							}
							return ""
						}
						operand = "T-9999"
						if form == "hidden-id" {
							operand = personal.ID
						}
						if form == "hidden-folder" {
							operand = personal.Path
						}
						if form == "hidden-folder-name" {
							operand = filepath.Base(personal.Path)
						}
						wantCode, wantMessage = 1, "task not found"
					}
					before := treeBytes(t, h.gridHome)
					code := h.run(h.home, remainingTaskArgs(command, operand)...)
					if code != wantCode {
						t.Errorf("exit %d, want %d: %s", code, wantCode, h.norm(h.errb.String()))
					}
					if wantCode == 0 {
						if !json.Valid(h.out.Bytes()) || h.errb.Len() != 0 {
							t.Errorf("expected JSON success: %s %s", h.norm(h.out.String()), h.norm(h.errb.String()))
						}
						return
					}
					// All commands use the same refusal body and exit code as L85 task show.
					got := h.errb.String()
					errorCode := CodeNoSuchTask
					if wantCode == 2 {
						errorCode = CodeInvalidArgument
					}
					assertErrorOutput(t, h.out.String(), h.errb.String(), errorCode)
					if !strings.HasSuffix(got, ": "+wantMessage+"\n") {
						t.Errorf("expected refusal %q, got %s", wantMessage, h.norm(got))
					}
					baselineCode := h.run(h.home, "task", "show", operand)
					if baselineCode != wantCode || !strings.HasSuffix(h.errb.String(), ": "+wantMessage+"\n") {
						t.Errorf("L85 refusal: exit %d: %s", baselineCode, h.norm(h.errb.String()))
					}
					if !reflect.DeepEqual(before, treeBytes(t, h.gridHome)) {
						t.Fatal("refused operand changed files")
					}
				})
			}
		})
	}
}

func TestRemainingTaskOperandHelp(t *testing.T) {
	commands := append([]string{"proof", "history", "brain"}, remainingTaskCommands...)
	for _, command := range commands {
		t.Run(command, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, append(strings.Fields(command), "--help")...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			text := h.out.String() + h.errb.String()
			if !strings.Contains(text, "<task>") || !strings.Contains(text, "<task> is an id, exact folder name, or folder path inside a project tasks directory.\n") {
				t.Fatalf("missing L85 task operand help: %s", text)
			}
			golden.Assert(t, strings.ReplaceAll(command, " ", "-")+"-help", text)
		})
	}
}
