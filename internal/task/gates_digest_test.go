package task

import (
	"encoding/json"
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
	for _, gate := range []string{"plan", "ship"} {
		t.Run(gate, func(t *testing.T) {
			s, task := waitingFixture(t)
			name := approvalFile
			if gate == "ship" {
				path := filepath.Join(task.Path, "task.md")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				write(t, path, strings.Replace(string(data), "state: waiting", "state: waiting\ngate: ship", 1))
				name = shipApprovalFile
			}
			hash := setPlan(t, task, planV1)
			digest := plan.Digest(planV1)
			event, err := s.Approve(WriteOptions{ID: task.ID}, digest)
			if err != nil {
				t.Fatalf("current digest refused: %v", err)
			}
			if event.PlanHash != hash || event.PlanDigest != digest {
				t.Fatalf("receipt hash = %q, digest = %q; want %q, %q", event.PlanHash, event.PlanDigest, hash, digest)
			}
			data, err := os.ReadFile(filepath.Join(task.Path, name))
			var receipt Event
			if err != nil || json.Unmarshal(data, &receipt) != nil || receipt != event {
				t.Fatalf("stored receipt differs: %s (%v)", data, err)
			}
			if got, err := s.ApprovalForGate("", task.ID, gate); err != nil || got != event {
				t.Fatalf("receipt cannot release %s gate: %v", gate, err)
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
				if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err != nil {
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
	if _, err := os.Stat(filepath.Join(task.Path, approvalFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("collision wrote an approval receipt")
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
					if _, err := os.Stat(filepath.Join(task.Path, approvalFile)); !errors.Is(err, os.ErrNotExist) {
						t.Fatal("stale snapshot wrote an approval receipt")
					}
				}
			case <-time.After(10 * time.Second):
				t.Fatal("approval did not resume after plan lock release")
			}
		})
	}
}
