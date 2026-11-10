package guard

import (
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// A task whose plan is not accepted does not limit where its agents write. The
// fixture task is at its plan gate (pendingTask), with no approval anywhere; every
// repo write below was refused while the plan hold existed.
func TestUnacceptedPlanDoesNotBlockRepoWrites(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*fixture) hookio.Input
	}{
		{"cwd in the task folder", func(f *fixture) hookio.Input {
			return f.input("Write", filepath.Join(f.repo, "file.go"))
		}},
		{"task remembered by the main agent", func(f *fixture) hookio.Input {
			f.pointer(".tasks", f.task)
			i := f.input("Write", filepath.Join(f.repo, "file.go"))
			i.Cwd = f.repo
			return i
		}},
		{"task remembered by a child agent", func(f *fixture) hookio.Input {
			f.pointer(".tasks-worker", f.task)
			i := f.input("Write", filepath.Join(f.repo, "file.go"))
			i.Cwd = f.repo
			i.AgentID = "worker"
			return i
		}},
		{"task listed in the legacy session list", func(f *fixture) hookio.Input {
			i := f.input("Write", filepath.Join(f.repo, "file.go"))
			i.Cwd = f.repo
			f.pointer("", f.task)
			return i
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := setup(t)
			i := tc.prepare(f)
			d := EvaluatePlan(i, f.root, f.env)
			expect(t, d, "")
			checkAdapter(t, PlanGuard, i, f.env, d)
		})
	}
}
