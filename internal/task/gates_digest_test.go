package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestApproveDigestReceipt(t *testing.T) {
	for _, gate := range []string{"plan"} {
		t.Run(gate, func(t *testing.T) {
			s, task := waitingFixture(t)
			hash := setPlan(t, task, planV1)
			digest := plan.Digest(planV1)
			event, err := approve(t, s, task, WriteOptions{ID: task.ID}, digest)
			if err != nil {
				t.Fatalf("current digest refused: %v", err)
			}
			if event.PlanHash != hash || event.PlanDigest != digest {
				t.Fatalf("approval hash = %q, digest = %q; want %q, %q", event.PlanHash, event.PlanDigest, hash, digest)
			}
			noReceiptFile(t, task.Path)
			found, err := planJobResult(task.Path)
			if err != nil || found.PlanHash != hash || found.PlanDigest != digest {
				t.Fatalf("the plan Job result differs: %+v (%v)", found, err)
			}
			if got, err := s.ApprovalForGate("", task.ID, gate); err != nil || !sameApproval(got, event) {
				t.Fatalf("the plan Job result cannot release %s gate: %v", gate, err)
			}
			history, err := s.History("", task.ID)
			if err != nil || len(history) == 0 || !strings.Contains(history[len(history)-1], `"plan_hash":"`+hash+`"`) {
				t.Fatalf("history must retain short hash: %v", err)
			}
		})
	}
}

func TestApproveDigestMismatch(t *testing.T) {
	for _, previous := range []bool{false, true} {
		t.Run(fmt.Sprint(previous), func(t *testing.T) {
			s, task := waitingFixture(t)
			setPlan(t, task, planV1)
			if previous {
				if _, err := approve(t, s, task, WriteOptions{ID: task.ID}, ""); err != nil {
					t.Fatal(err)
				}
			}
			before := treeSnapshot(t, task.Path)
			_, err := s.Approve(WriteOptions{ID: task.ID}, plan.Digest(planV2))
			assertDigestMismatch(t, err, planV1)
			after := treeSnapshot(t, task.Path)
			// A valid token may create the persistent plan coordination lock.
			delete(before, ".plan.lock")
			delete(after, ".plan.lock")
			if !reflect.DeepEqual(before, after) {
				t.Fatal("mismatched digest wrote task data")
			}
		})
	}
}

func assertDigestMismatch(t *testing.T, err error, current string) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), "plan hash no longer matches") || errors.Is(err, ErrEditUsage) {
		t.Fatalf("expected stale-plan refusal, got %v", err)
	}
	if !strings.Contains(err.Error(), plan.Hash(current)) || strings.Contains(err.Error(), plan.Digest(current)) {
		t.Fatalf("mismatch must show only the current short hash: %v", err)
	}
}

func TestApproveDigestCollision(t *testing.T) {
	// Birthday search over the real 24-bit hash; both plans keep the same heading.
	seen := make(map[string]string)
	var first, second string
	for i := 0; i < 100000; i++ {
		text := fmt.Sprintf("# Plan\n\nReview choice %d.\n", i)
		hash := plan.Hash(text)
		if old, ok := seen[hash]; ok {
			first, second = old, text
			break
		}
		seen[hash] = text
	}
	if first == "" || plan.Hash(first) != plan.Hash(second) || plan.Digest(first) == plan.Digest(second) {
		t.Fatal("could not construct a short-hash collision")
	}
	s, task := waitingFixture(t)
	setPlan(t, task, first)
	// The person reviews first; a writer substitutes second before approval.
	token := plan.Digest(first)
	setPlan(t, task, second)
	_, err := s.Approve(WriteOptions{ID: task.ID}, token)
	assertDigestMismatch(t, err, second)
	if _, err := planJobResult(task.Path); !errors.Is(err, ErrNoPlanJob) {
		t.Fatalf("collision wrote an approval: %v", err)
	}
	// Legacy compatibility deliberately still allows the same short token.
	if _, err := s.Approve(WriteOptions{ID: task.ID}, plan.Hash(first)); err != nil {
		t.Fatalf("legacy short token refused: %v", err)
	}
}

