package task

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// The digest is over each step's name, kind and plan-gate place, in order.
func TestWorkflowDigestReadsNameKindGateAndOrder(t *testing.T) {
	base := []taskfile.WorkflowStep{{Step: "plan", Kind: "approve"}, {Step: "build", Kind: "run"}}
	same := []taskfile.WorkflowStep{{Step: "plan", Kind: "approve"}, {Step: "build", Kind: "run"}}
	if WorkflowDigest(base) != WorkflowDigest(same) {
		t.Fatal("equal steps give different digests")
	}
	if got := WorkflowDigest(base); len(got) != 64 {
		t.Fatalf("digest %q is not 64 hex digits", got)
	}
	for name, other := range map[string][]taskfile.WorkflowStep{
		"kind":  {{Step: "plan", Kind: "run"}, {Step: "build", Kind: "run"}},
		"name":  {{Step: "plan", Kind: "approve"}, {Step: "make", Kind: "run"}},
		"order": {{Step: "build", Kind: "run"}, {Step: "plan", Kind: "approve"}},
		"gate":  {{Step: "sign", Kind: "approve"}, {Step: "build", Kind: "run"}, {Step: "plan", Kind: "approve"}},
		"none":  nil,
	} {
		if WorkflowDigest(other) == WorkflowDigest(base) {
			t.Errorf("%s: a different list gives the same digest", name)
		}
	}
}
