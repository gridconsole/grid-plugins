package cli

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// leRwNoRetired fails when a Job body holds the retired review or max_rounds key.
func leRwNoRetired(t *testing.T, name string, body map[string]any) {
	t.Helper()
	for _, k := range []string{"review", "max_rounds"} {
		if _, ok := body[k]; ok {
			t.Errorf("%s writes the retired key %q", name, k)
		}
	}
}

func TestLeRwBodies(t *testing.T) {
	leRwNoRetired(t, "firstJob", firstJob())
	leRwNoRetired(t, "curatorJobBody", curatorJobBody("J-1", "T-01", []string{"p"}))
	leRwNoRetired(t, "ReplanJobBody", orchestrator.ReplanJobBody("J-7", "J-2", "", []string{"J-4"}))
	leRwNoRetired(t, "approvefix.JobBody", approvefix.JobBody("J-2"))
}
