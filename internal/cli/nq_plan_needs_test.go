package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNqNeedsPlanItemsAndCheck(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	two := "# Plan\n- [ ] 1 One\n  needs: none\n  jobs: []\n- [ ] 2 Two\n  needs: 1\n  jobs: []\n"
	h.write(filepath.Join(task, "plan.md"), two)
	if code := h.run(task, "plan", "items", "--json"); code != 0 {
		t.Fatalf("items %d: %s %s", code, h.out, h.errb)
	}
	for _, frag := range []string{`"needs":[]`, `"needs":[1]`} {
		if !strings.Contains(h.out.String(), frag) {
			t.Fatalf("items output lacks %s: %s", frag, h.out)
		}
	}
	bad := "# Plan\n- [ ] 1 One\n  needs: none\n  jobs: []\n- [ ] 2 Two\n  needs: 7\n  jobs: []\n"
	h.write(filepath.Join(task, "plan.md"), bad)
	if code := h.run(task, "plan", "check"); code != 0 {
		t.Fatalf("check %d: %s %s", code, h.out, h.errb)
	}
	if want := "warning: item 2: needs step 7, which is not in the plan"; !strings.Contains(h.out.String(), want) {
		t.Fatalf("output lacks %q: %s", want, h.out)
	}
}
