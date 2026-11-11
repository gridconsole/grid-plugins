package jobgraph

import (
	"strings"
	"testing"
)

func nqNeedsPlan(lines ...string) string {
	var b strings.Builder
	b.WriteString("# Plan\n")
	for i, l := range lines {
		b.WriteString("- [ ] " + string(rune('1'+i)) + " Step\n")
		if l != "" {
			b.WriteString("  " + l + "\n")
		}
		b.WriteString("  jobs: []\n")
	}
	return b.String()
}

func TestNqNeedsWarnings(t *testing.T) {
	for _, tc := range []struct{ name, plan, want string }{
		{"missing line", nqNeedsPlan("needs: none", "", "needs: 1"), "item 2: no needs: line"},
		{"unknown step", nqNeedsPlan("needs: none", "needs: 7"), "item 2: needs step 7, which is not in the plan"},
		{"itself", nqNeedsPlan("needs: none", "needs: 1", "needs: 3"), "item 3: needs itself"},
		{"cycle", nqNeedsPlan("needs: 2", "needs: 1"), "item 1: needs cycle 1 -> 2 -> 1"},
		{"not numbers", nqNeedsPlan("needs: original", "needs: none"), "item 1: needs: original is not step numbers or none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := CheckPlanReport(tc.plan)
			if len(r.Problems) != 0 {
				t.Fatalf("problems: %v", r.Problems)
			}
			found := false
			for _, w := range r.Warnings {
				found = found || w == tc.want
			}
			if !found {
				t.Fatalf("warnings %q lack %q", r.Warnings, tc.want)
			}
		})
	}
}

func TestNqNeedsSilentWithoutAnyLine(t *testing.T) {
	r := CheckPlanReport(nqNeedsPlan("", "", ""))
	if len(r.Problems) != 0 {
		t.Fatalf("problems: %v", r.Problems)
	}
	for _, w := range r.Warnings {
		if strings.HasPrefix(w, "item ") {
			t.Fatalf("warning %q on a plan with no needs line", w)
		}
	}
}

func TestNqNeedsCleanPlanHasNoWarnings(t *testing.T) {
	r := CheckPlanReport(nqNeedsPlan("needs: none", "needs: 1", "needs: 1, 2"))
	if len(r.Problems) != 0 || len(r.Warnings) != 0 {
		t.Fatalf("%v %v", r.Problems, r.Warnings)
	}
}