func TestApproveTokenValidationBeforeWrite(t *testing.T) {
	for _, token := range []string{"1234567", strings.Repeat("a", 63), "ABCDEF", strings.Repeat("A", 64), "abcdeg", strings.Repeat("a", 63) + "g", " abcde", "abcde\n"} {
		// No resolver or task: usage errors must precede even task resolution.
		if _, err := (Store{}).Approve(WriteOptions{}, token); !errors.Is(err, ErrEditUsage) {
			t.Errorf("token %q: expected usage error before write, got %v", token, err)
		}
	}
}

func TestApproveTokenPolicy(t *testing.T) {
	for _, policy := range []string{"", "any", "digest", "unknown"} {
		for _, token := range []string{"", plan.Hash(planV1), plan.Digest(planV1)} {
			t.Run(fmt.Sprintf("%s/length-%d", policy, len(token)), func(t *testing.T) {
				s, task := waitingFixture(t)
				setPlan(t, task, planV1)
				o := WriteOptions{ID: task.ID, PlanHashToken: policy}
				before := treeSnapshot(t, task.Path)
				_, err := approve(t, s, task, o, token)
				if policy == "unknown" || policy == "digest" && len(token) == 6 {
					if !errors.Is(err, ErrEditUsage) {
						t.Fatalf("policy %q: expected usage error, got %v", policy, err)
					}
					if !reflect.DeepEqual(before, treeSnapshot(t, task.Path)) {
						t.Fatal("policy refusal wrote task data")
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				// Empty tokens retain the old receipt rule, including digest-only policy.
				if _, err := s.Approve(o, ""); err != nil {
					t.Fatalf("unchanged implicit reapproval refused: %v", err)
				}
				setPlan(t, task, planV2)
				before = treeSnapshot(t, task.Path)
				if _, err := s.Approve(o, ""); err == nil || !strings.HasPrefix(err.Error(), "Changed since you approved") {
					t.Fatalf("changed implicit reapproval: %v", err)
				}
				if !reflect.DeepEqual(before, treeSnapshot(t, task.Path)) {
					t.Fatal("implicit refusal changed receipt or history")
				}
			})
		}
	}
}

func TestApproveDigestKeepsPlanLockForReceipt(t *testing.T) {
	s, task := waitingFixture(t)
	setPlan(t, task, planV1)
	lock, err := fslock.OpenNoFollow(filepath.Join(task.Path, ".plan.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	var lockErr error
	called := false
	s.Now = func() time.Time {
		called = true
		lockErr = fslock.TryLockExclusive(lock)
		if lockErr == nil {
			fslock.Unlock(lock)
		}
		return time.Now()
	}
	if _, err := s.Approve(WriteOptions{ID: task.ID}, plan.Digest(planV1)); err != nil {
		t.Fatal(err)
	}
	if !called || !errors.Is(lockErr, fslock.ErrLocked) {
		t.Fatalf("receipt constructed without holding plan lock: called=%v, lock=%v", called, lockErr)
	}
}

func TestApproveDigestUsesLockedSnapshot(t *testing.T) {
	for _, becomesCurrent := range []bool{false, true} {
		t.Run(fmt.Sprint(becomesCurrent), func(t *testing.T) {
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
			defer fslock.Unlock(lock)
			token := plan.Digest(planV1)
			if becomesCurrent {
				token = plan.Digest(planV2)
			}
			done := make(chan error, 1)
			go func() {
				_, err := s.Approve(WriteOptions{ID: task.ID}, token)
				done <- err
			}()
			select {
			case err := <-done:
				t.Fatalf("digest checked before plan lock was released: %v", err)
			case <-time.After(200 * time.Millisecond):
			}
			// Complete a cooperating write while approval waits on its plan lock.
			setPlan(t, task, planV2)
			fslock.Unlock(lock)
			select {
			case err := <-done:
				if becomesCurrent {
					if err != nil {
						t.Fatalf("locked current digest refused: %v", err)
					}
				} else {
					assertDigestMismatch(t, err, planV2)
					if _, err := planJobResult(task.Path); !errors.Is(err, ErrNoPlanJob) {
						t.Fatalf("stale snapshot wrote an approval: %v", err)
					}
				}
			case <-time.After(10 * time.Second):
				t.Fatal("approval did not resume after plan lock release")
			}
		})
	}
}
