package testenv

import (
	"os"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/persongate"
)

func TestPersonRestoresEnvironment(t *testing.T) {
	// Include future names in both wildcard families, not just today's keys.
	keys := persongate.EnvironmentKeys([]string{"GRID_WORKER_FUTURE=agent", "GRID_LEASE_FUTURE=agent"})
	for _, key := range keys {
		t.Setenv(key, "agent")
	}
	t.Setenv("GRID_TESTENV_UNRELATED", "keep")
	t.Run("person", func(t *testing.T) {
		Person(t)
		for _, key := range keys {
			if os.Getenv(key) != "" {
				t.Errorf("inherited marker %s survived", key)
			}
		}
		if os.Getenv("GRID_TESTENV_UNRELATED") != "keep" {
			t.Fatal("unrelated environment changed")
		}
		// Explicit agent setup after isolation must remain visible.
		t.Setenv("GRID_AGENT", "builder")
		if os.Getenv("GRID_AGENT") != "builder" {
			t.Fatal("explicit agent marker lost")
		}
	})
	for _, key := range keys {
		if os.Getenv(key) != "agent" {
			t.Errorf("inherited marker %s was not restored", key)
		}
	}
}
