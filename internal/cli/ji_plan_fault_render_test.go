package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// The re-plan Job the tick posts when plan.md cannot be published must start: it
// passes the lint options render uses, and it is a job-record, with the longest
// cause the body holds and with a cause that is a plan error as the tick builds it.
func TestJiPfBodyPassesRenderLint(t *testing.T) {
	h := newTaskHarness(t)
	h.env.Getenv = func(k string) string {
		if k == "GRID_WHO" {
			return "alex"
		}
		return ""
	}
	path := newTask(t, h)
	for name, cause := range map[string]string{
		"plan error": "plan.md: each jobs entry must be an object",
		"empty":      "",
		"too long":   strings.Repeat("plan.md Job J-3 is wrong; ", 60),
	} {
		t.Run(name, func(t *testing.T) {
			body := orchestrator.ReplanPlanFaultJobBody("J-4", cause)
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
		})
	}
}
