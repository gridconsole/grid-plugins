package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A new home holding only what `grid-worker start` writes (an empty worker.lock and
// worker-status.json) is a fresh home: `grid migrate` with no --from has nothing to
// migrate, says so on stdout and exits 0.
func TestMigrateWorkerOnlyHome(t *testing.T) {
	h := newHarness(t)
	for _, name := range []string{"worker.lock", "worker-status.json"} {
		if err := os.WriteFile(filepath.Join(h.gridHome, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code := h.run(h.root, "migrate"); code != 0 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, h.out, h.errb)
	}
	if h.out.String() != "migrate: nothing to migrate\n" {
		t.Fatalf("stdout %q, stderr %q", h.out, h.errb)
	}
	if h.errb.Len() != 0 {
		t.Fatalf("stderr %q", h.errb)
	}
}

// A home with other content but no projects folder keeps its error.
func TestMigrateWorkerFilesPlusOtherContentStillErrors(t *testing.T) {
	h := newHarness(t)
	for _, name := range []string{"worker.lock", "worker-status.json", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(h.gridHome, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if code := h.run(h.root, "migrate"); code != 1 {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, h.out, h.errb)
	}
	if !strings.Contains(h.errb.String(), "no old projects folder found") {
		t.Fatalf("stderr %q", h.errb)
	}
}
