package cli

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/task"
)

// approveGateFixture is a waiting Personal task with a plan and no approval.
func approveGateFixture(t *testing.T) (*harness, string, string) {
	t.Helper()
	h := newHarness(t)
	created, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: h.home, Project: "personal", Words: "Approve the gate"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(created.Path, "plan.md"), "# Plan 1\n- [ ] 1 Build it\n  proof: check\n  jobs: []\n")
	if code := h.run(created.Path, "task", "status", created.ID, "waiting"); code != 0 {
		t.Fatal(h.errb)
	}
	return h, created.Path, created.ID
}

func approveEvent(t *testing.T, h *harness) task.Event {
	t.Helper()
	var event task.Event
	if err := json.Unmarshal(h.out.Bytes(), &event); err != nil || event.What != "approve" {
		t.Fatalf("%+v %v: %s", event, err, h.out)
	}
	return event
}

// refuseOtherGates presses approve for each gate in wrong and expects the
// refusal the decision words, with no file changed.
func refuseOtherGates(t *testing.T, h *harness, dir, id, at string, wrong ...string) {
	t.Helper()
	before := previewFiles(t, dir)
	for _, want := range wrong {
		code := h.run(dir, "approve", id, "--gate", want, "--json")
		if code != 1 || !strings.Contains(h.errb.String(), "Changed since you approved: the task is at gate "+at+", not "+want) {
			t.Fatalf("--gate %s at gate %s: exit %d: %s", want, at, code, h.errb)
		}
		if after := previewFiles(t, dir); !maps.Equal(after, before) {
			t.Fatalf("--gate %s at gate %s wrote:\n%v\n%v", want, at, before, after)
		}
	}
}

// A press on the plan card that reaches the plugin after deliver raised the
// ship gate is refused, not recorded as a ship approval.
func TestApproveGateAfterDeliveryRaisedShip(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	if code := h.run(dir, "deliver", "Ready"); code != 0 {
		t.Fatal(h.errb)
	}
	refuseOtherGates(t, h, dir, "T-01", "ship", "plan", "preview")
	planBytes := previewFiles(t, dir)["approval.json"]
	if code := h.run(dir, "approve", "T-01", "--gate", "ship", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	if event := approveEvent(t, h); event.Gate != "ship" {
		t.Fatalf("%+v", event)
	}
	if after := previewFiles(t, dir); after["approval.json"] != planBytes || after["approval-ship.json"] == "" {
		t.Fatal("--gate ship did not write the ship receipt alone")
	}
}

func TestApproveGateAtPlan(t *testing.T) {
	h, dir, id := approveGateFixture(t)
	refuseOtherGates(t, h, dir, id, "plan", "ship", "preview")
	if code := h.run(dir, "approve", id, "--gate", "plan", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	if event := approveEvent(t, h); event.Gate != "" || !task.Approved(dir) {
		t.Fatalf("%+v", event)
	}
}

func TestApproveGateAtPreview(t *testing.T) {
	h, dir, id := previewFixture(t)
	if code := h.run(dir, "preview", "--task", id); code != 0 {
		t.Fatal(h.errb)
	}
	refuseOtherGates(t, h, dir, id, "preview", "plan", "ship")
	planBytes := previewFiles(t, dir)["approval.json"]
	if code := h.run(dir, "approve", id, "--gate", "preview", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	if event := approveEvent(t, h); event.Gate != "preview" {
		t.Fatalf("%+v", event)
	}
	after := previewFiles(t, dir)
	if after["approval.json"] != planBytes || after["approval-preview.json"] == "" || strings.Count(after["history.md"], " · preview approved · ") != 1 {
		t.Fatal("--gate preview did not record the preview approval alone")
	}
}

// An unknown gate is a usage error: exit 2, nothing written, no lock taken.
func TestApproveUnknownGate(t *testing.T) {
	h, dir, id := approveGateFixture(t)
	locks := []string{filepath.Join(dir, ".task.lock"), filepath.Join(dir, ".plan.lock")}
	for _, lock := range locks {
		if err := os.Remove(lock); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	before := previewFiles(t, dir)
	if code := h.run(dir, "approve", id, "--gate", "bogus"); code != 2 || !strings.Contains(h.errb.String(), "gate must be plan, preview or ship") {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if after := previewFiles(t, dir); !maps.Equal(after, before) {
		t.Fatal("an unknown gate wrote")
	}
	for _, lock := range locks {
		if _, err := os.Lstat(lock); !os.IsNotExist(err) {
			t.Fatalf("%s created: %v", filepath.Base(lock), err)
		}
	}
}

// Agents and people run the same verbs: an agent environment approves, and the
// approval names the plan digest (simplify.md row Hard enforcement around those gates).
func TestApproveGateAgentApproves(t *testing.T) {
	h, dir, id := approveGateFixture(t)
	agent := map[string]string{"GRID_AGENT": "builder", "GRID_LEASE": "J-1", "GRID_ROLE": "worker"}
	h.env.Getenv = func(key string) string { return agent[key] }
	if code := h.run(dir, "approve", id, "--gate", "plan", "--json"); code != 0 {
		t.Fatalf("agent approve: exit %d: %s", code, h.errb)
	}
	event := approveEvent(t, h)
	if event.PlanHash == "" || !task.Approved(dir) {
		t.Fatalf("approval does not name the plan digest: %+v", event)
	}
}

// Pressing approve again on a preview already approved writes nothing: one
// preview approved line, and the receipt is not rewritten.
func TestApproveRepeatPreviewWritesNothing(t *testing.T) {
	h, dir, id := previewFixture(t)
	if code := h.run(dir, "preview", "--task", id); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(dir, "approve", id, "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	first := approveEvent(t, h)
	receipt := filepath.Join(dir, "approval-preview.json")
	old := time.Date(2001, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(receipt, old, old); err != nil {
		t.Fatal(err)
	}
	before := previewFiles(t, dir)
	for _, extra := range [][]string{nil, {"--plan-hash", first.PlanHash}, {"--gate", "preview"}} {
		args := append([]string{"approve", id, "--json"}, extra...)
		if code := h.run(dir, args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
		after := previewFiles(t, dir)
		if n := strings.Count(after["history.md"], " · preview approved · "); n != 1 || !maps.Equal(after, before) {
			t.Fatalf("%v wrote: %d preview approved lines\n%s", args, n, after["history.md"])
		}
		if again := approveEvent(t, h); again != first {
			t.Fatalf("%v returned %+v, want the stored %+v", args, again, first)
		}
	}
	if info, err := os.Stat(receipt); err != nil {
		t.Fatal(err)
	} else if !info.ModTime().Equal(old) {
		t.Fatalf("receipt rewritten at %v", info.ModTime())
	}
}
