package cli

import (
	"strings"
	"testing"
)

const opsDefaultsWant = "No telemetry. Crash reports and a daily copy (7 days) stay in your Grid home. Old sessions are not imported."

// Setup into a temp home prints the ops defaults once, for both a write and a removal.
func TestSetupSummaryCarriesOpsDefaultsOnce(t *testing.T) {
	h := claudeHarness(t)
	for _, args := range [][]string{{"setup", "--yes"}, {"setup", "--yes"}, {"setup", "--remove", "--yes"}} {
		h.out.Reset()
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("%v: %d %s", args, code, h.errb)
		}
		if n := strings.Count(h.out.String(), opsDefaultsWant); n != 1 {
			t.Fatalf("%v: ops defaults line found %d times in:\n%s", args, n, h.out)
		}
	}
}

// The JSON form stays machine-readable: no summary line in it.
func TestSetupJSONHasNoOpsDefaultsLine(t *testing.T) {
	h := claudeHarness(t)
	if code := h.run(h.home, "setup", "--yes", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	if strings.Contains(h.out.String(), "No telemetry") {
		t.Fatal(h.out)
	}
}
