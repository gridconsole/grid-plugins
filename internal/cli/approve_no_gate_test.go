package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// grid approve takes no --gate (S6c4): a task has one approval, of its plan.
// The old flag is an unknown flag (exit 2) and writes nothing; a plain approve
// still records the plan hash and digest.
func TestApproveGateFlagIsUnknown(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	fixture := taskFixture(t, h)
	if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: fixture.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(fixture.Path, "plan.md"), approvedPlan)
	for _, gate := range []string{"ship", "preview", "plan"} {
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(h.home, "approve", "T-01", "--gate", gate); code != 2 {
			t.Fatalf("--gate %s: exit %d, want 2: %s", gate, code, h.errb)
		}
		if !strings.Contains(h.errb.String(), "gate") {
			t.Fatalf("--gate %s: stderr does not name the flag: %s", gate, h.errb)
		}
		if _, err := os.Stat(filepath.Join(fixture.Path, "approval.json")); err == nil {
			t.Fatalf("--gate %s wrote an approval", gate)
		}
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(h.home, "approve", "T-01", "--json"); code != 0 {
		t.Fatalf("plain approve: exit %d: %s", code, h.errb)
	}
	var got map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["gate"]; ok {
		t.Fatalf("approve output carries a gate: %s", h.out)
	}
	if got["plan_hash"] != plan.Hash(approvedPlan) || got["plan_digest"] != plan.Digest(approvedPlan) {
		t.Fatalf("approve output does not carry the plan hash and digest: %s", h.out)
	}
	data, err := os.ReadFile(filepath.Join(fixture.Path, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt task.Event
	if err := json.Unmarshal(data, &receipt); err != nil || receipt.PlanDigest != plan.Digest(approvedPlan) || receipt.Gate != "" {
		t.Fatalf("receipt %s: %v", data, err)
	}
	if !task.Approved(fixture.Path) {
		t.Fatal("plain approve did not approve the plan")
	}
}

// The refusal verbs are gone with the shell guard that raised refusals (S6c4).
func TestRefusalVerbsAreUnknown(t *testing.T) {
	for _, args := range [][]string{
		{"refusal", "decide", "T-01", "R-1", "allow"},
		{"decide", "T-01", "R-1", "allow"},
	} {
		h := newHarness(t)
		if code := h.run(h.home, args...); code != 2 {
			t.Fatalf("%v: exit %d, want 2: %s", args, code, h.errb)
		}
	}
}
