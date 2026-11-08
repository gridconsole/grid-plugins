package taskfile

import "testing"

func TestPlanApprovalKinds(t *testing.T) {
	for _, tc := range []struct {
		name, workflow string
		want           bool
	}{
		{"sign-off before work", "[{step: sign-off, kind: approve}, {step: build, kind: work}]", true},
		{"sign-off", "[{step: sign-off, kind: approve}, {step: build, kind: run}]", true},
		{"named plan after run", "[{step: research, kind: run}, {step: plan-review, kind: approve}]", true},
		{"ship after run", "[{step: build, kind: run}, {step: sign-off, kind: approve}]", false},
		{"ship after work", "[{step: build, kind: work}, {step: sign-off, kind: approve}]", false},
		{"no gate", "[{step: build, kind: run}]", false},
		{"unknown kind", "[{step: build, kind: typo}]", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlanApprovalRequired("---\nworkflow: " + tc.workflow + "\n---\n"); got != tc.want {
				t.Fatalf("plan approval required = %t, want %t", got, tc.want)
			}
		})
	}
}
