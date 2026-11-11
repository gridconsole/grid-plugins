//go:build unix

package routine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The timer path runs through a regular file, so Lstat fails with something
// other than not-exist on unix. Windows reports not-exist here.
func TestSICheckFilesLstatFailure(t *testing.T) {
	s, store, fake := siSetup(t, siBackend{})
	if err := os.WriteFile(filepath.Join(s.Home, "timers"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := s.Schedule(context.Background(), store)
	if err == nil || !strings.HasPrefix(err.Error(), "lstat ") {
		t.Fatalf("lstat through a file: %v, want prefix %q", err, "lstat ")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("runner calls: %v", fake.calls)
	}
}
