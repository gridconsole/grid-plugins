package settings

import "testing"

// plan-guard warns by default (needed.md:47, simplify.md:55 row 4), and so does
// host-guard (v-enforce). job-lint and stop-check keep block; only-writer and
// every other hook warn.
func TestDefaultLevelPlanGuardWarns(t *testing.T) {
	for name, want := range map[string]string{
		"plan-guard":  "warn",
		"host-guard":  "warn",
		"job-lint":    "block",
		"stop-check":  "block",
		"only-writer": "warn",
	} {
		if got := defaultLevel(name); got != want {
			t.Errorf("defaultLevel(%q) = %q, want %q", name, got, want)
		}
	}
}
