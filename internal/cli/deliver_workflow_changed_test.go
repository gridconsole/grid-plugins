package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
)

// wcLargeOpen is another project's Large: the same name with no plan gate.
const wcLargeOpen = "| 1 | build | run | grid-builder | all | |\n| 2 | ship | approve | owner | all | |\n"

func wcLarge(rows string) string {
	return "## Large\napplies when: label large\nreview rounds: 2\n\n" + previewStepsTable + rows + "\n"
}

// wcWorkspace is the workspace file: its Large has no plan gate, so a project
// rebuilt without its workflows.md reads that one.
var wcWorkspace = "# Workflows\n\n" + wcLarge(wcLargeOpen) +
	"## Standard\napplies when: nothing else matches\nreview rounds: 2\n\n" + previewStepsTable +
	"| 1 | plan | approve | owner | all | |\n| 2 | build | run | grid-builder | all | |\n\n"

var wcDigest = regexp.MustCompile(`\nworkflow_digest: (\S+)\n`)

type wcTask struct {
	t       *testing.T
	h       *harness
	id, dir string
}

// newWCTask makes a task with `grid task new` in project alpha, whose Large has
// the L-only preview step; size "" keeps the command's default, auto. The task
// runs at step with a checked plan and its proof, and nobody approved the plan.
func newWCTask(t *testing.T, size, step string) *wcTask {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.mkdir(filepath.Join(h.root, "repo"))
	h.write(filepath.Join(h.gridHome, "workflows.md"), wcWorkspace)
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project.md"), "---\nid: alpha\n---\n")
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "workflows.md"), "# Workflows\n\n"+wcLarge(previewLarge))
	args := []string{"task", "new", "Swap probe", "--project", "alpha", "--workflow", "Large", "--json"}
	if size != "" {
		args = append(args, "--size", size)
	}
	h.mustRun(h.home, args...)
	var made struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &made); err != nil || made.ID == "" || made.Path == "" {
		t.Fatalf("task new --json: %v %s", err, h.out)
	}
	w := &wcTask{t: t, h: h, id: made.ID, dir: made.Path}
	if size == "" {
		size = "auto"
	}
	text := w.read("task.md")
	if !strings.Contains(text, "\nstate: queued\n") || !strings.Contains(text, "\nsize: "+size+"\n") {
		t.Fatalf("task.md from grid task new: %q", text)
	}
	h.write(filepath.Join(w.dir, "task.md"), strings.Replace(text, "\nstate: queued\n", "\nstate: running\nstep: "+step+"\n", 1))
	h.write(filepath.Join(w.dir, "history.md"), "")
	h.write(filepath.Join(w.dir, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n")
	if _, err := (proof.Store{TaskDir: w.dir}).Add(proof.Input{Item: "1", Text: "ok", Kind: "check", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	return w
}

func (w *wcTask) read(name string) string {
	w.t.Helper()
	data, err := os.ReadFile(filepath.Join(w.dir, name))
	if err != nil {
		w.t.Fatal(err)
	}
	return string(data)
}

func (w *wcTask) digest() string {
	if m := wcDigest.FindStringSubmatch(w.read("task.md")); m != nil {
		return m[1]
	}
	return ""
}

// hook is `grid hook pretooluse` on an agent's Write into the repo from the
// task folder: "deny" or "".
func (w *wcTask) hook() string {
	w.t.Helper()
	b, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "wc", "tool_name": "Write", "cwd": w.dir,
		"tool_input": map[string]any{"file_path": filepath.Join(w.h.root, "repo", "file.go"), "content": "x"}})
	w.h.env.Stdin = bytes.NewReader(b)
	defer func() { w.h.env.Stdin = strings.NewReader("") }()
	if code := w.h.run(w.dir, "hook", "pretooluse"); code != 0 {
		w.t.Fatalf("hook pretooluse exit %d: %s %s", code, w.h.out, w.h.errb)
	}
	if strings.Contains(w.h.out.String(), `"permissionDecision":"deny"`) {
		return "deny"
	}
	return ""
}

func (w *wcTask) deliver() (int, string) {
	code := w.h.run(w.dir, "deliver", "--task", w.id, "--outcome", "Ready", "--json")
	return code, w.h.out.String()
}

func (w *wcTask) rename(from, to string) {
	w.t.Helper()
	if err := os.Rename(from, to); err != nil {
		w.t.Fatal(err)
	}
}

// rebuild renames alpha aside, makes a new alpha with the named files of from,
// and moves the task back in, so its folder still sits under the project
// task.md names.
func (w *wcTask) rebuild(from func(old string) string, names ...string) {
	w.t.Helper()
	alpha := filepath.Join(w.h.gridHome, "projects", "alpha")
	w.rename(alpha, alpha+"-old")
	w.h.mkdir(filepath.Join(alpha, "tasks"))
	src := from(alpha + "-old")
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			w.t.Fatal(err)
		}
		w.h.write(filepath.Join(alpha, name), string(data))
	}
	slug := filepath.Base(w.dir)
	w.rename(filepath.Join(alpha+"-old", "tasks", slug), filepath.Join(alpha, "tasks", slug))
	w.dir = filepath.Join(alpha, "tasks", slug)
}

// s1 is the review's S1: alpha swapped for a copy of beta, whose Large has no plan gate.
func (w *wcTask) s1() {
	beta := filepath.Join(w.h.gridHome, "projects", "beta")
	w.h.write(filepath.Join(beta, "project.md"), "---\nid: beta\n---\n")
	w.h.write(filepath.Join(beta, "workflows.md"), "# Workflows\n\n"+wcLarge(wcLargeOpen))
	w.rebuild(func(string) string { return beta }, "project.md", "workflows.md")
}

