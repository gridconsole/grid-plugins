package task

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A press names the gate the person saw; a task now waiting at another gate
// refuses it and writes nothing.
func TestApproveRefusesAnotherGate(t *testing.T) {
	for _, tc := range []struct{ gate, at, want string }{
		{"ship", "ship", "plan"},
		{"plan", "plan", "ship"},
		{"ship", "ship", "preview"},
		{"preview", "preview", "plan"},
		{"preview", "preview", "ship"},
		{"plan", "plan", "preview"},
		// Any other gate value is the plan gate.
		{"review", "plan", "ship"},
	} {
		t.Run(tc.gate+"/"+tc.want, func(t *testing.T) {
			s, task := waitingFixture(t)
			setPlan(t, task, planV1)
			if tc.gate != "plan" {
				if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err != nil {
					t.Fatal(err)
				}
				setGate(t, task, tc.gate)
			}
			before := approvalSnapshot(t, task.Path)
			_, err := s.Approve(WriteOptions{ID: task.ID, Gate: tc.want}, "")
			if err == nil || errors.Is(err, ErrEditUsage) || err.Error() != "Changed since you approved: the task is at gate "+tc.at+", not "+tc.want {
				t.Fatalf("gate %s at %s: %v", tc.want, tc.gate, err)
			}
			if after := approvalSnapshot(t, task.Path); !maps.Equal(after, before) {
				t.Fatalf("a refused approval wrote:\n%v\n%v", before, after)
			}
		})
	}
}

// A press that names the gate the task waits at records as it did without it.
func TestApproveMatchingGateWritesItsReceipt(t *testing.T) {
	for _, tc := range []struct{ at, file, eventGate string }{
		{"plan", approvalFile, ""},
		{"preview", previewApprovalFile, "preview"},
		{"ship", shipApprovalFile, "ship"},
	} {
		t.Run(tc.at, func(t *testing.T) {
			s, task := waitingFixture(t)
			h := setPlan(t, task, planV1)
			if tc.at != "plan" {
				setGate(t, task, tc.at)
			}
			event, err := s.Approve(WriteOptions{ID: task.ID, Gate: tc.at}, h)
			if err != nil || event.Gate != tc.eventGate || event.PlanHash != h || event.What != "approve" {
				t.Fatalf("%+v: %v", event, err)
			}
			var stored Event
			if err := json.Unmarshal([]byte(readFile(t, filepath.Join(task.Path, tc.file))), &stored); err != nil || stored != event {
				t.Fatalf("%+v: %v", stored, err)
			}
			n := strings.Count(readFile(t, filepath.Join(task.Path, "history.md")), " · preview approved · ")
			if (tc.at == "preview") != (n == 1) || n > 1 {
				t.Fatalf("gate %s: %d preview approved lines", tc.at, n)
			}
		})
	}
}

// An unknown gate is a usage error found before any lock is taken.
func TestApproveUnknownGateIsUsage(t *testing.T) {
	for _, gate := range []string{"bogus", "Plan", "ship ", "none"} {
		t.Run(gate, func(t *testing.T) {
			s, task := waitingFixture(t)
			setPlan(t, task, planV1)
			locks := []string{filepath.Join(task.Path, ".task.lock"), filepath.Join(task.Path, ".plan.lock")}
			for _, lock := range locks {
				if err := os.Remove(lock); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
			before := approvalSnapshot(t, task.Path)
			if _, err := s.Approve(WriteOptions{ID: task.ID, Gate: gate}, ""); !errors.Is(err, ErrEditUsage) {
				t.Fatalf("gate %q: %v", gate, err)
			}
			if after := approvalSnapshot(t, task.Path); !maps.Equal(after, before) {
				t.Fatal("an unknown gate wrote")
			}
			for _, lock := range locks {
				if _, err := os.Lstat(lock); !os.IsNotExist(err) {
					t.Fatalf("%s created: %v", filepath.Base(lock), err)
				}
			}
		})
	}
}
