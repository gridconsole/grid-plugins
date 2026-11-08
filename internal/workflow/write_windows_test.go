//go:build windows

package workflow

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func TestWriteUnsupportedLock(t *testing.T) {
	home := t.TempDir()
	_, err := Write(home, "", "none", strings.NewReader(writeFixture(t, "root.md")))
	if !errors.Is(err, fslock.ErrUnsupported) || !strings.Contains(err.Error(), "WSL2") || strings.Contains(err.Error(), home) {
		t.Fatalf("want path-free unsupported lock hint, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "workflows.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported lock created workflow: %v", err)
	}
}
