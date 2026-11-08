//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd && !dragonfly

package task

import (
	"errors"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func TestNewFailsWhereLockingIsUnsupported(t *testing.T) {
	s, home := storeFor(t)
	if _, err := s.New(NewOptions{Cwd: home, Words: "Fix the label"}); !errors.Is(err, fslock.ErrUnsupported) {
		t.Fatalf("err = %v, want fslock.ErrUnsupported", err)
	}
}
