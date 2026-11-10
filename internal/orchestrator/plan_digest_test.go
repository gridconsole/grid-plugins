package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/guard"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestPlanPublicationRejectsChangedStepMetadata(t *testing.T) {
	for _, section := range []string{"## Steps", "## Where it stands"} {
		for _, field := range []string{"evidence", "needs"} {
			t.Run(section+"/"+field, func(t *testing.T) {
				f := setupPlan(t)
				original := strings.Replace(plannedText(t), "- [ ] 1", section+"\n\n- [ ] 1", 1)
				original = strings.Replace(original, "  kind:", "  "+field+": original\n  kind:", 1)
				writePlan(t, f, original)
				changed := strings.Replace(original, field+": original", field+": injected", 1)
				if plan.Digest(original) == plan.Digest(changed) || plan.Hash(original) != plan.Hash(changed) {
					t.Fatal("metadata must change the digest while preserving the legacy short hash")
				}
				anchor, err := plan.Anchor(changed, "step-1")
				if err != nil || !strings.Contains(anchor, field+": injected") {
					t.Fatalf("injected metadata must reach worker anchor: %q err=%v", anchor, err)
				}
				writePlan(t, f, changed)
				if out := f.tick(t); out.Assigned != 1 {
					t.Fatalf("changed metadata did not publish: %+v", out)
				}
			})
		}
	}
}

// Brute force the actual 24-bit preimage, hashing already canonical bytes in the
// hot loop. Both final plans are checked through the production canonicalizer.
func forgedPlan(t *testing.T, original string) (string, string) {
	t.Helper()
	_, body, _ := strings.Cut(original, "\n")
	body = strings.Trim(body, "\n") + "\n"
	sum := sha256.Sum256([]byte(body))
	digest := hex.EncodeToString(sum[:])
	if plan.Hash(original) != digest[:6] {
		t.Fatal("collision fixture must already be canonical")
	}
	prefix := []byte(strings.Replace(body, "role: writer", "role: builder", 1) + "\nVariant: ")
	buf := make([]byte, len(prefix), len(prefix)+32)
	copy(buf, prefix)
	for nonce := uint64(0); nonce < 1<<32; nonce++ {
		candidate := strconv.AppendUint(buf[:len(prefix)], nonce, 10)
		candidate = append(candidate, '\n')
		got := sha256.Sum256(candidate)
		if got[0] == sum[0] && got[1] == sum[1] && got[2] == sum[2] && got != sum {
			forged := "# Plan, rev 1 · " + digest[:6] + "\n" + string(candidate)
			if plan.Hash(forged) != plan.Hash(original) {
				t.Fatal("brute force did not collide through plan.Hash")
			}
			t.Logf("found 24-bit collision after %d candidates", nonce+1)
			return forged, plan.Digest(original)
		}
	}
	t.Fatal("exhausted collision search")
	return "", ""
}

func TestForgedPlanDigest(t *testing.T) {
	f := setupPlan(t)
	root := t.TempDir()
	home := filepath.Join(root, "grid")
	dir := filepath.Join(home, "projects", "example", "tasks", "T-01-example")
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f.s.Feed.TaskDir, dir); err != nil {
		t.Fatal(err)
	}
	f.s.Feed.TaskDir = dir
	original := plannedText(t)
	forged, digest := forgedPlan(t, original)
	writePlan(t, f, original)
	store := task.Store{Resolver: where.Resolver{GridHome: home, Ceiling: root}}
	if _, err := store.Approve(task.WriteOptions{ID: "T-01"}, ""); err != nil {
		t.Fatal(err)
	}
	// Supply the new field explicitly so this regression also runs on the base.
	receipt := fmt.Sprintf(`{"task":"T-01","what":"approve","plan_hash":%q,"plan_digest":%q}`, plan.Hash(original), digest)
	if err := os.WriteFile(filepath.Join(dir, "approval.json"), []byte(receipt), 0600); err != nil {
		t.Fatal(err)
	}
	in := hookio.Input{Cwd: dir, ToolName: "Write", ToolInput: map[string]any{"file_path": filepath.Join(root, "result.go"), "content": "changed"}}
	env := map[string]string{"GRID_HOME": home}
	if got := guard.EvaluatePlan(in, root, env); got.Behavior != "" {
		t.Fatalf("original approved plan refused: %+v", got)
	}
	taskText, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	lookup := task.StepsFor(store.Resolver, dir)
	if !task.Approved(dir) || task.PlanHeldWith(lookup, dir, string(taskText), "") {
		t.Fatal("the original approved plan does not read as approved")
	}
	writePlan(t, f, forged)
	if _, err := store.Approval("", "T-01"); err == nil {
		t.Error("locked approval accepted a forged plan with a matching short hash")
	}
	// The plan hold is gone: no guard refuses the write. Whether the plan is
	// approved is task-level, and a forged plan must still read as not approved.
	if task.Approved(dir) {
		t.Error("Approved accepted a forged plan with a matching short hash")
	}
	if !task.PlanHeldWith(lookup, dir, string(taskText), "") {
		t.Error("PlanHeldWith released a forged plan with a matching short hash")
	}
	if got := guard.EvaluatePlan(in, root, env); got.Behavior != "" {
		t.Errorf("a plan that is not approved must not block a repo write: %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "history.md"), []byte("- 2026-01-02T03:04 · plan accepted · rev 1 · "+plan.Hash(original)+" · by person\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if task.Approved(dir) || !task.PlanHeldWith(lookup, dir, string(taskText), "- 2026-01-02T03:04 · plan accepted · rev 1 · "+plan.Hash(original)+" · by person\n") {
		t.Error("legacy history bypassed a mismatching full digest")
	}
	if _, err := store.Approve(task.WriteOptions{ID: "T-01"}, ""); err == nil {
		t.Error("implicit reapproval accepted a forged plan with a matching short hash")
	}
	// The person may explicitly reapprove after reviewing the changed plan.
	if _, err := store.Approve(task.WriteOptions{ID: "T-01"}, plan.Hash(forged)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	var renewed map[string]any
	if err := json.Unmarshal(data, &renewed); err != nil {
		t.Fatal(err)
	}
	if value, ok := renewed["plan_digest"].(string); !ok || len(value) != 64 || value == digest {
		t.Error("reapproval did not record the changed full digest")
	}
}
