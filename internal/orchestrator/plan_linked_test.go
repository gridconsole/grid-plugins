package orchestrator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// A task.md forged through a planted link could drop the approve step, so a
// linked task posts no Jobs even when its workflow asks for no plan approval.
func TestLinkedTaskPostsNoJobs(t *testing.T) {
	for _, tc := range []struct {
		name, link string
		want       int
	}{
		{"unlinked", "", 1},
		{"linked task.md", "task.md", 0},
		{"linked plan.md", "plan.md", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupPlan(t)
			writePlan(t, f, plannedText(t))
			rewritePlanFile(t, f.s.Feed.TaskDir, "task.md", "workflow: ''", "workflow: [{step: build, kind: run}, {step: review, kind: approve}]")
			if tc.link != "" {
				if err := os.Link(filepath.Join(f.s.Feed.TaskDir, tc.link), filepath.Join(t.TempDir(), "t")); err != nil {
					t.Fatal(err)
				}
			}
			out, err := f.s.Tick()
			if tc.want > 0 && err != nil {
				t.Fatal(err)
			}
			records, readErr := f.s.Feed.Read(feed.Filter{Type: "job"})
			if readErr != nil || len(records.Records) != tc.want || out.Assigned != tc.want {
				t.Fatalf("jobs=%d assigned=%d want=%d tick err=%v read err=%v", len(records.Records), out.Assigned, tc.want, err, readErr)
			}
		})
	}
}
