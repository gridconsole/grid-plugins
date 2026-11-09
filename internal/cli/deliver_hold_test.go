package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// `grid deliver` on a task whose plan nobody approved exits 1 with one
// "Deliver blocked:" line, and writes nothing.
func TestDeliverRefusesAHeldPlanThroughTheCommand(t *testing.T) {
	h, dir := deliveryFixture(t)
	if err := os.Remove(filepath.Join(dir, "approval.json")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	addDeliveryProof(t, h, dir)
	before := treeBytes(t, h.gridHome)
	code := h.run(dir, "deliver", "Result ready")
	line := h.errb.String()
	if code != 1 || !strings.HasPrefix(line, "Deliver blocked: plan not approved: ") || !strings.Contains(line, "waits for a person to approve its plan") || strings.Count(line, "\n") != 1 {
		t.Fatalf("exit %d: %q", code, line)
	}
	if after := treeBytes(t, h.gridHome); !reflect.DeepEqual(after, before) {
		t.Fatalf("a refused deliver changed the grid home:\n%v\nwas:\n%v", after, before)
	}
}
