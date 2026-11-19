package cli

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// A verify command of a builder Job whose role carries the grid- prefix is the
// rebase Job's verify too.
func TestRebaseVerifyReadsAGridBuilderJob(t *testing.T) {
	items := []job.Job{{ID: "J-1", Role: "grid-builder", Verify: job.Verify{Command: "go test ./...", MinTests: 1}}}
	got := rebaseVerify(items, &mergeConflictError{Files: []string{"a.txt"}})
	if got["command"] != "go test ./..." {
		t.Fatalf("rebase verify %v, want the builder Job's command", got)
	}
}
