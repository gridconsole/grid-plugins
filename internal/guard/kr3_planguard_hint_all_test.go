package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// Every plan-guard refusal ends with the off hint, at the default (warning) level
// and at block. krPlanGuardHint is written out in kr_planguard_hint_test.go.
func TestKr3PlanGuardEveryRefusalEndsWithTheOffHint(t *testing.T) {
	for _, tc := range []struct {
		name  string
		agent string
		path  func(*fixture) string
	}{
		{"reviewer outside the projects root", "grid:grid-reviewer",
			func(f *fixture) string { return filepath.Join(f.repo, "file.go") }},
		{"checker outside the projects root", "grid:grid-checker",
			func(f *fixture) string { return filepath.Join(f.repo, "file.go") }},
	} {
		t.Run(tc.name+"/default", func(t *testing.T) {
			f := setup(t)
			in := f.input("Write", tc.path(f))
			in.AgentType = tc.agent
			d := EvaluatePlan(in, f.root, f.env)
			if d.Behavior != "" || len(d.Warnings) != 1 {
				t.Fatalf("want one plan-guard warning and no deny, got %+v", d)
			}
			if !strings.HasSuffix(d.Warnings[0], krPlanGuardHint) {
				t.Fatalf("warning %q does not end with %q", d.Warnings[0], krPlanGuardHint)
			}
		})
		t.Run(tc.name+"/block", func(t *testing.T) {
			f := setup(t)
			planBlock(f)
			in := f.input("Write", tc.path(f))
			in.AgentType = tc.agent
			d := EvaluatePlan(in, f.root, f.env)
			expect(t, d, hookio.Deny("").Behavior)
			if !strings.HasSuffix(d.Reason, krPlanGuardHint) {
				t.Fatalf("deny %q does not end with %q", d.Reason, krPlanGuardHint)
			}
		})
	}
}
