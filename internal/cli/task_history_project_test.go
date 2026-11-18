package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// grid task history takes --project, as task show and task merge do: a T-NN that two
// projects share names the one in the project given, and the history read is its own.
func TestTaskHistoryAcceptsProject(t *testing.T) {
	h, exampleTask, twinTask := twinHarness(t)
	h.write(filepath.Join(exampleTask, "history.md"), "- 2026-01-02T03:04 · note · lives in example · by local · via test\n")
	h.write(filepath.Join(twinTask, "history.md"), "- 2026-01-02T03:04 · note · lives in twin · by local · via test\n")
	if code := h.run(h.home, "task", "history", "T-07"); code == 0 {
		t.Fatalf("T-07 in two projects resolved without --project: %s", h.out.String())
	}
	for _, p := range []string{"example", "twin"} {
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(h.home, "task", "history", "T-07", "--project", p); code != 0 {
			t.Fatalf("task history T-07 --project %s: exit %d: %s", p, code, h.errb)
		}
		if got := h.out.String(); !strings.Contains(got, "lives in "+p) || strings.Contains(got, "lives in "+map[string]string{"example": "twin", "twin": "example"}[p]) {
			t.Fatalf("task history T-07 --project %s printed %q, want only that project's line", p, got)
		}
	}
	if code := h.run(h.home, "task", "history", "T-07", "--project", "nowhere"); code == 0 {
		t.Fatal("an unknown project resolved")
	}
}
