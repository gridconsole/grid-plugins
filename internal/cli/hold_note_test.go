package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The dated line, bullet and all, typed as a history note or an actor id. It
// lands mid-line inside a JSON event and must not release an unapproved plan.
func TestDatedAcceptanceTypedIntoACommandDoesNotRelease(t *testing.T) {
	const dated = "- 2026-01-02T03:04 · plan accepted · by person"
	for _, tc := range []struct {
		name, actor string
		args        []string
	}{
		{"history-add-note", "", []string{"history", "add", "--", dated}},
		{"actor-id", dated, []string{"history", "add", "checked the plan"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, dir := deliveryFixture(t)
			if err := os.Remove(filepath.Join(dir, "approval.json")); err != nil {
				t.Fatal(err)
			}
			addDeliveryProof(t, h, dir)
			if tc.actor != "" {
				t.Setenv("GRID_ACTOR", tc.actor)
			}
			if code := h.run(dir, tc.args...); code != 0 {
				t.Fatalf("exit %d: %q", code, h.errb.String())
			}
			history, err := os.ReadFile(filepath.Join(dir, "history.md"))
			if err != nil {
				t.Fatal(err)
			}
			var event string
			for _, line := range strings.Split(string(history), "\n") {
				if strings.Contains(line, dated) {
					event = line
				}
			}
			if !strings.HasPrefix(event, "- {") {
				t.Fatalf("no JSON event line carries the dated line: %q", history)
			}
			before := treeBytes(t, h.gridHome)
			code := h.run(dir, "deliver", "Result ready")
			line := h.errb.String()
			if code != 1 || !strings.HasPrefix(line, "Deliver blocked: plan not approved: ") || strings.Count(line, "\n") != 1 {
				t.Fatalf("exit %d: %q", code, line)
			}
			if after := treeBytes(t, h.gridHome); !reflect.DeepEqual(after, before) {
				t.Fatalf("a refused deliver changed the grid home:\n%v\nwas:\n%v", after, before)
			}
		})
	}
}
