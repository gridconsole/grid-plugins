package task

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func waitingFixture(t *testing.T) (Store, Task) {
	t.Helper()
	s, task := writeFixture(t)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	return s, task
}

const planV1 = "# Plan, rev 1 \u00b7 000000\n\n## Where it stands\n\n- [ ] 1 Build it\n\n## How we approach it\n\nUsed: brain:project/conventions/example\n"
const planV2 = planV1 + "\nA new risk the person has not seen.\n"

func TestApprovalBindsStepMetadata(t *testing.T) {
	for _, field := range []string{"evidence", "needs"} {
		t.Run(field, func(t *testing.T) {
			s, task := waitingFixture(t)
			text := strings.Replace(planV1, "Build it\n", "Build it\n  "+field+": original\n", 1)
			hash := setPlan(t, task, text)
			event, err := s.Approve(WriteOptions{ID: task.ID}, hash)
			if err != nil || event.PlanHash != hash || event.PlanDigest != plan.Digest(text) || !Approved(task.Path) {
				t.Fatalf("metadata approval: %+v err=%v", event, err)
			}
			if _, err := s.Approval("", task.ID); err != nil {
				t.Fatal(err)
			}
			setPlan(t, task, strings.Replace(text, ": original", ": injected", 1))
			if _, err := s.Approval("", task.ID); err == nil || Approved(task.Path) {
				t.Fatal("changed metadata retained approval")
			}
			if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err == nil {
				t.Fatal("changed metadata allowed implicit reapproval")
			}
		})
	}
}

func setPlan(t *testing.T, task Task, text string) string {
	t.Helper()
	write(t, filepath.Join(task.Path, "plan.md"), text)
	return plan.Hash(text)
}

