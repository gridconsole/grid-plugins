package task

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

func setGate(t *testing.T, task Task, gate string) {
	t.Helper()
	path := filepath.Join(task.Path, "task.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, path, strings.Replace(string(data), "state: waiting", "state: waiting\ngate: "+gate, 1))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A task at gate preview gets its own receipt and history line; the plan
// receipt stays byte for byte as the person left it.
func TestApproveAtPreviewGateRecordsPreviewReceipt(t *testing.T) {
	s, task := waitingFixture(t)
	h1 := setPlan(t, task, planV1)
	planEvent, err := s.Approve(WriteOptions{ID: task.ID, Note: "Plan ok"}, "")
	if err != nil || planEvent.Gate != "" {
		t.Fatalf("%+v: %v", planEvent, err)
	}
	planBytes := readFile(t, filepath.Join(task.Path, "approval.json"))
	setGate(t, task, "preview")
	event, err := s.Approve(WriteOptions{ID: task.ID, Note: "Spec ok", Via: "grid-app"}, "")
	if err != nil || event.Gate != "preview" || event.PlanHash != h1 || event.PlanDigest != plan.Digest(planV1) || event.What != "approve" {
		t.Fatalf("%+v: %v", event, err)
	}
	var stored Event
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(task.Path, "approval-preview.json"))), &stored); err != nil || stored != event {
		t.Fatalf("%+v: %v", stored, err)
	}
	if got := readFile(t, filepath.Join(task.Path, "approval.json")); got != planBytes {
		t.Fatalf("preview approval changed the plan receipt:\n%s\n%s", got, planBytes)
	}
	if _, err := os.Stat(filepath.Join(task.Path, "approval-ship.json")); !os.IsNotExist(err) {
		t.Fatal("preview approval wrote a ship receipt", err)
	}
	if !Approved(task.Path) {
		t.Fatal("plan approval no longer valid")
	}
	history := readFile(t, filepath.Join(task.Path, "history.md"))
	if n := strings.Count(history, " · preview approved · "+h1+" · by local · via grid-app\n"); n != 1 {
		t.Fatalf("want one preview approved line, got %d:\n%s", n, history)
	}
	if strings.Contains(history, "plan accepted") || strings.Contains(history, "ship approved") {
		t.Fatalf("preview approval logged another gate:\n%s", history)
	}
	if got, err := s.ApprovalForGate("", task.ID, "preview"); err != nil || got != event {
		t.Fatalf("%+v: %v", got, err)
	}
	if got, err := s.ApprovalForGate("", task.ID, "plan"); err != nil || got != planEvent {
		t.Fatalf("%+v: %v", got, err)
	}
	if _, err := s.ApprovalForGate("", task.ID, "ship"); err == nil {
		t.Fatal("a preview approval released the ship gate")
	}
	// Reopening a task invalidates every receipt.
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "reopen"}); err != nil {
		t.Fatal(err)
	}
	var e Event
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(task.Path, "approval-preview.json"))), &e); err != nil || e.What != "reopen" {
		t.Fatalf("preview receipt survived reopen: %+v %v", e, err)
	}
}

// With no plan approval yet, a preview approval must not create one.
func TestPreviewApprovalWritesNoPlanReceipt(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	setGate(t, task, "preview")
	if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, "approval.json")); !os.IsNotExist(err) {
		t.Fatal("preview approval wrote approval.json", err)
	}
	if Approved(task.Path) {
		t.Fatal("a preview approval counted as the plan approval")
	}
}

// The preview gate follows the ship gate's rules for a repeat and a stale token.
func TestApproveGateRepeatAndStaleToken(t *testing.T) {
	for _, tc := range []struct{ gate, file string }{
		{"ship", "approval-ship.json"},
		{"preview", "approval-preview.json"},
	} {
		t.Run(tc.gate, func(t *testing.T) {
			s, task := waitingFixture(t)
			h1 := setPlan(t, task, planV1)
			if _, err := s.Approve(WriteOptions{ID: task.ID}, h1); err != nil {
				t.Fatal(err)
			}
			planBytes := readFile(t, filepath.Join(task.Path, "approval.json"))
			setGate(t, task, tc.gate)
			first, err := s.Approve(WriteOptions{ID: task.ID}, h1)
			if err != nil {
				t.Fatal(err)
			}
			again, err := s.Approve(WriteOptions{ID: task.ID}, h1)
			if err != nil || again.PlanHash != first.PlanHash || again.PlanDigest != first.PlanDigest || again.Gate != tc.gate {
				t.Fatalf("repeat with the same token: %+v: %v", again, err)
			}
			if got := readFile(t, filepath.Join(task.Path, "approval.json")); got != planBytes {
				t.Fatal("a repeat approval changed the plan receipt")
			}
			// Edit plan.md: the old token and a tokenless approve are both refused.
			h2 := setPlan(t, task, planV2)
			receipt := readFile(t, filepath.Join(task.Path, tc.file))
			history := readFile(t, filepath.Join(task.Path, "history.md"))
			if _, err := s.Approve(WriteOptions{ID: task.ID}, h1); err == nil || !strings.Contains(err.Error(), "plan hash no longer matches") {
				t.Fatalf("stale token: %v", err)
			}
			if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
				t.Fatalf("stale receipt with no token: %v", err)
			}
			if readFile(t, filepath.Join(task.Path, tc.file)) != receipt || readFile(t, filepath.Join(task.Path, "history.md")) != history {
				t.Fatal("a refused approval wrote")
			}
			if _, err := s.ApprovalForGate("", task.ID, tc.gate); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
				t.Fatalf("stale %s receipt accepted: %v", tc.gate, err)
			}
			if event, err := s.Approve(WriteOptions{ID: task.ID}, h2); err != nil || event.PlanHash != h2 {
				t.Fatalf("explicit reapproval: %+v: %v", event, err)
			}
		})
	}
}

// Only a waiting task can be approved at the preview gate.
func TestPreviewApprovalNeedsWaiting(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	setGate(t, task, "preview")
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: "running"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err == nil {
		t.Fatal("approved a running task at preview")
	}
	for _, name := range []string{"approval-preview.json", "approval.json"} {
		if _, err := os.Stat(filepath.Join(task.Path, name)); !os.IsNotExist(err) {
			t.Fatalf("%s written: %v", name, err)
		}
	}
}

// A crash between the journal and the renames must replay the preview receipt.
func TestRecoverWriteReplaysPreviewReceipt(t *testing.T) {
	_, task := waitingFixture(t)
	files := map[string][]byte{
		"approval-preview.json": []byte("{}\n"),
		"history.md":            []byte("- note\n"),
	}
	data, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(task.Path, journalName), string(data))
	if err := recoverWrite(task.Path); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(task.Path, "approval-preview.json")); got != "{}\n" {
		t.Fatalf("receipt not replayed: %q", got)
	}
}
