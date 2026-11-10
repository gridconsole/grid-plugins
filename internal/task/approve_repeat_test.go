package task

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

// approvalSnapshot holds every file an approval may write.
func approvalSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, name := range []string{"task.md", "history.md", approvalFile} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		out[name] = string(data)
	}
	return out
}

// A repeat approve whose receipt already matches the plan returns that receipt
// and writes nothing: no second receipt write and no second history line.
func TestApproveRepeatWritesNothing(t *testing.T) {
	for _, tc := range []struct{ at, file string }{
		{"plan", approvalFile},
	} {
		t.Run(tc.at, func(t *testing.T) {
			s, task := waitingFixture(t)
			h := setPlan(t, task, planV1)
			first, err := s.Approve(WriteOptions{ID: task.ID, Note: "Looks right"}, "")
			if err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(task.Path, tc.file)
			old := time.Date(2001, 1, 2, 3, 4, 5, 0, time.UTC)
			if err := os.Chtimes(receipt, old, old); err != nil {
				t.Fatal(err)
			}
			before := approvalSnapshot(t, task.Path)
			for _, token := range []string{"", h, plan.Digest(planV1)} {
				again, err := s.Approve(WriteOptions{ID: task.ID, Note: "Pressed twice", Via: "grid-app"}, token)
				if err != nil {
					t.Fatalf("repeat with token %q: %v", token, err)
				}
				if after := approvalSnapshot(t, task.Path); !maps.Equal(after, before) {
					t.Fatalf("repeat with token %q wrote:\nhistory before:\n%s\nhistory after:\n%s", token, before["history.md"], after["history.md"])
				}
				if again != first {
					t.Fatalf("repeat with token %q returned %+v, want the stored %+v", token, again, first)
				}
			}
			if info, err := os.Stat(receipt); err != nil {
				t.Fatal(err)
			} else if !info.ModTime().Equal(old) {
				t.Fatalf("receipt rewritten at %v", info.ModTime())
			}
		})
	}
}

// A receipt bound to another deploy rule is not a repeat: approving again rebinds it.
func TestApproveRepeatWithNewLandsHashRewrites(t *testing.T) {
	s, task := waitingFixture(t)
	h := setPlan(t, task, planV1)
	if _, err := s.Approve(WriteOptions{ID: task.ID, LandsHash: "rule-a"}, ""); err != nil {
		t.Fatal(err)
	}
	event, err := s.Approve(WriteOptions{ID: task.ID, LandsHash: "rule-b"}, h)
	if err != nil || event.LandsHash != "rule-b" {
		t.Fatalf("%+v: %v", event, err)
	}
	var stored Event
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(task.Path, approvalFile))), &stored); err != nil || stored.LandsHash != "rule-b" {
		t.Fatalf("receipt not rebound: %+v %v", stored, err)
	}
}

