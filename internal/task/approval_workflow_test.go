package task

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
)

func TestApprovalWorkflowEdits(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		approved   bool
	}{
		{"tick", strings.Replace(planV1, "[ ]", "[X]", 1), true},
		{"status", strings.Replace(planV1, "## Where it stands\n", "## Where it stands\nBuild started; the next step is underway.\n", 1), true},
		{"CRLF", strings.ReplaceAll(planV1, "\n", "\r\n"), true},
		{"trailing spaces", strings.ReplaceAll(planV1, "\n", " \t\n"), true},
		{"line one", strings.Replace(planV1, "rev 1", "rev 2", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := waitingFixture(t)
			setPlan(t, task, planV1)
			if _, err := approve(t, s, task, WriteOptions{ID: task.ID}, ""); err != nil {
				t.Fatal(err)
			}
			setPlan(t, task, tc.text)
			_, err := s.Approval("", task.ID)
			if (err == nil) != tc.approved || Approved(task.Path) != tc.approved {
				t.Fatalf("approval = %t, want %t: %v", err == nil, tc.approved, err)
			}
		})
	}
}

func TestApprovalPriorDigestCompatibility(t *testing.T) {
	const body = "- [ ] 1 Build it\n## Verify\nIt works.\n"
	for _, tc := range []struct {
		name, text, oldInput string
		approved             bool
	}{
		{"canonical blank first line", "\n" + body, body, true},
		{"canonical heading", "# Plan\n" + body, body, false},
		{"canonical step first line", "- [ ] 2 Earlier\n" + body, body, false},
		{"L107 canonical bytes", "# Plan\n" + body, "# Plan\n" + body, true},
		{"L107 ticked bytes", "# Plan\n" + strings.Replace(body, "[ ]", "[x]", 1), "# Plan\n" + strings.Replace(body, "[ ]", "[x]", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, task := waitingFixture(t)
			hash := setPlan(t, task, tc.text)
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(tc.oldInput)))
			postResult(t, task.Path, openJob(t, task.Path), map[string]any{"plan_hash": hash, "plan_digest": digest})
			_, err := s.Approval("", task.ID)
			if (err == nil) != tc.approved || Approved(task.Path) != tc.approved {
				t.Fatalf("prior receipt approval = %t, want %t: %v", err == nil, tc.approved, err)
			}
			if !tc.approved {
				if _, err := s.Approve(WriteOptions{ID: task.ID}, ""); err == nil {
					t.Fatal("implicit migration accepted")
				}
				event, err := approve(t, s, task, WriteOptions{ID: task.ID}, hash)
				if err != nil || event.PlanDigest != plan.Digest(tc.text) || !Approved(task.Path) {
					t.Fatalf("explicit reapproval: %v", err)
				}
			}
		})
	}
}
