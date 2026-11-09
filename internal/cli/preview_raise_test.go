package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

const previewStepsTable = "| # | step | kind | who | sizes | on reject, back to |\n|---|------|------|-----|-------|--------------------|\n"

// previewLarge is the Large template: a plan approval, an L-only preview approval, a build.
const previewLarge = "| 1 | plan | approve | owner | all | |\n| 2 | preview | approve | owner | L | |\n| 3 | build | run | grid-builder | all | |\n"

const previewInline = "workflow:\n  - {step: plan, kind: approve}\n  - {step: preview, kind: approve}\n  - {step: build, kind: run}\n"

// previewFixture is a running large task in project alpha that names the Large
// workflow, with an approved plan and a spec.md.
func previewFixture(t *testing.T) (*harness, string, string) {
	return previewFixtureWith(t, previewLarge, "large", "Large", false)
}

// previewInlineFixture is the same task carrying the steps as an inline list.
func previewInlineFixture(t *testing.T) (*harness, string, string) {
	return previewFixtureWith(t, previewLarge, "large", "Large", true)
}

func previewFixtureWith(t *testing.T, rows, size, name string, inline bool) (*harness, string, string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: alpha\n---\n")
	h.env.Stdin = strings.NewReader("# Workflows\n\n## Large\napplies when: label large\nreview rounds: 2\n\n" + previewStepsTable + rows + "\n")
	if code := h.run(h.root, "workflow", "write", "--project", "alpha", "--rev", "none", "--file", "-", "--json"); code != 0 {
		t.Fatalf("workflow write: %d %s", code, h.errb)
	}
	created, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: h.home, Project: "alpha", Words: "Preview result", Size: size, Workflow: name})
	if err != nil {
		t.Fatal(err)
	}
	dir, id := created.Path, created.ID
	h.write(filepath.Join(dir, "plan.md"), "# Plan 1\n- [ ] 1 Build the page\n  proof: check\n  jobs: []\n")
	h.write(filepath.Join(dir, "spec.md"), "# Preview spec\n\nThe page layout.\n")
	for _, args := range [][]string{{"task", "status", id, "waiting"}, {"approve", id, "--json"}, {"task", "status", id, "running"}} {
		if code := h.run(dir, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
	}
	if inline {
		// Written after the Go moves: rewriteTask dumps list items at column 0, which taskfile.ParseFrontmatter drops (base defect).
		taskMD := filepath.Join(dir, "task.md")
		data, err := os.ReadFile(taskMD)
		if err != nil || !strings.Contains(string(data), "workflow: "+name+"\n") {
			t.Fatalf("no workflow name in task.md: %v\n%s", err, data)
		}
		h.write(taskMD, strings.Replace(string(data), "workflow: "+name+"\n", previewInline, 1))
	}
	return h, dir, id
}

func previewFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range []string{"task.md", "history.md", "approval.json", "approval-preview.json", "approval-ship.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out[name] = string(data)
	}
	return out
}

func checkRaised(t *testing.T, dir, id string, before map[string]string) {
	t.Helper()
	after := previewFiles(t, dir)
	rec, err := task.Read(dir)
	if err != nil {
		t.Fatal(err)
	}
	fm := taskfile.ParseFrontmatter(after["task.md"])
	if rec.State != "waiting" || fm == nil || fm.Data["step"] != "preview" || fm.Data["gate"] != "preview" || fm.Data["state"] != "waiting" {
		t.Fatalf("task.md not at the preview gate:\n%s", after["task.md"])
	}
	added := strings.TrimPrefix(after["history.md"], before["history.md"])
	if added == after["history.md"] || strings.Count(added, "\n") != 1 {
		t.Fatalf("want one history event, got %q", added)
	}
	var event task.Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(added), "- ")), &event); err != nil || event.What != "preview" || event.Note != "Preview spec" || event.ToState != "waiting" || event.Task != id {
		t.Fatalf("%+v %v", event, err)
	}
	for _, name := range []string{"approval.json", "approval-preview.json", "approval-ship.json"} {
		if after[name] != before[name] {
			t.Fatalf("%s changed", name)
		}
	}
}

func TestPreviewRaisesGateFromNamedWorkflow(t *testing.T) {
	h, dir, id := previewFixture(t)
	before := previewFiles(t, dir)
	if code := h.run(dir, "preview", "--task", id); code != 0 || h.out.String() != id+" preview raised; preview gate waiting\n" {
		t.Fatalf("exit %d: %q %s", code, h.out, h.errb)
	}
	checkRaised(t, dir, id, before)
}

func TestPreviewRaisesGateFromInlineWorkflow(t *testing.T) {
	h, dir, id := previewInlineFixture(t)
	before := previewFiles(t, dir)
	if code := h.run(dir, "preview", "--task", id); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	checkRaised(t, dir, id, before)
}

