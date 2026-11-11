package cli

import (
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/guard"
)

// The words of the plan-guard warning's hint, run as a person would run them,
// change the setting; a renamed verb or hook turns this red.
func TestKrPlanGuardOffCommandRuns(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	args := strings.Split(strings.TrimPrefix(guard.PlanGuardOffCommand, "grid "), " ")
	if code := h.run(h.home, args...); code != 0 || !strings.Contains(h.out.String(), "updated in personal") {
		t.Fatalf("%v: exit %d: %s %s", args, code, h.out, h.errb)
	}
	if code := h.run(h.home, "hook", "levels"); code != 0 || !strings.Contains(h.out.String(), "plan-guard off") {
		t.Fatalf("hook levels after the write: exit %d: %s %s", code, h.out, h.errb)
	}
}