// s2 is the review's S2: alpha rebuilt with its project.md only, so the
// workspace Large, with no plan gate, is the one found.
func (w *wcTask) s2() { w.rebuild(func(old string) string { return old }, "project.md") }

func (w *wcTask) renamedBack() {
	alpha := filepath.Join(w.h.gridHome, "projects", "alpha")
	w.rename(alpha, alpha+"-old")
	w.rename(alpha+"-old", alpha)
}

func (w *wcTask) identical() {
	w.rebuild(func(old string) string { return old }, "project.md", "workflows.md")
}

// The L155T review's S1 and S2 on a task made with the default size, auto,
// through the CLI: the guard denies and `grid deliver --json` exits 1 with the
// coded envelope; a folder renamed back or rebuilt identically is no change.
func TestDeliverWorkflowChangedThroughCLI(t *testing.T) {
	for _, row := range []struct {
		name, size, step string
		swap             func(*wcTask)
		changed          bool
	}{
		{"size auto, S1", "", "plan", (*wcTask).s1, true},
		{"size auto, S2", "", "plan", (*wcTask).s2, true},
		{"size auto past plan, S2", "", "build", (*wcTask).s2, true},
		{"size large, S1", "large", "plan", (*wcTask).s1, true},
		{"size large past plan, S2", "large", "build", (*wcTask).s2, true},
		{"size auto, renamed away and back", "", "build", (*wcTask).renamedBack, false},
		{"size auto, rebuilt with identical files", "", "build", (*wcTask).identical, false},
	} {
		t.Run(row.name, func(t *testing.T) {
			w := newWCTask(t, row.size, row.step)
			if w.digest() == "" {
				t.Errorf("grid task new wrote no workflow_digest: %q", w.read("task.md"))
			}
			// Unsized, or at the plan gate, the task waits for a person already.
			if got, want := w.hook(), map[bool]string{true: "deny"}[row.size == "" || row.step == "plan"]; got != want {
				t.Errorf("before the swap: guard %q, want %q", got, want)
			}
			row.swap(w)
			before := w.read("task.md")
			if got := w.hook(); got != "deny" {
				t.Errorf("guard %q, want deny", got)
			}
			code, out := w.deliver()
			if code != 1 || !strings.Contains(w.h.errb.String(), "Deliver blocked: plan not approved") {
				t.Errorf("deliver --json exit %d, stderr %q; want 1 naming the plan", code, w.h.errb)
			}
			if coded := strings.Contains(out, `{"error":{"code":"workflow_changed","message":"Deliver blocked: plan not approved`); coded != row.changed {
				t.Errorf("deliver --json stdout %q; want code workflow_changed %v", out, row.changed)
			}
			if w.read("task.md") != before {
				t.Error("a refused deliver changed task.md")
			}
		})
	}
}

// A size-auto task carries the digest a task made at size large carries, so a
// person who sizes it later, with the project folder untouched, renamed back
// or rebuilt identically, releases it past the plan as before.
func TestSizedLaterKeepsWorkflowDigest(t *testing.T) {
	for _, row := range []struct {
		name string
		swap func(*wcTask)
	}{
		{"sized in place", func(*wcTask) {}},
		{"renamed away and back, then sized", (*wcTask).renamedBack},
		{"rebuilt with identical files, then sized", (*wcTask).identical},
	} {
		t.Run(row.name, func(t *testing.T) {
			w := newWCTask(t, "", "build")
			want := newWCTask(t, "large", "build").digest()
			if got := w.digest(); got == "" || got != want {
				t.Errorf("size auto workflow_digest %q, size large %q; want equal", got, want)
			}
			row.swap(w)
			if got := w.hook(); got != "deny" {
				t.Errorf("unsized: guard %q, want deny", got)
			}
			text := w.read("task.md")
			w.h.write(filepath.Join(w.dir, "task.md"), strings.Replace(text, "\nsize: auto\n", "\nsize: large\n", 1))
			if got := w.digest(); got != want {
				t.Errorf("sized: workflow_digest %q, want %q", got, want)
			}
			if got := w.hook(); got != "" {
				t.Errorf("sized past plan: guard %q, want allow", got)
			}
			if code, out := w.deliver(); code != 0 {
				t.Errorf("sized past plan: deliver --json exit %d %s %s", code, out, w.h.errb)
			}
		})
	}
}

// A person who rewrites the workflow line of a task stamped from Large as the
// same steps inline acts outside the digest: the inline list is not compared,
// so the task past its plan is released as before.
func TestInlineRewriteOfStampedTaskNotHeld(t *testing.T) {
	w := newWCTask(t, "large", "build")
	text := w.read("task.md")
	if w.digest() == "" || !strings.Contains(text, "\nworkflow: Large\n") {
		t.Fatalf("task.md from grid task new: %q", text)
	}
	w.h.write(filepath.Join(w.dir, "task.md"), strings.Replace(text, "\nworkflow: Large\n", "\n"+previewInline, 1))
	if got := w.hook(); got != "" {
		t.Errorf("inline past plan: guard %q, want allow", got)
	}
	if code, out := w.deliver(); code != 0 {
		t.Errorf("inline past plan: deliver --json exit %d %s %s", code, out, w.h.errb)
	}
}
