package settings

import "testing"

// plan-guard warns by default (needed.md:47, simplify.md:55 row 4). The other
// guards keep the default they had at the base of this change: host-guard,
// job-lint and stop-check block; only-writer and every other hook warn.
func TestDefaultLevelPlanGuardWarns(t *testing.T) {
	for name, want := range map[string]string{
		"plan-guard":  "warn",
		"host-guard":  "block",
		"job-lint":    "block",
		"stop-check":  "block",
		"only-writer": "warn",
	} {
		if got := defaultLevel(name); got != want {
			t.Errorf("defaultLevel(%q) = %q, want %q", name, got, want)
		}
	}
}
