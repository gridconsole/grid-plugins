package task

import (
	"bytes"
	"crypto/sha256"
	"errors"
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
			event, err := approve(t, s, task, WriteOptions{ID: task.ID}, hash)
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
	event, err := approve(t, s, task, o, "")
	if err != nil || event.PlanHash != h1 || event.PlanDigest != plan.Digest(planV1) || event.Note != "Reviewed" {
		t.Fatalf("%+v: %v", event, err)
	}
	got, err := s.Approval("", task.ID)
	if err != nil || !sameApproval(got, event) || !Approved(task.Path) {
		t.Fatalf("%+v: %v", got, err)
	}
	// Ticking a step is not a plan change.
	setPlan(t, task, strings.Replace(planV1, "[ ]", "[x]", 1))
	if _, err := s.Approval("", task.ID); err != nil || !Approved(task.Path) {
		t.Fatalf("tick lost the approval: %v", err)
	}
	before, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
	oldResult, _ := planJobResult(task.Path)
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
	result, _ := planJobResult(task.Path)
	if !bytes.Equal(before, after) || oldResult != result {
		t.Fatal("refused approval wrote data")
	}
	noReceiptFile(t, task.Path)
	if _, err := approve(t, s, task, o, h2); err != nil {
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
	// grid task reopen posts the new plan Job that continues the old one; the old
	// result stops approving the plan with it (planJobResult).
	postReopenJob(t, task.Path)
	if _, err := s.Approval("", task.ID); err == nil || Approved(task.Path) {
		t.Fatal("reopen reused an old approval")
	}
}

// Nothing in the task folder but the plan Job's result approves the plan: a file
// named like an old receipt, for the plan or for another gate, authorizes
// nothing, and a task with no plan Job is not approved. This replaces
// TestApprovedRejectsAnotherGate, whose receipt files are no longer read.
func TestAnApprovalFileApprovesNothing(t *testing.T) {
	for _, gate := range []string{"", "ship", "unknown"} {
		t.Run("gate="+gate, func(t *testing.T) {
			s, task := waitingFixture(t)
			setPlan(t, task, planV1)
			receipt := `{"task":"` + task.ID + `","what":"approve","plan_hash":"` + plan.Hash(planV1) + `","plan_digest":"` + plan.Digest(planV1) + `"`
			if gate != "" {
				receipt += `,"gate":"` + gate + `"`
			}
			for _, name := range []string{"approval.json", "approval-ship.json", "approval-preview.json"} {
				write(t, filepath.Join(task.Path, name), receipt+"}")
			}
			if _, err := s.Approval("", task.ID); !errors.Is(err, ErrNoPlanJob) {
				t.Fatalf("a task with no plan Job and an approval file: %v", err)
			}
			if _, err := s.ApprovalForGate("", task.ID, "ship"); !errors.Is(err, ErrNoPlanJob) {
				t.Fatalf("another gate: %v", err)
			}
			if Approved(task.Path) || ApprovedDigest(task.Path, plan.Hash(planV1), plan.Digest(planV1)) {
				t.Fatal("lock-free approval read a file")
			}
			// A plan Job nobody answered approves nothing either.
			openJob(t, task.Path)
			if _, err := s.Approval("", task.ID); !errors.Is(err, ErrPlanJobOpen) || Approved(task.Path) {
				t.Fatalf("an open plan Job with an approval file: %v", err)
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
	t.Run("symlink/plan.md", func(t *testing.T) {
		s, task := waitingFixture(t)
		hash := setPlan(t, task, planV1)
		target := filepath.Join(filepath.Dir(task.Path), "outside")
		write(t, target, planV1)
		if err := os.Remove(filepath.Join(task.Path, "plan.md")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(task.Path, "plan.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Approve(WriteOptions{ID: task.ID}, hash); err == nil {
			t.Fatal("followed symlink")
		}
		if Approved(task.Path) {
			t.Fatal("hook predicate followed symlink")
		}
	})
	// An old receipt name that is a symlink is neither read nor written any more:
	// Approve does not follow it, and it approves nothing.
	t.Run("symlink/old receipt", func(t *testing.T) {
		s, task := waitingFixture(t)
		hash := setPlan(t, task, planV1)
		target := filepath.Join(filepath.Dir(task.Path), "outside")
		write(t, target, planV1)
		link := filepath.Join(task.Path, "approval.json")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Approve(WriteOptions{ID: task.ID}, hash); err != nil {
			t.Fatal(err)
		}
		if Approved(task.Path) {
			t.Fatal("a symlink approved the plan")
		}
		if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("the symlink was replaced: %v", err)
		}
		if b, _ := os.ReadFile(target); string(b) != planV1 {
			t.Fatal("Approve wrote through the symlink")
		}
	})
}

// Approve works through a symlinked Grid home and records the plan digest.
func TestApproveWithSymlinkedHome(t *testing.T) {
	s, task := waitingFixture(t)
	alias := filepath.Join(filepath.Dir(s.Resolver.GridHome), "grid-alias")
	if err := os.Symlink(s.Resolver.GridHome, alias); err != nil {
		t.Fatal(err)
	}
	s.Resolver.GridHome = alias
	hash := setPlan(t, task, planV1)
	event, err := approve(t, s, task, WriteOptions{ID: task.ID}, hash)
	if err != nil || event.Gate != "" || event.PlanHash != hash || event.PlanDigest != plan.Digest(planV1) {
		t.Fatalf("%+v: %v", event, err)
	}
	if !Approved(task.Path) {
		t.Fatal("approval through a symlinked home not valid")
	}
}

// The plan Job's result is the only approval record and no receipt file is
// written (S6c3, verdict 12). Reopening a task writes no file either; the new plan
// Job that grid task reopen posts supersedes the old result, so the plan is
// approved again.
func TestReopenWritesNoReceiptAndTheNewPlanJobSupersedes(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	planEvent, err := approve(t, s, task, WriteOptions{ID: task.ID, Note: "Plan ok"}, "")
	if err != nil || planEvent.Gate != "" {
		t.Fatalf("%+v: %v", planEvent, err)
	}
	noReceiptFile(t, task.Path)
	if found, err := planJobResult(task.Path); err != nil || found.Note != "Plan ok" || found.PlanHash != planEvent.PlanHash {
		t.Fatalf("%+v: %v", found, err)
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
	noReceiptFile(t, task.Path)
	postReopenJob(t, task.Path)
	if Approved(task.Path) {
		t.Fatal("reopen kept the plan approved")
	}
}

func TestApprovalDigestCompatibility(t *testing.T) {
	// The old canonical digest omitted the revision and status heading.
	legacyDigest := fmt.Sprintf("%x", sha256.Sum256([]byte("- [ ] 1 Build it\n## How we approach it\n\nUsed: brain:project/conventions/example\n")))
	// The cases are results of the plan Job; a result with no plan_digest is not
	// an approval at all, so there is nothing to replace and Approve needs no token.
	for _, tc := range []struct {
		name          string
		fields        func(hash string) map[string]any
		want, refused bool // approved now; an implicit Approve is refused
	}{
		{"result without a digest", func(h string) map[string]any { return map[string]any{"plan_hash": h} }, false, false},
		{"legacy canonical digest", func(h string) map[string]any { return map[string]any{"plan_hash": h, "plan_digest": legacyDigest} }, false, true},
		{"full digest", func(h string) map[string]any {
			return map[string]any{"plan_hash": h, "plan_digest": plan.Digest(planV1)}
		}, true, false},
		{"wrong digest", func(h string) map[string]any {
			return map[string]any{"plan_hash": h, "plan_digest": strings.Repeat("0", 64)}
		}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := waitingFixture(t)
			hash := setPlan(t, task, planV1)
			postResult(t, task.Path, openJob(t, task.Path), tc.fields(hash))
			_, err := s.Approval("", task.ID)
			if (err == nil) != tc.want || Approved(task.Path) != tc.want || ApprovedDigest(task.Path, hash, plan.Digest(planV1)) != tc.want {
				t.Fatalf("approval allowed = %t, want %t: %v", err == nil, tc.want, err)
			}
			if !tc.want {
				_, err := s.Approve(WriteOptions{ID: task.ID}, "")
				if (err != nil) != tc.refused {
					t.Fatalf("implicit approve: refused = %t, want %t: %v", err != nil, tc.refused, err)
				}
			}
			event, err := approve(t, s, task, WriteOptions{ID: task.ID}, hash)
			if err != nil || event.PlanDigest != plan.Digest(planV1) || !Approved(task.Path) {
				t.Fatalf("explicit reapproval: %+v %v", event, err)
			}
		})
	}
}