// The raised gate makes grid approve record the preview, not the plan.
func TestPreviewThenApproveRecordsPreviewApproval(t *testing.T) {
	h, dir, id := previewFixture(t)
	if code := h.run(dir, "preview", "--task", id); code != 0 {
		t.Fatal(h.errb)
	}
	before := previewFiles(t, dir)
	if code := h.run(dir, "approve", id, "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	after := previewFiles(t, dir)
	if after["approval.json"] != before["approval.json"] || after["approval-preview.json"] == "" {
		t.Fatal("approve at the raised gate did not write the preview receipt alone")
	}
	if !strings.Contains(after["history.md"], " · preview approved · ") {
		t.Fatalf("no preview approved line:\n%s", after["history.md"])
	}
}

// An agent may raise the preview, never approve it.
func TestPreviewAgentRaisesButCannotApprove(t *testing.T) {
	h, dir, id := previewFixture(t)
	agent := map[string]string{"GRID_AGENT": "builder", "GRID_LEASE": "J-1", "GRID_ROLE": "worker"}
	h.env.Getenv = func(key string) string { return agent[key] }
	if code := h.run(dir, "preview", "--task", id, "--by", "grid-builder"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	before := previewFiles(t, dir)
	if code := h.run(dir, "approve", id); code != 3 {
		t.Fatalf("agent approve exit %d, want 3: %s", code, h.errb)
	}
	if after := previewFiles(t, dir); after["approval-preview.json"] != before["approval-preview.json"] || after["approval-preview.json"] != "" {
		t.Fatal("agent approve wrote a preview receipt")
	}
}

func TestPreviewRefusals(t *testing.T) {
	noPreview := "| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n"
	previewFirst := "| 1 | preview | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n"
	for _, tc := range []struct {
		name, rows, size, workflow string
		setup                      func(h *harness, dir, id string)
		args                       []string
		want                       string
	}{
		{"no preview step", noPreview, "large", "Large", nil, nil, "no preview approve step"},
		{"preview filtered out by size", previewLarge, "normal", "Large", nil, nil, "no preview approve step"},
		{"size auto with an L-only step", previewLarge, "auto", "Large", nil, nil, "cannot resolve the task's workflow: task size is not set"},
		{"workflow name not found", previewLarge, "large", "Missing", nil, nil, "cannot resolve the task's workflow: no workflow is named"},
		{"no plan approval before preview", previewFirst, "large", "Large", nil, nil, "no plan approval before preview"},
		{"spec missing", previewLarge, "large", "Large", func(h *harness, dir, _ string) {
			if err := os.Remove(filepath.Join(dir, "spec.md")); err != nil {
				h.t.Fatal(err)
			}
		}, nil, "spec.md is missing"},
		{"spec blank", previewLarge, "large", "Large", func(h *harness, dir, _ string) { h.write(filepath.Join(dir, "spec.md"), " \n\t\n") }, nil, "spec.md has no nonblank line"},
		{"spec symlink", previewLarge, "large", "Large", func(h *harness, dir, _ string) {
			h.write(filepath.Join(dir, "design.md"), "# Real\n")
			if err := os.Remove(filepath.Join(dir, "spec.md")); err != nil {
				h.t.Fatal(err)
			}
			if err := os.Symlink("design.md", filepath.Join(dir, "spec.md")); err != nil {
				h.t.Skip(err)
			}
		}, nil, "refusing symlink"},
		{"spec outside the task", previewLarge, "large", "Large", func(h *harness, dir, _ string) {
			h.write(filepath.Join(filepath.Dir(dir), "x"), "# Outside\n")
		}, []string{"--spec", "../x"}, "../x is not a file inside the task folder"},
		{"plan not approved", previewLarge, "large", "Large", func(h *harness, dir, _ string) {
			h.write(filepath.Join(dir, "plan.md"), "# Plan 1\n- [ ] 1 Build another page\n  proof: check\n  jobs: []\n")
		}, nil, "the plan is not approved"},
		{"paused", previewLarge, "large", "Large", func(h *harness, dir, id string) { h.mustRun(dir, "task", "status", id, "paused") }, nil, "Preview blocked: "},
		{"closed", previewLarge, "large", "Large", func(h *harness, dir, id string) {
			data, err := os.ReadFile(filepath.Join(dir, "task.md"))
			if err != nil {
				h.t.Fatal(err)
			}
			h.write(filepath.Join(dir, "task.md"), strings.Replace(string(data), "state: running\n", "state: closed\n", 1))
		}, nil, "task is closed or stopped"},
		{"already waiting", previewLarge, "large", "Large", func(h *harness, dir, id string) { h.mustRun(dir, "task", "status", id, "waiting") }, nil, "Preview blocked: "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, dir, id := previewFixtureWith(t, tc.rows, tc.size, tc.workflow, false)
			if tc.setup != nil {
				tc.setup(h, dir, id)
			}
			before := previewFiles(t, dir)
			code := h.run(dir, append([]string{"preview", "--task", id}, tc.args...)...)
			if code != 1 || !strings.HasPrefix(h.errb.String(), "Preview blocked: ") || !strings.Contains(h.errb.String(), tc.want) {
				t.Fatalf("exit %d, want 1 with %q: %s", code, tc.want, h.errb)
			}
			if after := previewFiles(t, dir); after["task.md"] != before["task.md"] || after["history.md"] != before["history.md"] {
				t.Fatal("a refused preview changed the task")
			}
		})
	}
}

func TestPreviewUsageAndDryRun(t *testing.T) {
	h, dir, id := previewFixture(t)
	before := previewFiles(t, dir)
	if code := h.run(dir, "preview", "--task", id, "--bogus"); code != 2 {
		t.Fatalf("unknown flag exit %d, want 2", code)
	}
	if code := h.run(dir, "preview", "--task", id, "--dry-run"); code != 0 || !strings.Contains(h.out.String(), "Would raise preview gate") {
		t.Fatalf("dry run exit %d: %q %s", code, h.out, h.errb)
	}
	if after := previewFiles(t, dir); after["task.md"] != before["task.md"] || after["history.md"] != before["history.md"] {
		t.Fatal("dry run wrote the task")
	}
	if code := h.run(dir, "preview", "--task", id, "--dry-run", "--json"); code != 0 || !strings.Contains(h.out.String(), `"spec":"spec.md"`) {
		t.Fatalf("dry run json exit %d: %q %s", code, h.out, h.errb)
	}
}

func (h *harness) mustRun(cwd string, args ...string) {
	h.t.Helper()
	if code := h.run(cwd, args...); code != 0 {
		h.t.Fatalf("%v: exit %d %s", args, code, h.errb)
	}
}
