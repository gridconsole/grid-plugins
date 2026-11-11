package cli

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// A re-plan Job that names the cause passes the lint options render uses, as the
// one without a cause does (TestGkReplanBodyPassesRenderLint). The cause sits in
// the acceptance text, not in intent.goal, which render resolves as a pointer.
func TestGqReplanBodyWithCausePassesRenderLint(t *testing.T) {
	h := newTaskHarness(t)
	h.env.Getenv = func(k string) string {
		if k == "GRID_WHO" {
			return "alex"
		}
		return ""
	}
	path := newTask(t, h)
	for _, cause := range []string{"launch_failed: no such file", "2", strings.Repeat("y", 900)} {
		body := orchestrator.ReplanJobBody("J-7", "J-2", cause, []string{"J-4", "J-6"})
		findings, err := postFindings(h.env, path, body)
		if err != nil {
			t.Fatalf("the lint options could not be built: %v", err)
		}
		if lint.HasErrors(findings) {
			t.Fatalf("render would refuse the re-plan Job with cause %.40q: %+v", cause, findings)
		}
	}
}
