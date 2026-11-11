package job_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

// A task with no approved plan renders Jobs: the plan rule is gone.
func TestJobsRenderWithoutApprovedPlan(t *testing.T) {
	cases := map[string]func(t *testing.T, task string){
		// The fixture task has no approve person Job, which is the only approval
		// record, so its plan is unapproved as it stands.
		"unapproved plan": func(t *testing.T, task string) {},
		"edited after approval": func(t *testing.T, task string) {
			path := filepath.Join(task, "plan.md")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(b, []byte("\nOne more note.\n")...), 0o600); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			_, resolver, h := sample(t)
			setup(t, h.Task)
			j, err := job.LoadPlan(h.Task, "1")
			if err != nil {
				t.Fatalf("LoadPlan: %v", err)
			}
			for _, f := range []*job.Files{resolver, {Task: h.Task}} {
				if s, err := f.Resolve("plan.md#step-1"); err != nil || s.Text == "" {
					t.Fatalf("plan anchor: %q %v", s.Text, err)
				}
			}
			if r, err := job.Render(j, job.RenderOptions{Resolver: resolver}); err != nil || r.Brief == "" {
				t.Fatalf("render: %v", err)
			}
		})
	}
}

// A task with no plan.md resolves its other anchors and renders no plan item.
func TestNoPlanFileResolvesTaskAnchors(t *testing.T) {
	_, resolver, h := sample(t)
	if err := os.Remove(filepath.Join(h.Task, "plan.md")); err != nil {
		t.Fatal(err)
	}
	if s, err := resolver.Resolve("intent.md#request"); err != nil || s.Text == "" {
		t.Fatalf("intent anchor: %v", err)
	}
}
