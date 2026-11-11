package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// jwDaRefusal is the state table's refusal of a move from waiting to waiting
// (task.NextState), which delivery returns for a task it may not deliver.
const jwDaRefusal = `cannot status task in current state "waiting" to "waiting"`

// jwDaPlan is the plan of deliveryFixture: one step whose proof delivery checks.
const jwDaPlan = "# Plan 1\n- [x] 1 Check result\n  proof: check\n  jobs: []\n"

// jwDaHarness is approveJobHarness with jwDaPlan and the proof of its step, so
// only the task's state can refuse a delivery. The task waits at its plan gate
// with an open plan Job and no approval.
func jwDaHarness(t *testing.T) (*harness, string, string) {
	t.Helper()
	h, path, _, _ := approveJobHarness(t)
	h.write(filepath.Join(path, "plan.md"), jwDaPlan)
	addDeliveryProof(t, h, path)
	return h, path, jwDaPlan
}

// jwDaApproved is jwDaHarness after grid approve of the current plan. The task
// still waits at its plan gate: grid approve keeps the task's state.
func jwDaApproved(t *testing.T) (*harness, string, string) {
	t.Helper()
	h, path, text := jwDaHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("approve: %d %s", code, h.errb.String())
	}
	jwDaRequireFront(t, path, "waiting", "")
	return h, path, text
}

// jwDaRequireFront fails unless task.md holds this state and step.
func jwDaRequireFront(t *testing.T, path, state, step string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := frontmatter.Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := doc.Data["step"].(string)
	if doc.Data["state"] != state || got != step {
		t.Fatalf("task.md has state %v step %v, want %s %q\n%s", doc.Data["state"], doc.Data["step"], state, step, b)
	}
}

// jwDaDeliver runs grid deliver in the task folder and fails unless it exits 0.
func jwDaDeliver(t *testing.T, h *harness, path string, args ...string) {
	t.Helper()
	h.errb.Reset()
	if code := h.run(path, append([]string{"deliver"}, args...)...); code != 0 {
		t.Fatalf("deliver %v: exit %d %s", args, code, h.errb.String())
	}
}

// jwDaRequireRefused runs grid deliver, then grid deliver --dry-run, and fails
// unless both exit 1 with the state table's refusal and task.md keeps its bytes.
func jwDaRequireRefused(t *testing.T, h *harness, path string) {
	t.Helper()
	taskPath := filepath.Join(path, "task.md")
	before, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"deliver", "Result again"}, {"deliver", "--dry-run", "Result again"}} {
		h.errb.Reset()
		if code := h.run(path, args...); code != 1 || !strings.Contains(h.errb.String(), jwDaRefusal) {
			t.Fatalf("%v: exit %d %s", args, code, h.errb.String())
		}
		after, err := os.ReadFile(taskPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatalf("%v changed task.md:\n%s\nwas:\n%s", args, after, before)
		}
	}
}

// Row 1: grid approve keeps the task waiting at its plan gate, and grid deliver
// then moves it to waiting at step ship.
func TestJwDaDeliverAfterApprove(t *testing.T) {
	h, path, _ := jwDaApproved(t)
	jwDaDeliver(t, h, path, "Result ready")
	jwDaRequireFront(t, path, "waiting", "ship")
	b, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil || !strings.Contains(string(b), "Result ready") {
		t.Fatalf("Outcome not written: %v\n%s", err, b)
	}
}

// Row 2: the dry run after approve passes the same check and writes nothing.
func TestJwDaDryRunAfterApprove(t *testing.T) {
	h, path, _ := jwDaApproved(t)
	before, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	jwDaDeliver(t, h, path, "--dry-run", "Result ready")
	after, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("dry run changed task.md: %v\n%s", err, after)
	}
}

// Row 3: a waiting task with no approval, with an approval of an older plan, or
// already delivered (step ship) with or without an approval, is refused as on
// main, and task.md keeps its bytes.
func TestJwDaRefusesWaitingWithoutCurrentApproval(t *testing.T) {
	t.Run("no approval", func(t *testing.T) {
		h, path, _ := jwDaHarness(t)
		jwDaRequireRefused(t, h, path)
	})
	t.Run("plan changed after approval", func(t *testing.T) {
		h, path, text := jwDaApproved(t)
		changed := text + "\nOne more step.\n"
		if plan.Hash(changed) == plan.Hash(text) {
			t.Fatal("the changed plan has the approved hash")
		}
		if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
		jwDaRequireRefused(t, h, path)
	})
	t.Run("delivered after approval", func(t *testing.T) {
		h, path, _ := jwDaApproved(t)
		jwDaDeliver(t, h, path, "Result ready")
		jwDaRequireFront(t, path, "waiting", "ship")
		jwDaRequireRefused(t, h, path)
	})
	t.Run("delivered without approval", func(t *testing.T) {
		h, path, _ := jwDaHarness(t)
		if code := h.run(path, "task", "status", "T-07", "running"); code != 0 {
			t.Fatalf("task status: %d %s", code, h.errb.String())
		}
		jwDaDeliver(t, h, path, "Result ready")
		jwDaRequireFront(t, path, "waiting", "ship")
		jwDaRequireRefused(t, h, path)
	})
}

// Row 4: the rig path (approve, task status running, deliver) delivers as before.
func TestJwDaRunningDeliversAsBefore(t *testing.T) {
	h, path, _ := jwDaApproved(t)
	if code := h.run(path, "task", "status", "T-07", "running"); code != 0 {
		t.Fatalf("task status: %d %s", code, h.errb.String())
	}
	jwDaRequireFront(t, path, "running", "")
	jwDaDeliver(t, h, path, "Result ready")
	jwDaRequireFront(t, path, "waiting", "ship")
}
