package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// The re-plan Job the loop posts for a held Job whose wait can no longer be met
// must start: it passes the lint options render uses although the Jobs it names
// (J-2, J-4, J-6) never had a folder under jobs/.
func TestJhReplanHeldBodyPassesRenderLint(t *testing.T) {
	h := newTaskHarness(t)
	h.env.Getenv = func(k string) string {
		if k == "GRID_WHO" {
			return "alex"
		}
		return ""
	}
	path := newTask(t, h)
	for _, name := range []string{"J-2", "J-4", "J-6"} {
		if _, err := os.Stat(filepath.Join(path, "jobs", name)); err == nil {
			t.Fatalf("jobs/%s exists: the test needs a Job that never rendered", name)
		}
	}
	body := orchestrator.ReplanHeldJobBody("J-7", "J-6", "J-2", "A1", "J-4", contract.Cancelled)
	findings, err := postFindings(h.env, path, body)
	if err != nil {
		t.Fatalf("the lint options could not be built: %v", err)
	}
	if lint.HasErrors(findings) {
		t.Fatalf("render would refuse the re-plan Job: %+v", findings)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the re-plan body is not a job-record: %v", err)
	}
}
