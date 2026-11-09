package testenv

import (
	"os"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/persongate"
)

// Person clears the gate's environment inputs for a test that starts as a
// person. Call before setting any agent or role markers under test, never at
// command dispatch, so refusal tests still exercise the real gate.
// Setenv restores the inherited values when the test finishes.
func Person(t testing.TB) {
	t.Helper()
	for _, key := range persongate.EnvironmentKeys(os.Environ()) {
		t.Setenv(key, "")
	}
}
