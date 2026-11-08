package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
)

func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{
		{"stale", feed.ErrStaleLease, 3},
		{"invalid", feed.ErrInvalidRecord, 3},
		{"typed invalid", &feed.InvalidRecordError{Err: errors.New("old diagnostic")}, 3},
		{"disk full", feed.ErrDiskFull, 4},
		{"storage", errors.New("storage failure"), 1},
		{"duplicate wake", orchestrator.ErrDuplicateWake, 1},
		{"plain stale text", errors.New("stale lease"), 1},
		{"plain invalid text", errors.New("invalid record"), 1},
		{"plain disk text", errors.New("disk full"), 1},
		{"plain body text", errors.New("--body must be a JSON object"), 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.err
			for depth := 0; depth < 3; depth++ {
				if got := exitCode(err); got != tc.want {
					t.Errorf("depth %d: %d, want %d", depth, got, tc.want)
				}
				err = fmt.Errorf("wrapped: %w", err)
			}
		})
	}
}
