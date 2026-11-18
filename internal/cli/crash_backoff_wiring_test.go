package cli

import (
	"io"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

// The scheduler of job next and worker next waits between the attempts of a Job that keeps
// failing: configureTickStore sets the back-off every production Store takes.
func TestConfigureTickStoreSetsTheCrashBackoff(t *testing.T) {
	var s orchestrator.Store
	configureTickStore(Env{GridHome: t.TempDir(), Home: t.TempDir(), Stderr: io.Discard}, &s, "example", "01ARZ3NDEKTSV4RRFFQ69G5FAV")
	if s.RetryDelay == nil {
		t.Fatal("no back-off between the attempts of a failing Job")
	}
	if first, third := s.RetryDelay(1), s.RetryDelay(3); first != 10*time.Second || third <= first {
		t.Fatalf("back-off %s then %s", first, third)
	}
}
