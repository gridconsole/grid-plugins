package cli

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

// TestDtGateWordsHelpGolden compares `grid help` with the shipped golden, so the
// golden cannot drift from the command table unseen.
func TestDtGateWordsHelpGolden(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "help"); code != 0 {
		t.Fatalf("grid help: exit %d: %s", code, h.errb)
	}
	golden.Assert(t, "help", h.out.String())
}
