package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// On a fresh Grid home `grid migrate` with no --from has nothing to migrate: it says
// so on stdout and exits 0.
func TestMigrateFreshHome(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(h *harness)
	}{
		{"missing", func(h *harness) {
			if err := os.RemoveAll(h.gridHome); err != nil {
				t.Fatal(err)
			}
		}},
		{"empty", func(h *harness) {}},
		// A home with only worker.lock and worker-status.json is not covered: it ends
		// in "no old projects folder found" (internal/migrate/plan.go:149), which this
		// lane does not change.
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			tc.setup(h)
			if code := h.run(h.root, "migrate"); code != 0 {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, h.out, h.errb)
			}
			if h.out.String() != "migrate: nothing to migrate\n" {
				t.Fatalf("stdout %q, stderr %q", h.out, h.errb)
			}
			if h.errb.Len() != 0 {
				t.Fatalf("stderr %q", h.errb)
			}
		})
	}
}

// A folder named with --from that is missing still fails and names the path.
func TestMigrateFromMissingNamesPath(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.root, "migrate", "--from", "missing"); code != 1 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, h.out, h.errb)
	}
	if want := filepath.Join(h.root, "missing"); !strings.Contains(h.errb.String(), want) {
		t.Fatalf("stderr lacks %q: %q", want, h.errb)
	}
}