func TestApprovalTracksPlan(t *testing.T) {
	s, task := waitingFixture(t)
	o := WriteOptions{ID: task.ID, Note: "Reviewed"}
	if _, err := s.Approval("", task.ID); err == nil || !strings.Contains(err.Error(), "not been approved") {
		t.Fatalf("approval without a plan: %v", err)
	}
	if _, err := s.Approve(o, ""); err == nil || !strings.Contains(err.Error(), "plan.md is missing") {
		t.Fatalf("approved a task with no plan.md: %v", err)
	}
	h1 := setPlan(t, task, planV1)
	// A stored plan.hash is not a source of truth and is never read.
	write(t, filepath.Join(task.Path, "plan.hash"), "opaque:v1\n")
	event, err := s.Approve(o, "")
	if err != nil || event.PlanHash != h1 || event.PlanDigest != plan.Digest(planV1) || event.Note != "Reviewed" {
		t.Fatalf("%+v: %v", event, err)
	}
	got, err := s.Approval("", task.ID)
	if err != nil || got != event || !Approved(task.Path) {
		t.Fatalf("%+v: %v", got, err)
	}
	// Ticking a step is not a plan change.
	setPlan(t, task, strings.Replace(planV1, "[ ]", "[x]", 1))
	if _, err := s.Approval("", task.ID); err != nil || !Approved(task.Path) {
		t.Fatalf("tick lost the approval: %v", err)
	}
	before, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
	oldApproval, _ := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	// An agent edits plan.md after the approval, and may rewrite plan.hash to match.
	h2 := setPlan(t, task, planV2)
	write(t, filepath.Join(task.Path, "plan.hash"), h1+"\n")
	if _, err := s.Approval("", task.ID); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
		t.Fatalf("stale gate accepted: %v", err)
	}
	if Approved(task.Path) {
		t.Fatal("hook predicate accepted a changed plan")
	}
	for _, hash := range []string{h1, ""} {
		if _, err := s.Approve(o, hash); err == nil {
			t.Fatalf("approved stale hash %q", hash)
		}
	}
	after, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
	approval, _ := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	if !bytes.Equal(before, after) || !bytes.Equal(oldApproval, approval) {
		t.Fatal("refused approval wrote data")
	}
	if _, err := s.Approve(o, h2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approval("", task.ID); err != nil || !Approved(task.Path) {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(task.Path, "plan.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approval("", task.ID); err == nil || Approved(task.Path) {
		t.Fatal("approval survived a missing plan")
	}
	setPlan(t, task, planV2)
	for _, action := range []string{"close", "reopen"} {
		if _, err := s.Change(WriteOptions{ID: task.ID, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Approval("", task.ID); err == nil || Approved(task.Path) {
		t.Fatal("reopen reused an old approval")
	}
}

func TestApprovedRejectsAnotherGate(t *testing.T) {
	for _, gate := range []string{"ship", "unknown"} {
		t.Run(gate, func(t *testing.T) {
			s, task := waitingFixture(t)
			setPlan(t, task, planV1)
			event, err := s.Approve(WriteOptions{ID: task.ID}, "")
			if err != nil {
				t.Fatal(err)
			}
			event.Gate = gate
			data, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(task.Path, "approval.json"), string(data))
			if _, err := s.Approval("", task.ID); err == nil {
				t.Fatal("locked approval accepted another gate")
			}
			if Approved(task.Path) {
				t.Fatal("lock-free approval accepted another gate")
			}
		})
	}
}

// Approve and Approval take .plan.lock, so they wait for a plan write in flight.
func TestApprovalWaitsForPlanLock(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	lock, err := fslock.OpenNoFollow(filepath.Join(task.Path, ".plan.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err := fslock.LockExclusive(lock); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := s.Approve(WriteOptions{ID: task.ID}, "")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("approved while a plan write held .plan.lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	fslock.Unlock(lock)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("approve did not resume after .plan.lock was released")
	}
}

func TestApproveStateAndFiles(t *testing.T) {
	for _, state := range []string{"queued", "running", "closed", "waiting"} {
		t.Run(state, func(t *testing.T) {
			s, task := writeFixture(t)
			if state != "queued" {
				o := WriteOptions{ID: task.ID, Action: "status", State: state}
				if state == "closed" {
					o.Action = "close"
				}
				if _, err := s.Change(o); err != nil {
					t.Fatal(err)
				}
			}
			_, err := s.Approve(WriteOptions{ID: task.ID}, setPlan(t, task, planV1))
			if (err == nil) != (state == "waiting") {
				t.Fatalf("%s: %v", state, err)
			}
		})
	}
	for _, name := range []string{"plan.md", "approval.json"} {
		t.Run("symlink/"+name, func(t *testing.T) {
			s, task := waitingFixture(t)
			hash := setPlan(t, task, planV1)
			target := filepath.Join(filepath.Dir(task.Path), "outside")
			write(t, target, planV1)
			if name == "plan.md" {
				if err := os.Remove(filepath.Join(task.Path, name)); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join(task.Path, name)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Approve(WriteOptions{ID: task.ID}, hash); err == nil {
				t.Fatal("followed symlink")
			}
			if Approved(task.Path) {
				t.Fatal("hook predicate followed symlink")
			}
		})
	}
}

func TestRefusalDecisions(t *testing.T) {
	for _, decision := range []string{"allow", "deny"} {
		t.Run(decision, func(t *testing.T) {
			s, task := writeFixture(t)
			o := WriteOptions{ID: task.ID, Note: "One command only"}
			if _, err := s.Decide(o, "R-1", decision); err == nil {
				t.Fatal("decided a missing refusal")
			}
			write(t, filepath.Join(task.Path, "refusals", "R-1.json"), `{"id":"R-1","command":"echo example","rule":"Bash"}`)
			for _, id := range []string{"../R-1", "R-0", "R-1/other"} {
				if _, err := s.Decide(o, id, decision); err == nil {
					t.Fatal("accepted invalid id")
				}
			}
			if _, err := s.Decide(o, "R-1", "yes"); err == nil {
				t.Fatal("accepted invalid decision")
			}
			event, err := s.Decide(o, "R-1", decision)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(filepath.Join(task.Path, "refusal-R-1.json"))
			var got Event
			if err := json.Unmarshal(data, &got); err != nil || got != event || got.Decision != decision || got.FromState != got.ToState {
				t.Fatalf("%+v: %v", got, err)
			}
			if _, err := s.Decide(o, "R-1", decision); err == nil {
				t.Fatal("overwrote decision")
			}
			lines, err := s.History("", task.ID)
			if err != nil || len(lines) != 1 {
				t.Fatalf("%v: %v", lines, err)
			}
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(task.Path, "refusals", "R-2.json"), `{}`)
			if _, err := s.Decide(o, "R-2", decision); err == nil {
				t.Fatal("decided a closed task")
			}
		})
	}
}

func TestGateWithSymlinkedHome(t *testing.T) {
	s, task := waitingFixture(t)
	alias := filepath.Join(filepath.Dir(s.Resolver.GridHome), "grid-alias")
	if err := os.Symlink(s.Resolver.GridHome, alias); err != nil {
		t.Fatal(err)
	}
	s.Resolver.GridHome = alias
	if _, err := s.Approve(WriteOptions{ID: task.ID}, setPlan(t, task, planV1)); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(task.Path, "refusals", "R-1.json"), `{}`)
	if _, err := s.Decide(WriteOptions{ID: task.ID}, "R-1", "deny"); err != nil {
		t.Fatal(err)
	}
}

// The plan receipt is the only receipt (S6c3). Reopening a task overwrites it
// with the reopen event, so the plan is approved again.
func TestReopenOverwritesPlanReceipt(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	planEvent, err := s.Approve(WriteOptions{ID: task.ID, Note: "Plan ok"}, "")
	if err != nil || planEvent.Gate != "" {
		t.Fatalf("%+v: %v", planEvent, err)
	}
	b, _ := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	var stored Event
	if err := json.Unmarshal(b, &stored); err != nil || stored != planEvent {
		t.Fatalf("%+v: %v", stored, err)
	}
	if !Approved(task.Path) {
		t.Fatal("plan approval not valid")
	}
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "reopen"}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(task.Path, "approval.json"))
	var e Event
	if err := json.Unmarshal(b, &e); err != nil || e.What != "reopen" {
		t.Fatalf("approval.json survived reopen: %+v %v", e, err)
	}
}

func TestApprovalDigestCompatibility(t *testing.T) {
	// The old canonical digest omitted the revision and status heading.
	legacyDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("- [ ] 1 Build it\n## How we approach it\n\nUsed: brain:project/conventions/example\n")))
	for _, tc := range []struct {
		name, digest string
		want         bool
	}{
		{"legacy short hash", "", false},
		{"legacy canonical digest", `,"plan_digest":"` + legacyDigest + `"`, false},
		{"full digest", `,"plan_digest":"` + plan.Digest(planV1) + `"`, true},
		{"wrong digest", `,"plan_digest":"` + strings.Repeat("0", 64) + `"`, false},
		{"empty digest", `,"plan_digest":""`, false},
		{"null digest", `,"plan_digest":null`, false},
		{"non-string digest", `,"plan_digest":123`, false},
		{"short digest", `,"plan_digest":"abcdef"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := waitingFixture(t)
			hash := setPlan(t, task, planV1)
			write(t, filepath.Join(task.Path, "approval.json"), `{"task":"`+task.ID+`","what":"approve","plan_hash":"`+hash+`"`+tc.digest+`}`)
			_, err := s.Approval("", task.ID)
			if (err == nil) != tc.want || Approved(task.Path) != tc.want || ApprovedDigest(task.Path, hash, plan.Digest(planV1)) != tc.want {
				t.Fatalf("approval allowed = %t, want %t: %v", err == nil, tc.want, err)
			}
			if tc.want || strings.HasPrefix(tc.name, "legacy ") {
				if !tc.want {
					if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err == nil {
						t.Fatal("legacy receipt upgraded without explicit reapproval")
					}
				}
				event, err := s.Approve(WriteOptions{ID: task.ID}, hash)
				if err != nil || event.PlanDigest != plan.Digest(planV1) {
					t.Fatalf("reapproval must upgrade legacy receipts: %+v %v", event, err)
				}
			}
		})
	}
}
